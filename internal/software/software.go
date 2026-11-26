// Package software installs extra server software: more PHP versions,
// Redis, Node.js and Docker.
package software

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/software/apt"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
	"github.com/nefocp/nefo/internal/version"
)

// PHPVersions are the versions offered for installation.
var PHPVersions = []string{"7.4", "8.0", "8.1", "8.2", "8.3", "8.4", "8.5"}

// Item is one installable component.
type Item struct {
	ID        string `json:"id"`   // php8.2, redis, nodejs, docker
	Kind      string `json:"kind"` // php, redis, nodejs, docker
	Version   string `json:"version"`
	Installed bool   `json:"installed"`
	// Stopped means it is installed but its service is not running (it
	// failed, or someone stopped it on the Services page).
	Stopped bool `json:"stopped,omitempty"`
	// ThirdParty means the packages come from an extra repository
	// (ppa:ondrej/php or packages.sury.org) rather than the distribution.
	ThirdParty bool `json:"third_party"`
	UsedBy     int  `json:"used_by"` // websites using this PHP version
	// Removable means the panel can uninstall it now; otherwise Blocked is
	// the error code (with BlockedDetail) that a removal would get.
	Removable     bool   `json:"removable"`
	Blocked       string `json:"blocked,omitempty"`
	BlockedDetail string `json:"blocked_detail,omitempty"`
	// Group is where the Software page shows it: php, data, runtimes,
	// containers or security.
	Group string `json:"group"`
	// RAMMB is a rough idea of the memory it uses while running, for
	// choosing what to install; 0 means nothing runs while it is idle.
	// RAMMax means it is a configured upper limit.
	RAMMB  int  `json:"ram_mb"`
	RAMMax bool `json:"ram_max,omitempty"`
	// DataDir is the data an uninstall keeps unless asked to delete it,
	// HasData whether it exists.
	DataDir string `json:"data_dir,omitempty"`
	HasData bool   `json:"has_data,omitempty"`
	// Usage counts what uses it, such as {"sites": 2} or {"containers": 3}.
	Usage map[string]int `json:"usage,omitempty"`
	// Lines is the number of Node.js release lines installed in /opt.
	Lines int `json:"lines,omitempty"`
}

// Block marks an installed item as not removable for err (a usererr).
func (it *Item) Block(err error) {
	if err == nil {
		it.Removable = it.Installed
		return
	}
	it.Removable = false
	var ue *usererr.Error
	if errors.As(err, &ue) {
		it.Blocked, it.BlockedDetail = ue.Code, ue.Detail
	} else {
		it.Blocked, it.BlockedDetail = "operation_failed", err.Error()
	}
}

// Logger receives progress messages for a job log.
type Logger interface {
	io.Writer
	Logf(format string, args ...any)
}

// Manager installs software.
type Manager struct {
	run   run.Runner
	store *store.Store
	mu    sync.Mutex
	// reloadPHP is called after PHP extensions change.
	distroPHP func(ctx context.Context) string
}

