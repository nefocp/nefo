package software

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/store"
)

// fakeJDK makes /usr/lib/jvm/java-<major>-openjdk-<arch> with bin/java and
// a release file.
func fakeJDK(t *testing.T, major, arch, version string) string {
	t.Helper()
	home := filepath.Join(JVMDir, "java-"+major+"-openjdk-"+arch)
	touch(t, filepath.Join(home, "bin", "java"))
	if err := os.WriteFile(filepath.Join(home, "release"), []byte("IMPLEMENTOR=\"Ubuntu\"\nJAVA_VERSION=\""+version+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return home
}

func javaSandbox(t *testing.T) {
	t.Helper()
	dir := sandbox(t)
	oldJVM, oldArch := JVMDir, legacyArch
	JVMDir, legacyArch = filepath.Join(dir, "usr/lib/jvm"), "amd64"
	t.Cleanup(func() { JVMDir, legacyArch = oldJVM, oldArch })
	if err := os.MkdirAll(JVMDir, 0o755); err != nil {
		t.Fatal(err)
	}
}

const javaPolicy = `openjdk-17-jdk-headless:
  Installed: 17.0.17+10-1~24.04
  Candidate: 17.0.17+10-1~24.04
  Version table:
openjdk-21-jdk-headless:
  Installed: (none)
  Candidate: 21.0.9+10-1~24.04
openjdk-25-jdk-headless:
  Installed: (none)
  Candidate: 25.0.1+8-1~24.04
openjdk-29-jdk-headless:
  Installed: (none)
  Candidate: (none)
`

func TestParseJavaPolicy(t *testing.T) {
	got := parseJavaPolicy(javaPolicy)
	if len(got) != 3 || got[17] != "17.0.17" || got[21] != "21.0.9" || got[25] != "25.0.1" {
		t.Fatalf("%v", got)
	}
	// Debian 12 has 17 only; a version with an epoch.
	got = parseJavaPolicy("openjdk-17-jdk-headless:\n  Installed: (none)\n  Candidate: 1:17.0.17+10-1~deb12u1\nopenjdk-21-jdk-headless:\n  Installed: (none)\n  Candidate: (none)\n")
	if len(got) != 1 || got[17] != "17.0.17" {
		t.Fatalf("%v", got)
	}
	for _, n := range []int{17, 21, 25, 29, 33} {
		if !javaLTS(n) {
			t.Errorf("%d should be LTS", n)
		}
	}
	for _, n := range []int{8, 11, 18, 22, 26} {
		if javaLTS(n) {
			t.Errorf("%d is not offered as LTS", n)
		}
	}
}

func TestInstalledJava(t *testing.T) {
	javaSandbox(t)
	home := fakeJDK(t, "21", "amd64", "21.0.9")
	fakeJDK(t, "17", "amd64", "17.0.17")
	fakeJDK(t, "17", "arm64", "17.0.1") // multiarch leftover: this machine's wins
	os.Symlink(home, filepath.Join(JVMDir, "default-java"))
	os.Symlink(home, filepath.Join(JVMDir, "java-1.21.0-openjdk-amd64"))
	os.MkdirAll(filepath.Join(JVMDir, "java-11-openjdk-amd64"), 0o755) // no bin/java: dpkg's leftovers
	inst := InstalledJava()
	if len(inst) != 2 || inst[21].Version != "21.0.9" || inst[21].Home != home || !inst[21].LTS {
		t.Fatalf("%+v", inst)
	}
	if inst[17].Version != "17.0.17" || !strings.HasSuffix(inst[17].Home, "amd64") {
		t.Fatalf("17: %+v", inst[17])
	}
	if h, ok := JavaHome(21); !ok || h != home {
		t.Fatalf("JavaHome(21) = %q, %v", h, ok)
	}
	if _, ok := JavaHome(25); ok {
		t.Fatal("Java 25 is not installed")
	}
}

func TestJavaVersions(t *testing.T) {
	javaSandbox(t)
	fakeJDK(t, "17", "amd64", "17.0.17")
	fakeJDK(t, "11", "amd64", "11.0.29") // installed by hand: listed too
	m, f, _ := newTestManager(t)
	f.answers["apt-cache pkgnames openjdk-"] = "openjdk-17-jdk\nopenjdk-21-jre\nopenjdk-29-jdk-headless\nopenjdk-30-jdk-headless\n"
	f.answers["apt-cache policy"] = javaPolicy
	vs := m.JavaVersions(context.Background())
	var got []string
	for _, v := range vs {
		s := v.Version
		if v.Installed {
			s += "*"
		}
		got = append(got, s)
	}
	if strings.Join(got, " ") != "25.0.1 21.0.9 17.0.17* 11.0.29*" {
		t.Fatalf("%v", got)
	}
	// The policy asked for the known LTS versions and 29, not for 30.
	var policy string
	for _, c := range f.cmds {
		if strings.HasPrefix(c, "apt-cache policy") {
			policy = c
		}
	}
	if policy != "apt-cache policy openjdk-17-jdk-headless openjdk-21-jdk-headless openjdk-25-jdk-headless openjdk-29-jdk-headless" {
		t.Fatalf("policy: %q", policy)
	}
}

func TestInstallJava(t *testing.T) {
	javaSandbox(t)
	m, f, _ := newTestManager(t)
	f.answers["apt-cache policy"] = javaPolicy
	osReleaseFile = filepath.Join(t.TempDir(), "os-release")
	os.WriteFile(osReleaseFile, []byte("NAME=\"Ubuntu\"\nVERSION_ID=\"24.04\"\n"), 0o644)
	defer func() { osReleaseFile = "/etc/os-release" }()
	ctx := context.Background()

	// The card's Install: the newest LTS the distribution has.
	fakeJDK(t, "25", "amd64", "25.0.1") // what the package would put there
	if err := m.Install(ctx, "java", &nopLog{}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(f.apt, "|") != "update|install --no-install-recommends openjdk-25-jdk-headless" {
		t.Fatalf("apt: %v", f.apt)
	}
	// A version the distribution does not have.
	f.apt = nil
	err := m.InstallJava(ctx, 29, &nopLog{})
	if code(err) != "java_not_published:Java 29, Ubuntu 24.04" {
		t.Fatalf("29: %v", err)
	}
	// The package installed, but no JDK folder: an error, not success.
	if err := m.InstallJava(ctx, 21, &nopLog{}); err == nil || !strings.Contains(err.Error(), "java-21-openjdk") {
		t.Fatalf("21 without its folder: %v", err)
	}
	if code(m.InstallJava(ctx, 3, &nopLog{})) != "invalid_java_version:3" {
		t.Fatal("bad major accepted")
	}
}

func TestRemoveJavaGuard(t *testing.T) {
	javaSandbox(t)
	fakeJDK(t, "21", "amd64", "21.0.9")
	fakeJDK(t, "17", "amd64", "17.0.17")
	m, f, st := newTestManager(t)
	ctx := context.Background()
	site := &store.Site{Domain: "app.example.com", SysUser: "app_example_com", ProxyPort: 3000, AppCommand: "java -jar app.jar", AppRuntime: "java:21"}
	if err := st.CreateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateSite(ctx, site); err != nil { // CreateSite leaves the runtime out
		t.Fatal(err)
	}
	if code(m.CheckRemoveJava(ctx, 21)) != "java_in_use:21" {
		t.Fatal("Java 21 is used by a site")
	}
	if code(m.CheckRemove(ctx, "java", RemoveOptions{})) != "java_in_use_sites:1" {
		t.Fatal("the card's uninstall must wait for the site")
	}
	if code(m.CheckRemoveJava(ctx, 25)) != "software_not_installed:Java 25" {
		t.Fatal("25 is not installed")
	}
	if err := m.CheckRemoveJava(ctx, 17); err != nil {
		t.Fatalf("17: %v", err)
	}
	f.answers["dpkg-query -W -f ${Package}\t${db:Status-Status}\n openjdk-17"] = "openjdk-17-jdk-headless\tinstalled\nopenjdk-17-jre-headless\tinstalled\n"
	if err := m.RemoveJava(ctx, 17, &nopLog{}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(f.apt, "|") != "purge openjdk-17-jdk-headless openjdk-17-jre-headless" {
		t.Fatalf("apt: %v", f.apt)
	}
	// The card shows the newest version and the site.
	it := m.javaItem([]*store.Site{site})
	if !it.Installed || it.Version != "21.0.9" || it.Lines != 2 || it.Usage["sites"] != 1 || it.Removable || it.Blocked != "java_in_use_sites" {
		t.Fatalf("%+v", it)
	}
	if got := JavaInUse([]*store.Site{site, {AppRuntime: "node:22"}, {AppRuntime: "java:x"}}); len(got) != 1 || got[0] != 21 {
		t.Fatalf("in use: %v", got)
	}
}
