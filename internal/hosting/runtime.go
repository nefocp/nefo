package hosting

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/software"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// App sites run with a runtime: the server's default node (""), a chosen
// Node.js line ("node:22", installed under /opt), or Python with the app's
// own virtual environment in app/.venv ("python").

var runtimeRe = regexp.MustCompile(`^(|python|node:[0-9]{2,3})$`)

var nodeDirRe = regexp.MustCompile(`^node-v([0-9]+)\.[0-9]+\.[0-9]+-linux-(x64|arm64)$`)

// nodeBin returns the bin folder of the newest installed release of a
// Node.js line.
func nodeBin(major int) string {
	entries, _ := os.ReadDir("/opt")
	best := ""
	for _, e := range entries {
		if m := nodeDirRe.FindStringSubmatch(e.Name()); m != nil && m[1] == strconv.Itoa(major) && e.IsDir() {
			if e.Name() > best { // same major: lexical order is fine for a pick
				best = e.Name()
			}
		}
	}
	if best == "" {
		return ""
	}
	return filepath.Join("/opt", best, "bin")
}

// checkRuntime validates a runtime for an app site.
func checkRuntime(rt string) error {
	if !runtimeRe.MatchString(rt) {
		return usererr.New("invalid_runtime", rt)
	}
	if n, ok := strings.CutPrefix(rt, "node:"); ok {
		major, _ := strconv.Atoi(n)
		if nodeBin(major) == "" {
			return usererr.New("node_not_installed", n)
		}
	}
	if rt == "python" && !software.PythonReady() {
		return usererr.New("python_venv_needed")
	}
	return nil
}

// RuntimePath is the PATH of an app site's service and commands: the bin
// folder of its Node.js version, or of its Python virtual environment,
// before the system's.
func (m *Manager) RuntimePath(site *store.Site) string { return m.runtimePath(site) }

func (m *Manager) runtimePath(site *store.Site) string {
	path := "/usr/local/bin:/usr/bin:/bin"
	switch rt := site.AppRuntime; {
	case rt == "python":
		return filepath.Join(m.paths.siteRoot(site.Domain), "app", ".venv", "bin") + ":" + path
	case strings.HasPrefix(rt, "node:"):
		major, _ := strconv.Atoi(strings.TrimPrefix(rt, "node:"))
		if bin := nodeBin(major); bin != "" {
			return bin + ":" + path
		}
	}
	return path
}

// InstallAppDependencies installs an app's dependencies as the site's
// user: npm (package.json) or pip in the app's virtual environment
// (requirements.txt).
func (m *Manager) InstallAppDependencies(ctx context.Context, id int64, lg Logger) error {
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return err
	}
	if site.ProxyPort == 0 {
		return usererr.New("no_app")
	}
	dir := filepath.Join(m.paths.siteRoot(site.Domain), "app")
	path := m.runtimePath(site)
	run := func(args ...string) error {
		full := append([]string{"-u", site.SysUser, "--", "sh", "-c", `umask 027 && cd "$0" && exec "$@"`, dir},
			"env", "HOME="+filepath.Join(m.paths.siteRoot(site.Domain), "tmp"), "PATH="+path)
		full = append(full, args...)
		ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
		defer cancel()
		return m.run.Stream(ctx, lg, "runuser", full...)
	}
	did := false
	if site.AppRuntime == "python" || fileExists(filepath.Join(dir, "requirements.txt")) {
		if !software.PythonReady() {
			return fmt.Errorf("Python apps need the python3-venv package: install Python on the Software page first")
		}
		// Also when pip is missing: an environment made before python3-venv
		// was installed has python but no pip, and venv adds it on a rerun.
		if !fileExists(filepath.Join(dir, ".venv", "bin", "python")) || !fileExists(filepath.Join(dir, ".venv", "bin", "pip")) {
			lg.Logf("Creating the virtual environment app/.venv")
			if err := run("python3", "-m", "venv", ".venv"); err != nil {
				return err
			}
		}
		if fileExists(filepath.Join(dir, "requirements.txt")) {
			lg.Logf("Installing requirements.txt with pip")
			if err := run(".venv/bin/pip", "install", "--no-input", "-r", "requirements.txt"); err != nil {
				return err
			}
		}
		did = true
	}
	if fileExists(filepath.Join(dir, "package.json")) {
		cmd := []string{"npm", "install", "--no-audit", "--no-fund"}
		if fileExists(filepath.Join(dir, "package-lock.json")) {
			cmd = []string{"npm", "ci", "--no-audit", "--no-fund"}
		}
		lg.Logf("Running %s", strings.Join(cmd, " "))
		if err := run(cmd...); err != nil {
			return err
		}
		did = true
	}
	if !did {
		return fmt.Errorf("there is no package.json or requirements.txt in %s", dir)
	}
	lg.Logf("Dependencies are installed. Restart the app to use them.")
	return nil
}