// New returns a Manager. distroPHP returns the distribution's PHP version.
func New(r run.Runner, st *store.Store, distroPHP func(context.Context) string) *Manager {
	return &Manager{run: r, store: st, distroPHP: distroPHP}
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// phpEtc holds each PHP version's configuration; tests point it elsewhere.
var phpEtc = "/etc/php"

func phpInstalled(v string) bool { return exists(filepath.Join(phpEtc, v, "fpm", "php-fpm.conf")) }

// Catalog lists what can be installed and what is.
func (m *Manager) Catalog(ctx context.Context) ([]Item, error) {
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		return nil, err
	}
	used := map[string]int{}
	for _, s := range sites {
		used[s.PHPVersion]++
	}
	distro := m.distroPHP(ctx)
	installedCount := 0
	for _, v := range PHPVersions {
		if phpInstalled(v) {
			installedCount++
		}
	}
	items := []Item{}
	for _, v := range PHPVersions {
		it := Item{ID: "php" + v, Kind: "php", Group: "php", Version: v, Installed: phpInstalled(v), ThirdParty: v != distro,
			UsedBy: used[v], RAMMB: 25}
		if it.Installed {
			it.Block(phpBlocker(v, it.UsedBy, installedCount))
		}
		items = append(items, it)
	}
	redis := Item{ID: "redis", Kind: "redis", Group: "data", Installed: RedisInstalled(), RAMMB: redisMemoryMB(), RAMMax: true}
	if redis.Installed {
		if out, err := m.run.Output(ctx, "redis-server", "--version"); err == nil {
			redis.Version = firstMatch(`v=([0-9.]+)`, out)
		}
		redis.Block(nil)
		redis.Stopped = !m.unitActive(ctx, "redis-server")
	}
	node := Item{ID: "nodejs", Kind: "nodejs", Group: "runtimes", Installed: NodeInstalled(), Lines: len(InstalledNode())}
	if node.Installed {
		if out, err := m.run.Output(ctx, filepath.Join(nodeLinkDir, "node"), "--version"); err == nil {
			node.Version = strings.TrimPrefix(strings.TrimSpace(out), "v")
		}
		n := defaultNodeUsers(sites)
		node.Usage = map[string]int{"sites": n}
		_, major, err := defaultNode()
		if err == nil {
			err = nodeBlocker(major, n)
		}
		node.Block(err)
	}
	py := Item{ID: "python", Kind: "python", Group: "runtimes", Installed: PythonReady()}
	if py.Installed {
		py.Version = pythonVersion()
		n := pythonUsers(sites)
		py.Usage = map[string]int{"sites": n}
		py.Block(pythonBlocker(n))
	}
	docker := Item{ID: "docker", Kind: "docker", Group: "containers", Installed: DockerInstalled(), RAMMB: 90,
		DataDir: dockerDataDirs[0], HasData: exists(dockerDataDirs[0])}
	if docker.Installed {
		if out, err := m.run.Output(ctx, "docker", "--version"); err == nil {
			docker.Version = firstMatch(`version ([0-9.]+)`, out)
		}
		u := m.dockerUsage(ctx, sites)
		docker.Usage = map[string]int{"apps": u.apps, "projects": u.projects, "containers": u.containers}
		docker.Block(u.blocker())
		docker.Stopped = !m.unitActive(ctx, "docker")
	}
	return append(items, redis, node, py, docker), nil
}

// unitActive reports whether a systemd unit is running.
func (m *Manager) unitActive(ctx context.Context, unit string) bool {
	out, _ := m.run.Output(ctx, "systemctl", "is-active", unit)
	return strings.TrimSpace(out) == "active"
}

// redisMemoryMB is the memory limit installRedis gives Redis on this server.
func redisMemoryMB() int {
	switch t := memTotal(); {
	case t >= 3<<30:
		return 256
	case t >= 1500<<20:
		return 128
	}
	return 64
}

func firstMatch(re, s string) string {
	if m := regexp.MustCompile(re).FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

// Install installs one item.
func (m *Manager) Install(ctx context.Context, id string, lg Logger) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch {
	case strings.HasPrefix(id, "php"):
		return m.installPHP(ctx, strings.TrimPrefix(id, "php"), lg)
	case id == "redis":
		return m.installRedis(ctx, lg)
	case id == "nodejs":
		return m.installNode(ctx, lg)
	case id == "docker":
		return m.installDocker(ctx, lg)
	}
	return usererr.New("unknown_software", id)
}

// ---- PHP ----

// PHPExtensions are installed with every PHP version.
var PHPExtensions = []string{"fpm", "cli", "mysql", "curl", "gd", "mbstring", "xml", "zip", "intl", "bcmath", "opcache"}

