package systools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const autoremoveSim = `Reading package lists...
The following packages will be REMOVED:
  linux-image-5.15.0-88-generic linux-modules-5.15.0-88-generic
0 upgraded, 0 newly installed, 2 to remove and 0 not upgraded.
Remv linux-image-5.15.0-88-generic [5.15.0-88.98]
Remv linux-modules-5.15.0-88-generic [5.15.0-88.98]
`

const dockerDF = `{"Active":"2","Reclaimable":"1.5GB (60%)","Size":"2.5GB","TotalCount":"5","Type":"Images"}
{"Active":"2","Reclaimable":"0B (0%)","Size":"12kB","TotalCount":"2","Type":"Containers"}
{"Active":"1","Reclaimable":"300MB (50%)","Size":"600MB","TotalCount":"2","Type":"Local Volumes"}
{"Active":"0","Reclaimable":"250.5MB","Size":"250.5MB","TotalCount":"12","Type":"Build Cache"}
`

func sized(t *testing.T, path string, n int) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, make([]byte, n), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupMeasure(t *testing.T) {
	e := newTest(t)
	ctx := context.Background()
	a := e.m.p.AptArchives
	sized(t, filepath.Join(a, "nginx_1.24.0_amd64.deb"), 3000)
	sized(t, filepath.Join(a, "libssl3_3.0.13_amd64.deb"), 2000)
	sized(t, filepath.Join(a, "lock"), 0)
	sized(t, filepath.Join(a, "partial", "php8.3_8.3.7.deb.FAILED"), 500)
	l := e.m.p.VarLog
	sized(t, filepath.Join(l, "syslog"), 9000)
	sized(t, filepath.Join(l, "syslog.1"), 100)
	sized(t, filepath.Join(l, "syslog.2.gz"), 200)
	sized(t, filepath.Join(l, "apt", "history.log.1.gz"), 300)
	sized(t, filepath.Join(l, "nginx", "example.com.access.log.1"), 5000) // traffic statistics
	sized(t, filepath.Join(l, "dpkg.log"), 400)
	os.Symlink(filepath.Join(l, "syslog.1"), filepath.Join(l, "link.1"))
	e.f.answers["journalctl --disk-usage"] = "Archived and active journals take up 1.5G in the file system.\n"
	e.f.answers["apt-get -s autoremove"] = autoremoveSim
	e.f.answers["dpkg-query -W"] = "linux-image-5.15.0-88-generic\t11712\nlinux-modules-5.15.0-88-generic\t102400\n"

	items := e.m.Cleanup(ctx)
	ids := []string{}
	byID := map[string]CleanItem{}
	for _, it := range items {
		ids = append(ids, it.ID)
		byID[it.ID] = it
	}
	if strings.Join(ids, " ") != "apt journal logs autoremove" {
		t.Fatalf("items = %q (no Docker without Docker)", ids)
	}
	if it := byID["apt"]; it.Bytes != 5500 || it.Files != 3 {
		t.Errorf("apt = %+v", it)
	}
	if it := byID["journal"]; it.Total != 1610612736 || it.Bytes != 1610612736-100<<20 {
		t.Errorf("journal = %+v", it)
	}
	if it := byID["logs"]; it.Bytes != 600 || it.Files != 3 {
		t.Errorf("logs = %+v", it)
	}
	if it := byID["autoremove"]; it.Bytes != (11712+102400)<<10 || len(it.Packages) != 2 || it.Packages[1].Name != "linux-modules-5.15.0-88-generic" {
		t.Errorf("autoremove = %+v", it)
	}
	if !e.f.ran("dpkg-query -W -f ${Package}\t${Installed-Size}\n linux-image-5.15.0-88-generic linux-modules-5.15.0-88-generic") {
		t.Errorf("calls = %q", e.f.calls)
	}

	// Docker, when it is installed: unused images and build cache, not volumes.
	e.write(t, e.m.p.DockerBin, "")
	e.f.answers["docker system df"] = dockerDF
	items = e.m.Cleanup(ctx)
	if d := items[len(items)-1]; d.ID != "docker" || d.Bytes != 1_500_000_000+250_500_000 {
		t.Fatalf("docker = %+v", d)
	}
	// The daemon is down: the item says so.
	e.f.fail["docker system df"] = "Cannot connect to the Docker daemon at unix:///var/run/docker.sock."
	if d := e.m.Cleanup(ctx)[len(items)-1]; d.Error == "" {
		t.Fatalf("docker down = %+v", d)
	}
}