func (m *Manager) installPHP(ctx context.Context, v string, lg Logger) error {
	if validate.PHPVersion(v) != nil || !slices.Contains(PHPVersions, v) {
		return usererr.New("unknown_software", "php"+v)
	}
	if v != m.distroPHP(ctx) {
		if err := m.addPHPRepository(ctx, lg); err != nil {
			return err
		}
	}
	if err := run.Apt(ctx, m.run, lg, "update"); err != nil {
		return err
	}
	pkgs := []string{}
	for _, e := range PHPExtensions {
		pkgs = append(pkgs, "php"+v+"-"+e)
	}
	if RedisInstalled() {
		pkgs = append(pkgs, "php"+v+"-redis")
	}
	lg.Logf("Installing PHP %s", v)
	if err := run.Apt(ctx, m.run, lg, append([]string{"install", "--no-install-recommends"}, pkgs...)...); err != nil {
		return err
	}
	if err := TunePHPPool(v); err != nil {
		lg.Logf("Warning: %v", err)
	}
	if _, err := m.run.Output(ctx, "systemctl", "enable", "--now", "php"+v+"-fpm"); err != nil {
		return err
	}
	m.run.Output(ctx, "systemctl", "restart", "php"+v+"-fpm")
	lg.Logf("PHP %s is installed. Choose it in a website's settings.", v)
	return nil
}

// TunePHPPool switches the distribution's default "www" pool to on-demand
// workers, so an unused PHP version costs almost no memory.
func TunePHPPool(v string) error {
	p := "/etc/php/" + v + "/fpm/pool.d/www.conf"
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	re := regexp.MustCompile(`(?m)^pm = (dynamic|static)$`)
	if !re.Match(b) {
		return nil
	}
	return fsutil.WriteFileAtomic(p, re.ReplaceAll(b, []byte("pm = ondemand")), 0o644)
}

// addPHPRepository adds ppa:ondrej/php on Ubuntu or packages.sury.org on
// Debian, which provide every supported PHP version.
func (m *Manager) addPHPRepository(ctx context.Context, lg Logger) error {
	osr, _ := os.ReadFile("/etc/os-release")
	id, codename := osField(osr, "ID"), osField(osr, "VERSION_CODENAME")
	switch id {
	case "ubuntu":
		if matches, _ := filepath.Glob("/etc/apt/sources.list.d/ondrej-*php*"); len(matches) > 0 {
			return nil
		}
		lg.Logf("Adding the PHP repository ppa:ondrej/php")
		return m.run.Stream(ctx, lg, "/bin/sh", "-c", `umask 022 && exec add-apt-repository -y "$1"`, "sh", "ppa:ondrej/php")
	case "debian":
		if !regexp.MustCompile(`^[a-z]{2,20}$`).MatchString(codename) {
			return errors.New("unknown Debian release")
		}
		keyring := "/usr/share/keyrings/sury-php.gpg"
		if exists(keyring) {
			return nil
		}
		lg.Logf("Adding the PHP repository packages.sury.org")
		key, err := download(ctx, "https://packages.sury.org/php/apt.gpg", 1<<20)
		if err != nil {
			return err
		}
		if err := fsutil.WriteFileAtomic(keyring, key, 0o644); err != nil {
			return err
		}
		src := fmt.Sprintf("deb [signed-by=%s] https://packages.sury.org/php/ %s main\n", keyring, codename)
		return fsutil.WriteFileAtomic("/etc/apt/sources.list.d/sury-php.list", []byte(src), 0o644)
	}
	return usererr.New("unsupported")
}

func osField(b []byte, key string) string {
	for _, line := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(line, key+"="); ok {
			return strings.Trim(v, `"`)
		}
	}
	return ""
}

// RemovePHP uninstalls a PHP version no website uses, with its extensions
// and the libraries only they needed.
func (m *Manager) RemovePHP(ctx context.Context, v string, lg Logger) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.phpRemovable(ctx, v); err != nil {
		return err
	}
	// php8.2 is the version's metapackage; libapache2-mod-php8.2 depends on
	// php8.2-common, so leaving it out would make apt refuse.
	pkgs, err := apt.Installed(ctx, m.run, true, "php"+v, "php"+v+"-*", "libapache2-mod-php"+v)
	if err != nil {
		return err
	}
	if err := apt.Check(ctx, m.run, true, pkgs...); err != nil {
		return err
	}
	// The package scripts stop it too, unless a policy forbids them.
	m.run.Output(ctx, "systemctl", "disable", "--now", "php"+v+"-fpm")
	lg.Logf("Removing PHP %s", v)
	if err := apt.Remove(ctx, m.run, lg, true, pkgs...); err != nil {
		return err
	}
	lg.Logf("PHP %s is removed.", v)
	return nil
}

// phpRemovable returns why PHP v cannot be removed now, or nil.
func (m *Manager) phpRemovable(ctx context.Context, v string) error {
	if validate.PHPVersion(v) != nil || !slices.Contains(PHPVersions, v) {
		return usererr.New("unknown_software", "php"+v)
	}
	if !phpInstalled(v) {
		return usererr.New("software_not_installed", "PHP "+v)
	}
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		return err
	}
	used := 0
	for _, s := range sites {
		if s.PHPVersion == v {
			used++
		}
	}
	n := 0
	for _, pv := range PHPVersions {
		if phpInstalled(pv) {
			n++
		}
	}
	return phpBlocker(v, used, n)
}

// phpBlocker: a version websites use stays, and so does the last one,
// which the web server's setup and Adminer run on.
func phpBlocker(v string, usedBy, installed int) error {
	if usedBy > 0 {
		return usererr.New("php_in_use", v)
	}
	if installed <= 1 {
		return usererr.New("php_last", v)
	}
	return nil
}

// ---- Redis ----

func (m *Manager) installRedis(ctx context.Context, lg Logger) error {
	if err := run.Apt(ctx, m.run, lg, "update"); err != nil {
		return err
	}
	pkgs := []string{"redis-server"}
	var versions []string
	for _, v := range PHPVersions {
		if phpInstalled(v) {
			pkgs = append(pkgs, "php"+v+"-redis")
			versions = append(versions, v)
		}
	}
	lg.Logf("Installing Redis")
	if err := run.Apt(ctx, m.run, lg, append([]string{"install", "--no-install-recommends"}, pkgs...)...); err != nil {
		return err
	}
	// Usually apt has started it already; not where a policy forbids that.
	if _, err := m.run.Output(ctx, "systemctl", "enable", "--now", "redis-server"); err != nil {
		return err
	}
	mem := strconv.Itoa(redisMemoryMB()) + "mb"
	lg.Logf("Limiting Redis to %s of memory, used as a cache (least recently used keys are dropped)", mem)
	for _, args := range [][]string{{"CONFIG", "SET", "maxmemory", mem}, {"CONFIG", "SET", "maxmemory-policy", "allkeys-lru"}, {"CONFIG", "REWRITE"}} {
		if _, err := m.run.Output(ctx, "redis-cli", args...); err != nil {
			return err
		}
	}
	for _, v := range versions {
		m.run.Output(ctx, "systemctl", "reload", "php"+v+"-fpm")
	}
	lg.Logf("Redis is running on 127.0.0.1:6379. WordPress can use it with a Redis object cache plugin.")
	return nil
}

func memTotal() int64 {
	b, _ := os.ReadFile("/proc/meminfo")
	for _, line := range strings.Split(string(b), "\n") {
		if f := strings.Fields(line); len(f) >= 2 && f[0] == "MemTotal:" {
			kb, _ := strconv.ParseInt(f[1], 10, 64)
			return kb << 10
		}
	}
	return 0
}

// ---- Node.js ----