func TestCleanLogs(t *testing.T) {
	e := newTest(t)
	l := e.m.p.VarLog
	sized(t, filepath.Join(l, "syslog"), 10)
	sized(t, filepath.Join(l, "syslog.1"), 100)
	sized(t, filepath.Join(l, "auth.log.3.gz"), 200)
	sized(t, filepath.Join(l, "nginx", "example.com.access.log.2.gz"), 300)
	sized(t, filepath.Join(l, "private", "secret.1"), 300)
	// Folders that are not root's own are left alone.
	e.m.trustedDir = func(fi os.FileInfo) bool { return fi.Name() != "private" }
	var lg testLog
	freed, err := e.m.Clean(context.Background(), "logs", nil, &lg)
	if err != nil || freed != 300 {
		t.Fatalf("freed %d, %v", freed, err)
	}
	for p, want := range map[string]bool{"syslog": true, "syslog.1": false, "auth.log.3.gz": false,
		"nginx/example.com.access.log.2.gz": true, "private/secret.1": true} {
		if exists(filepath.Join(l, filepath.FromSlash(p))) != want {
			t.Errorf("%s exists = %v", p, !want)
		}
	}
}

func TestCleanCommands(t *testing.T) {
	e := newTest(t)
	ctx := context.Background()
	var lg testLog
	if err := e.m.CheckClean("everything"); code(err) != "cleanup_unknown" {
		t.Fatalf("unknown: %v", err)
	}
	if _, err := e.m.Clean(ctx, "docker", nil, &lg); code(err) != "software_not_installed" {
		t.Fatalf("docker missing: %v", err)
	}

	// apt-get clean, measured before and after.
	sized(t, filepath.Join(e.m.p.AptArchives, "a.deb"), 1000)
	e.f.hook = func(line string) {
		if line == "APT clean" {
			os.Remove(filepath.Join(e.m.p.AptArchives, "a.deb"))
		}
	}
	if freed, err := e.m.Clean(ctx, "apt", nil, &lg); err != nil || freed != 1000 {
		t.Fatalf("apt: %d %v", freed, err)
	}

	// The journal: rotated, then vacuumed to 100 MB.
	usage := "Archived and active journals take up 800.0M in the file system."
	e.f.fn["journalctl --disk-usage"] = func(string) (string, error) { return usage, nil }
	e.f.hook = func(line string) {
		if line == "journalctl --vacuum-size=100M" {
			usage = "Archived and active journals take up 96.0M in the file system."
		}
	}
	if freed, err := e.m.Clean(ctx, "journal", nil, &lg); err != nil || freed != 704<<20 || !e.f.ran("journalctl --rotate") {
		t.Fatalf("journal: %d %v %q", freed, err, e.f.calls)
	}

	// autoremove only removes what the administrator saw.
	e.f.hook = nil
	e.f.answers["apt-get -s autoremove"] = autoremoveSim
	e.f.answers["dpkg-query -W"] = "linux-image-5.15.0-88-generic\t1024\nlinux-modules-5.15.0-88-generic\t2048\n"
	if _, err := e.m.Clean(ctx, "autoremove", []string{"linux-image-5.15.0-88-generic"}, &lg); code(err) != "autoremove_changed" {
		t.Fatalf("changed list: %v", err)
	}
	if e.f.ran("APT autoremove") {
		t.Fatal("autoremove ran")
	}
	freed, err := e.m.Clean(ctx, "autoremove", []string{"linux-image-5.15.0-88-generic", "linux-modules-5.15.0-88-generic"}, &lg)
	if err != nil || freed != 3072<<10 || !e.f.ran("APT autoremove") {
		t.Fatalf("autoremove: %d %v", freed, err)
	}

	// Docker: images and build cache.
	e.write(t, e.m.p.DockerBin, "")
	df := dockerDF
	e.f.fn["docker system df"] = func(string) (string, error) { return df, nil }
	e.f.hook = func(line string) {
		if strings.HasPrefix(line, "docker builder prune") {
			df = `{"Reclaimable":"0B","Type":"Images"}` + "\n" + `{"Reclaimable":"0B","Type":"Build Cache"}`
		}
	}
	if freed, err := e.m.Clean(ctx, "docker", nil, &lg); err != nil || freed != 1_750_500_000 ||
		!e.f.ran("docker image prune --all --force") || !e.f.ran("docker builder prune --all --force") || e.f.ran("docker volume") {
		t.Fatalf("docker: %d %v %q", freed, err, e.f.calls)
	}
}