// installNode installs the newest LTS release of Node.js from nodejs.org,
// checked against the published SHA-256, into /opt, with node, npm and npx
// linked into /usr/local/bin.
func (m *Manager) installNode(ctx context.Context, lg Logger) error {
	arch := map[string]string{"amd64": "x64", "arm64": "arm64"}[runtime.GOARCH]
	if arch == "" {
		return usererr.New("unsupported")
	}
	lg.Logf("Looking up the latest LTS release on nodejs.org")
	idx, err := download(ctx, "https://nodejs.org/dist/index.json", 4<<20)
	if err != nil {
		return err
	}
	var releases []struct {
		Version string `json:"version"`
		LTS     any    `json:"lts"`
	}
	if err := json.Unmarshal(idx, &releases); err != nil {
		return err
	}
	ver := ""
	for _, r := range releases {
		if lts, ok := r.LTS.(string); ok && lts != "" && regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(r.Version) {
			ver = r.Version
			break
		}
	}
	if ver == "" {
		return errors.New("no LTS release found on nodejs.org")
	}
	name := "node-" + ver + "-linux-" + arch
	base := "https://nodejs.org/dist/" + ver + "/"
	sums, err := download(ctx, base+"SHASUMS256.txt", 1<<20)
	if err != nil {
		return err
	}
	want := ""
	sc := bufio.NewScanner(strings.NewReader(string(sums)))
	for sc.Scan() {
		if f := strings.Fields(sc.Text()); len(f) == 2 && f[1] == name+".tar.gz" {
			want = f[0]
		}
	}
	if len(want) != 64 {
		return errors.New("SHASUMS256.txt has no entry for " + name + ".tar.gz")
	}
	lg.Logf("Downloading Node.js %s", ver)
	// Stream to disk while hashing, so the archive is never held in memory.
	tmp, err := os.CreateTemp("", "node-*.tar.gz")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	h := sha256.New()
	err = downloadTo(ctx, base+name+".tar.gz", 200<<20, io.MultiWriter(tmp, h))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != want {
		return errors.New("the Node.js download does not match its published checksum")
	}
	lg.Logf("Extracting to /opt/%s", name)
	// The archive is verified, and npm needs the symlinks it contains, so the
	// system tar unpacks it.
	if _, err := m.run.Output(ctx, "/bin/sh", "-c", `umask 022 && exec tar -xzf "$1" -C /opt --no-same-owner`, "sh", tmp.Name()); err != nil {
		return err
	}
	if err := m.nodeLibraries(ctx, lg); err != nil {
		return err
	}
	for _, bin := range []string{"node", "npm", "npx", "corepack"} {
		link := "/usr/local/bin/" + bin
		os.Remove(link)
		if err := os.Symlink("/opt/"+name+"/bin/"+bin, link); err != nil && bin != "corepack" {
			return err
		}
	}
	lg.Logf("Node.js %s is installed: node, npm and npx are in /usr/local/bin.", ver)
	return nil
}

// ---- Docker ----

func (m *Manager) installDocker(ctx context.Context, lg Logger) error {
	if err := run.Apt(ctx, m.run, lg, "update"); err != nil {
		return err
	}
	lg.Logf("Installing Docker from the distribution's packages")
	pkgs := []string{"docker.io"}
	if out, _ := m.run.Output(ctx, "apt-cache", "show", "docker-compose-v2"); strings.Contains(out, "Package:") {
		pkgs = append(pkgs, "docker-compose-v2")
	}
	if err := run.Apt(ctx, m.run, lg, append([]string{"install", "--no-install-recommends"}, pkgs...)...); err != nil {
		return err
	}
	if _, err := m.run.Output(ctx, "systemctl", "enable", "--now", "docker"); err != nil {
		return err
	}
	lg.Logf("Docker is running. Note: ports published by containers bypass ufw; publish them on 127.0.0.1 and use a website as a proxy.")
	return nil
}

func download(ctx context.Context, url string, max int64) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", version.AppName+"/"+version.Version)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, fmt.Errorf("download %s: larger than expected", url)
	}
	return b, nil
}

// downloadTo streams a download into w, refusing more than max bytes.
func downloadTo(ctx context.Context, url string, max int64, w io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", version.AppName+"/"+version.Version)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	n, err := io.Copy(w, io.LimitReader(resp.Body, max+1))
	if err == nil && n > max {
		err = fmt.Errorf("download %s: larger than expected", url)
	}
	return err
}
