// Package gitdeploy deploys a website from a Git repository: on demand, or
// when the repository's host (GitHub, GitLab, Gitea, Forgejo) sends a
// webhook after a push.
//
// Everything that touches the site's files runs as the site's Linux user:
// git itself (repository hooks run as that user, never as root), the deploy
// key generation and the administrator's deploy script. The panel, as root,
// only reads the folder and removes its own files, through os.Root so that
// symlinks planted in the folder cannot lead it elsewhere.
package gitdeploy

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/ssh"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

const (
	// TargetRoot deploys into the site folder, for projects with their own
	// public/ folder (Laravel, Symfony…); TargetPublic into the web folder;
	// TargetApp into app/, the folder an app site (Node.js, Python) runs
	// its program from. App sites always deploy there (see siteTarget).
	TargetRoot   = "root"
	TargetPublic = "public"
	TargetApp    = "app"

	StatusOK     = "ok"
	StatusFailed = "failed"

	// JobTimeout is the longest a deploy job may take: the fetch, the
	// script (ScriptTimeout) and the small steps around them.
	JobTimeout    = 30 * time.Minute
	ScriptTimeout = 15 * time.Minute
	fetchTimeout  = 10 * time.Minute
	stepTimeout   = 2 * time.Minute

	// The deploy key and its known_hosts file, in <site root>/.ssh.
	sshDir      = ".ssh"
	keyName     = "panel_deploy"
	knownHosts  = "known_hosts"
	maxKeyBytes = 16 << 10

	// HookPath is the webhook's path on the site's own domain; the site ID follows.
	HookPath = "/.panel/deploy/"

	userPATH = "/usr/local/bin:/usr/bin:/bin"
	// placeholderMark is in the welcome page the panel puts in a new site
	// (see hosting's templates.go).
	placeholderMark = "placeholder page was created by the server's control panel"
)

// Logger receives a deploy's progress: lines from Logf and program output
// through Write. A *jobs.Job is one.
type Logger interface {
	io.Writer
	Logf(format string, args ...any)
}

// Store is the part of the panel database the deploys use; *store.Store
// implements it.
type Store interface {
	SiteByID(ctx context.Context, id int64) (*store.Site, error)
	GitDeploy(ctx context.Context, siteID int64) (*store.GitDeploy, error)
	SaveGitDeploy(ctx context.Context, g *store.GitDeploy) error
	SetGitDeployResult(ctx context.Context, siteID int64, commit, status string, at time.Time) error
	DeleteGitDeploy(ctx context.Context, siteID int64) error
}

// Manager configures and runs Git deploys. It keeps no state of its own.
type Manager struct {
	run    run.Runner
	store  Store
	root   func(domain string) string
	now    func() time.Time
	hasGit func() bool
	// appPath gives the PATH of an app site (Node.js or Python): with the
	// Node.js version picked for it, or its virtual environment, first.
	appPath func(site *store.Site) string
}

// SetAppPath sets how the PATH of an app site's deploy script is found
// (hosting.Manager.RuntimePath), so the script's npm, node, pip or python
// are the ones the app runs with.
func (m *Manager) SetAppPath(fn func(site *store.Site) string) { m.appPath = fn }

// New returns a Manager. root gives a site's folder (hosting.Manager.Root).
func New(r run.Runner, st Store, root func(domain string) string) *Manager {
	return &Manager{
		run: r, store: st, root: root, now: time.Now,
		hasGit: func() bool { _, err := os.Stat("/usr/bin/git"); return err == nil },
	}
}

// Settings is what the administrator chooses.
type Settings struct {
	Repo   string
	Branch string
	Target string
	Script string
}

// Info describes a site's Git deploy for the UI.
type Info struct {
	Configured bool   `json:"configured"`
	Repo       string `json:"repo"`
	Branch     string `json:"branch"`
	Target     string `json:"target"`
	Script     string `json:"script"`
	// SSH is true for an SSH address, which needs the deploy key.
	SSH bool `json:"ssh"`
	// Folder is where the repository goes; RootFolder and PublicFolder
	// are the two choices. An app site has no choice: AppFolder, set only
	// for app sites, is where its program runs and its repository goes.
	Folder       string `json:"folder"`
	RootFolder   string `json:"root_folder"`
	PublicFolder string `json:"public_folder"`
	AppFolder    string `json:"app_folder,omitempty"`
	// PublicKey is the deploy key to add to the repository (read-only).
	PublicKey      string `json:"public_key"`
	KeyFingerprint string `json:"key_fingerprint"`
	WebhookURL     string `json:"webhook_url"`
	WebhookSecret  string `json:"webhook_secret"`
	// The last deploy.
	LastCommit   string     `json:"last_commit"`
	LastSubject  string     `json:"last_subject"`
	LastDeployAt *time.Time `json:"last_deploy_at"`
	LastStatus   string     `json:"last_status"`
}

// WebhookURL is the address the repository's host calls after a push. It
// is on the site's own domain, because the panel's port is firewalled.
func WebhookURL(site *store.Site) string {
	scheme := "http"
	if site.SSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s%s%d", scheme, site.Domain, HookPath, site.ID)
}

// SplitCommit splits a stored last commit into its hash and subject line.
func SplitCommit(c string) (hash, subject string) {
	hash, subject, _ = strings.Cut(c, " ")
	return hash, subject
}

func newSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// site loads a site and checks the names that end up in paths and commands.
func (m *Manager) site(ctx context.Context, id int64) (*store.Site, error) {
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if validate.Domain(site.Domain) != nil || validate.SystemUsername(site.SysUser) != nil {
		return nil, fmt.Errorf("invalid site %q", site.Domain)
	}
	return site, nil
}

func (m *Manager) folder(site *store.Site, target string) string {
	root := m.root(site.Domain)
	switch target {
	case TargetRoot:
		return root
	case TargetApp:
		return filepath.Join(root, "app")
	}
	return filepath.Join(root, "public")
}

// isAppSite reports whether a site runs a program behind the web server
// (Node.js, Python…). Its systemd service runs in app/.
func isAppSite(site *store.Site) bool { return site.ProxyPort != 0 }

// siteTarget returns where a site's repository goes: an app site's always
// into app/, where its program runs; another site's into the saved folder.
func siteTarget(site *store.Site, saved string) string {
	if isAppSite(site) {
		return TargetApp
	}
	return saved
}

func (m *Manager) keyFile(site *store.Site) string {
	return filepath.Join(m.root(site.Domain), sshDir, keyName)
}

// Info returns a site's Git deploy settings, its deploy key and webhook.
func (m *Manager) Info(ctx context.Context, siteID int64) (*Info, error) {
	site, err := m.site(ctx, siteID)
	if err != nil {
		return nil, err
	}
	g, err := m.store.GitDeploy(ctx, siteID)
	if errors.Is(err, store.ErrNotFound) {
		g, err = nil, nil
	}
	if err != nil {
		return nil, err
	}
	return m.info(site, g), nil
}

func (m *Manager) info(site *store.Site, g *store.GitDeploy) *Info {
	in := &Info{
		Branch: "main", Target: siteTarget(site, TargetPublic),
		RootFolder: m.root(site.Domain), PublicFolder: m.folder(site, TargetPublic),
	}
	if isAppSite(site) {
		in.AppFolder = m.folder(site, TargetApp)
	}
	if g == nil {
		in.Folder = m.folder(site, in.Target)
		return in
	}
	in.Configured = true
	in.Repo, in.Branch, in.Target, in.Script = g.Repo, g.Branch, siteTarget(site, g.Target), g.Script
	in.SSH = IsSSH(g.Repo)
	in.Folder = m.folder(site, in.Target)
	in.PublicKey, in.KeyFingerprint = m.publicKey(site)
	in.WebhookURL, in.WebhookSecret = WebhookURL(site), g.HookSecret
	in.LastCommit, in.LastSubject = SplitCommit(g.LastCommit)
	in.LastStatus = g.LastStatus
	if !g.LastDeployAt.IsZero() {
		t := g.LastDeployAt
		in.LastDeployAt = &t
	}
	return in
}

// Save stores a site's settings. The first time, it also creates the
// webhook secret and the deploy key.
func (m *Manager) Save(ctx context.Context, siteID int64, in Settings) (*Info, error) {
	site, err := m.site(ctx, siteID)
	if err != nil {
		return nil, err
	}
	repo, err := ValidateRepo(in.Repo)
	if err != nil {
		return nil, err
	}
	branch := strings.TrimSpace(in.Branch)
	if err := ValidateBranch(branch); err != nil {
		return nil, err
	}
	target := TargetApp // an app site has no choice
	if !isAppSite(site) {
		if err := ValidateTarget(in.Target); err != nil {
			return nil, err
		}
		target = in.Target
	}
	script, err := CleanScript(in.Script)
	if err != nil {
		return nil, err
	}
	old, err := m.store.GitDeploy(ctx, siteID)
	if errors.Is(err, store.ErrNotFound) {
		old, err = &store.GitDeploy{}, nil
	}
	if err != nil {
		return nil, err
	}
	g := &store.GitDeploy{
		SiteID: siteID, Repo: repo, Branch: branch, Target: target, Script: script,
		HookSecret: old.HookSecret,
	}
	if g.HookSecret == "" {
		g.HookSecret = newSecret()
	}
	if err := m.store.SaveGitDeploy(ctx, g); err != nil {
		return nil, err
	}
	if pub, _ := m.publicKey(site); pub == "" {
		if err := m.generateKey(ctx, site); err != nil {
			return nil, err
		}
	}
	return m.Info(ctx, siteID)
}

// NewKey replaces the site's deploy key. The old one stops working, so the
// new one must be added to the repository.
func (m *Manager) NewKey(ctx context.Context, siteID int64) (*Info, error) {
	site, err := m.configured(ctx, siteID)
	if err != nil {
		return nil, err
	}
	if err := m.generateKey(ctx, site); err != nil {
		return nil, err
	}
	return m.Info(ctx, siteID)
}

// NewSecret replaces the webhook secret.
func (m *Manager) NewSecret(ctx context.Context, siteID int64) (*Info, error) {
	if _, err := m.configured(ctx, siteID); err != nil {
		return nil, err
	}
	g, err := m.store.GitDeploy(ctx, siteID)
	if err != nil {
		return nil, err
	}
	g.HookSecret = newSecret()
	if err := m.store.SaveGitDeploy(ctx, g); err != nil {
		return nil, err
	}
	return m.Info(ctx, siteID)
}

// Remove forgets a site's Git settings and deletes the deploy key. The
// deployed files, and the repository in them, stay.
func (m *Manager) Remove(ctx context.Context, siteID int64) error {
	site, err := m.site(ctx, siteID)
	if err != nil {
		return err
	}
	if err := m.store.DeleteGitDeploy(ctx, siteID); err != nil {
		return err
	}
	return m.removeKey(site)
}

// SiteDeleted forgets the settings of a site that is being deleted. The
// key goes with the site's folder.
func (m *Manager) SiteDeleted(ctx context.Context, siteID int64) error {
	return m.store.DeleteGitDeploy(ctx, siteID)
}

// configured returns the site of a configured Git deploy.
func (m *Manager) configured(ctx context.Context, siteID int64) (*store.Site, error) {
	site, err := m.site(ctx, siteID)
	if err != nil {
		return nil, err
	}
	if _, err := m.store.GitDeploy(ctx, siteID); errors.Is(err, store.ErrNotFound) {
		return nil, usererr.New("git_not_configured")
	} else if err != nil {
		return nil, err
	}
	return site, nil
}

// CheckDeploy is called before a deploy job starts.
func (m *Manager) CheckDeploy(ctx context.Context, siteID int64) (*store.Site, error) {
	return m.configured(ctx, siteID)
}

// ---- deploy key ----

// generateKey creates a new ed25519 key pair as the site's user, replacing
// the old one. The shell script is a constant; the paths and the comment
// are passed as separate arguments.
func (m *Manager) generateKey(ctx context.Context, site *store.Site) error {
	key := m.keyFile(site)
	const script = `umask 077 && mkdir -p -m 700 -- "$1" && chmod 700 -- "$1" && rm -f -- "$2" "$2.pub" && ` +
		`exec ssh-keygen -q -t ed25519 -N "" -C "$3" -f "$2"`
	ctx, cancel := context.WithTimeout(ctx, stepTimeout)
	defer cancel()
	out, err := m.run.Output(ctx, "runuser", "-u", site.SysUser, "--",
		"sh", "-c", script, "sh", filepath.Dir(key), key, "deploy@"+site.Domain)
	if err != nil {
		return fmt.Errorf("could not create the deploy key: %s", strings.TrimSpace(run.Tail(out, 5)))
	}
	return nil
}

// publicKey reads the deploy key's public half, through os.Root so a
// symlink cannot make the panel read a file outside the site.
func (m *Manager) publicKey(site *store.Site) (key, fingerprint string) {
	r, err := os.OpenRoot(m.root(site.Domain))
	if err != nil {
		return "", ""
	}
	defer r.Close()
	f, err := r.Open(filepath.Join(sshDir, keyName+".pub"))
	if err != nil {
		return "", ""
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxKeyBytes))
	if err != nil {
		return "", ""
	}
	pub, comment, _, _, err := ssh.ParseAuthorizedKey(b)
	if err != nil {
		return "", ""
	}
	comment = strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, comment)
	key = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))
	if comment != "" {
		key += " " + comment
	}
	return key, ssh.FingerprintSHA256(pub)
}

// removeKey deletes the key pair and the known_hosts file, and the .ssh
// folder if nothing else is in it.
func (m *Manager) removeKey(site *store.Site) error {
	r, err := os.OpenRoot(m.root(site.Domain))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer r.Close()
	for _, name := range []string{keyName, keyName + ".pub", knownHosts} {
		if err := r.Remove(filepath.Join(sshDir, name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	if st, err := r.Lstat(sshDir); err == nil && st.IsDir() {
		r.Remove(sshDir) // fails, as it should, if the folder is not empty
	}
	return nil
}

// ---- running commands as the site's user ----

// shellQuote quotes a path for GIT_SSH_COMMAND, which git runs with a shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (m *Manager) sshCommand(site *store.Site) string {
	dir := filepath.Join(m.root(site.Domain), sshDir)
	return "ssh -F /dev/null -i " + shellQuote(filepath.Join(dir, keyName)) +
		" -o IdentitiesOnly=yes -o UserKnownHostsFile=" + shellQuote(filepath.Join(dir, knownHosts)) +
		" -o StrictHostKeyChecking=accept-new -o BatchMode=yes -o ConnectTimeout=30"
}

func (m *Manager) binDir(site *store.Site) string {
	return filepath.Join(m.root(site.Domain), "tmp", ".panel-bin")
}

// userArgs builds the arguments of runuser for a command run as the site's
// user in dir: "runuser -u <user> -- sh -c 'umask 027 && exec "$@"' sh env
// --chdir=<dir> HOME=… <command>". The shell only sets the umask (files
// readable by Nginx through the site's group); the command comes as
// separate arguments.
func (m *Manager) userArgs(site *store.Site, dir string, extraEnv []string, command ...string) []string {
	tmp := filepath.Join(m.root(site.Domain), "tmp")
	path := userPATH
	switch {
	case site.PHPVersion != "":
		path = m.binDir(site) + ":" + path // "php" is the site's PHP version, see phpShim
	case site.ProxyPort != 0 && m.appPath != nil:
		path = m.appPath(site)
	}
	args := []string{"-u", site.SysUser, "--", "sh", "-c", `umask 027 && exec "$@"`, "sh",
		"env", "--chdir=" + dir,
		"HOME=" + tmp, "TMPDIR=" + tmp, "PATH=" + path,
		"COMPOSER_HOME=" + filepath.Join(tmp, ".composer"), "COMPOSER_NO_INTERACTION=1",
		// No password prompts, and no transports but https and ssh (also
		// for submodules a script may fetch).
		"GIT_TERMINAL_PROMPT=0", "GIT_ALLOW_PROTOCOL=https:ssh",
		"GIT_SSH_COMMAND=" + m.sshCommand(site),
	}
	args = append(args, extraEnv...)
	return append(args, command...)
}

func (m *Manager) gitOutput(ctx context.Context, site *store.Site, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, stepTimeout)
	defer cancel()
	return m.run.Output(ctx, "runuser", m.userArgs(site, dir, nil, append([]string{"git"}, args...)...)...)
}

func (m *Manager) gitStream(ctx context.Context, w io.Writer, timeout time.Duration, site *store.Site, dir string, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return m.run.Stream(ctx, w, "runuser", m.userArgs(site, dir, nil, append([]string{"git"}, args...)...)...)
}

// tailWriter passes output on and keeps its last 8 KB.
type tailWriter struct {
	w io.Writer
	b []byte
}

func (t *tailWriter) Write(p []byte) (int, error) {
	t.b = append(t.b, p...)
	if len(t.b) > 8<<10 {
		t.b = t.b[len(t.b)-8<<10:]
	}
	return t.w.Write(p)
}

// phpShim makes "php" in the deploy script the site's own PHP version
// (also for Composer, which starts with "#!/usr/bin/env php"), not the
// server's default one.
func (m *Manager) phpShim(ctx context.Context, site *store.Site) error {
	if site.PHPVersion == "" || validate.PHPVersion(site.PHPVersion) != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, stepTimeout)
	defer cancel()
	_, err := m.run.Output(ctx, "runuser", m.userArgs(site, m.root(site.Domain), nil,
		"sh", "-c", `mkdir -p -- "$1" && ln -sfn -- "$2" "$1/php"`, "sh", m.binDir(site), "/usr/bin/php"+site.PHPVersion)...)
	return err
}

// ---- the deploy ----

// Deploy fetches the configured branch into the target folder and runs the
// deploy script. It returns the deployed commit. Errors are sentences for
// the job log.
func (m *Manager) Deploy(ctx context.Context, siteID int64, lg Logger) (commit string, err error) {
	site, err := m.site(ctx, siteID)
	if err != nil {
		return "", err
	}
	g, err := m.store.GitDeploy(ctx, siteID)
	if errors.Is(err, store.ErrNotFound) {
		return "", errors.New("Git deploy is not set up for this website.")
	}
	if err != nil {
		return "", err
	}
	last := g.LastCommit
	defer func() {
		status := StatusOK
		if err != nil {
			status = StatusFailed
		}
		if serr := m.store.SetGitDeployResult(context.WithoutCancel(ctx), siteID, last, status, m.now()); serr != nil && err == nil {
			err = serr
		}
	}()
	if _, err := ValidateRepo(g.Repo); err != nil {
		return "", errors.New("The saved repository address is not valid: save the settings again.")
	}
	target := siteTarget(site, g.Target)
	if ValidateBranch(g.Branch) != nil || (target != TargetApp && ValidateTarget(target) != nil) {
		return "", errors.New("The saved branch or folder is not valid: save the settings again.")
	}

	root := m.root(site.Domain)
	dir := m.folder(site, target)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return "", fmt.Errorf("The folder %s does not exist.", dir)
	}
	if IsSSH(g.Repo) {
		if pub, _ := m.publicKey(site); pub == "" {
			return "", errors.New("The deploy key is missing. Create a new one in the panel and add it to the repository.")
		}
	}
	if !m.hasGit() {
		lg.Logf("Installing Git")
		if err := run.Apt(ctx, m.run, lg, "install", "-y", "--no-install-recommends", "git"); err != nil {
			return "", fmt.Errorf("Could not install Git: %v", err)
		}
	}

	lg.Logf("Deploying branch %s of %s into %s", g.Branch, g.Repo, dir)
	if !isWorkTree(dir) {
		// The folder must be empty: a deploy would mix the repository
		// with other files and later overwrite them.
		if err := checkEmpty(root, target); err != nil {
			return "", err
		}
		// "git init" and a fetch, rather than "git clone", work in a
		// folder that is not empty (the site folder has public/, logs/
		// and tmp/), and leave the site as it was if the fetch fails.
		lg.Logf("Creating a Git repository in %s", dir)
		if out, err := m.gitOutput(ctx, site, dir, "init", "-q", "-b", g.Branch); err != nil {
			return "", fmt.Errorf("git init failed: %s", strings.TrimSpace(run.Tail(out, 5)))
		}
		if out, err := m.gitOutput(ctx, site, dir, "remote", "add", "origin", g.Repo); err != nil {
			return "", fmt.Errorf("Could not set the repository address: %s", strings.TrimSpace(run.Tail(out, 5)))
		}
	} else {
		// The address may have changed since the last deploy.
		if _, err := m.gitOutput(ctx, site, dir, "remote", "set-url", "origin", g.Repo); err != nil {
			if out, err := m.gitOutput(ctx, site, dir, "remote", "add", "origin", g.Repo); err != nil {
				return "", fmt.Errorf("Could not set the repository address: %s", strings.TrimSpace(run.Tail(out, 5)))
			}
		}
	}

	lg.Logf("Fetching branch %s", g.Branch)
	// An explicit refspec updates origin/<branch> even in a repository
	// cloned with --single-branch for another branch.
	refspec := "+refs/heads/" + g.Branch + ":refs/remotes/origin/" + g.Branch
	fetchOut := &tailWriter{w: lg}
	if err := m.gitStream(ctx, fetchOut, fetchTimeout, site, dir, "fetch", "origin", refspec); err != nil {
		return "", fetchError(err.Error()+"\n"+string(fetchOut.b), g, root)
	}
	// Tracked files take the repository's version. Untracked files, such
	// as uploads, stay: never "git clean".
	if err := m.gitStream(ctx, lg, stepTimeout, site, dir, "reset", "--hard", "origin/"+g.Branch); err != nil {
		return "", fmt.Errorf("git reset failed: %v", err)
	}
	out, err := m.gitOutput(ctx, site, dir, "log", "-1", "--format=%H%n%s")
	if err != nil {
		return "", fmt.Errorf("Could not read the deployed commit: %s", strings.TrimSpace(run.Tail(out, 5)))
	}
	hash, subject := parseCommit(out)
	if hash == "" {
		return "", fmt.Errorf("Could not read the deployed commit: %s", strings.TrimSpace(run.Tail(out, 5)))
	}
	last = hash + " " + subject
	lg.Logf("Deployed commit %s: %s", hash[:12], subject)
	m.removePlaceholder(root, target, lg)

	if g.Script != "" {
		if err := m.phpShim(ctx, site); err != nil {
			lg.Logf("Warning: could not select PHP %s for the script: %v", site.PHPVersion, err)
		}
		lg.Logf("Running the deploy script in %s", dir)
		sctx, cancel := context.WithTimeout(ctx, ScriptTimeout)
		env := []string{"DEPLOY_COMMIT=" + hash, "DEPLOY_BRANCH=" + g.Branch}
		// -e: the script stops at the first command that fails.
		err := m.run.Stream(sctx, lg, "runuser", m.userArgs(site, dir, env, "sh", "-e", "-c", g.Script)...)
		timedOut := errors.Is(sctx.Err(), context.DeadlineExceeded)
		cancel()
		if err != nil {
			if timedOut {
				return hash, fmt.Errorf("The deploy script did not finish within %d minutes.", int(ScriptTimeout.Minutes()))
			}
			var re *run.Error
			if errors.As(err, &re) {
				err = re.Err
			}
			return hash, fmt.Errorf("The deploy script failed (%v).", err)
		}
		lg.Logf("The deploy script finished")
	}
	return hash, nil
}

// isWorkTree reports whether dir is the top of a Git work tree. .git may
// also be a file (a linked work tree).
func isWorkTree(dir string) bool {
	_, err := os.Lstat(filepath.Join(dir, ".git"))
	return err == nil
}

// isPlaceholder reports whether a file is the panel's welcome page.
func isPlaceholder(r *os.Root, name string) bool {
	f, err := r.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 4096))
	return err == nil && len(b) < 4096 && bytes.Contains(b, []byte(placeholderMark))
}

// checkEmpty refuses a first deploy into a folder with files in it. The
// panel's placeholder page, and logs/, tmp/ and .ssh/ (the deploy key) in
// the site folder, do not count; in an app's folder, neither does the
// virtual environment the panel makes for a Python app (.venv).
func checkEmpty(root, target string) error {
	r, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("The folder %s cannot be read: %v", root, err)
	}
	defer r.Close()
	if target == TargetApp {
		app := filepath.Join(root, "app")
		names, err := readDir(r, "app")
		if err != nil {
			return fmt.Errorf("The folder %s cannot be read: %v", app, err)
		}
		for _, name := range names {
			if name != ".venv" {
				return fmt.Errorf("The folder %s already has files in it (%s). Deploying would mix the repository with them: "+
					"move them away first.", app, name)
			}
		}
		return nil
	}
	public := filepath.Join(root, "public")
	names, err := readDir(r, "public")
	if err != nil && !(target == TargetRoot && errors.Is(err, fs.ErrNotExist)) {
		return fmt.Errorf("The folder %s cannot be read: %v", public, err)
	}
	for _, name := range names {
		if name == "index.html" && isPlaceholder(r, filepath.Join("public", "index.html")) {
			continue
		}
		return notEmpty(public, name)
	}
	if target == TargetRoot {
		names, err := readDir(r, ".")
		if err != nil {
			return fmt.Errorf("The folder %s cannot be read: %v", root, err)
		}
		for _, name := range names {
			switch name {
			case "public", "logs", "tmp", sshDir:
				continue
			}
			return notEmpty(root, name)
		}
	}
	return nil
}

// readDir lists a folder inside r, sorted by name.
func readDir(r *os.Root, name string) ([]string, error) {
	f, err := r.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	names, err := f.Readdirnames(-1)
	slices.Sort(names)
	return names, err
}

func notEmpty(dir, name string) error {
	return fmt.Errorf("The folder %s already has files in it (%s). Deploying would mix the repository with them: "+
		"move them away first, or deploy into the other folder.", dir, name)
}

// removePlaceholder deletes the panel's welcome page once the repository
// has put a website in public/. An app's repository goes into app/, and its
// program answers the website's requests.
func (m *Manager) removePlaceholder(root, target string, lg Logger) {
	if target == TargetApp {
		return
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return
	}
	defer r.Close()
	index := filepath.Join("public", "index.html")
	names, _ := readDir(r, "public")
	other := false
	for _, name := range names {
		if name != "index.html" && name != ".git" {
			other = true
		}
	}
	if !isPlaceholder(r, index) {
		return
	}
	if !other {
		if target == TargetRoot {
			lg.Logf("Note: the repository has no public/ folder, but the website shows %s.", filepath.Join(root, "public"))
		}
		return
	}
	if err := r.Remove(index); err == nil {
		lg.Logf("Removed the panel's placeholder page")
	}
}

var hashRe = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

// parseCommit reads the output of "git log -1 --format=%H%n%s", which may
// also hold warnings.
func parseCommit(out string) (hash, subject string) {
	lines := strings.Split(strings.ReplaceAll(out, "\r", ""), "\n")
	for i, l := range lines {
		if hashRe.MatchString(strings.TrimSpace(l)) {
			hash = strings.TrimSpace(l)
			if i+1 < len(lines) {
				subject = strings.TrimSpace(lines[i+1])
			}
			if len(subject) > 200 {
				subject = subject[:200]
			}
			return hash, subject
		}
	}
	return "", ""
}

// fetchError explains the usual reasons a fetch fails, from git's output s.
func fetchError(s string, g *store.GitDeploy, root string) error {
	hint := ""
	switch {
	case strings.Contains(s, "Permission denied (publickey)"), strings.Contains(s, "Repository not found") && IsSSH(g.Repo):
		hint = "The repository refused the deploy key: add the key shown in the panel to the repository (GitHub: Settings → Deploy keys)."
	case strings.Contains(s, "could not read Username"), strings.Contains(s, "terminal prompts disabled"), strings.Contains(s, "Authentication failed"):
		hint = "The repository is private or does not exist. For a private repository, use its SSH address and add the deploy key."
	case strings.Contains(s, "couldn't find remote ref"):
		hint = fmt.Sprintf("The repository has no branch %s.", g.Branch)
	case strings.Contains(s, "REMOTE HOST IDENTIFICATION HAS CHANGED"), strings.Contains(s, "Host key verification failed"):
		hint = fmt.Sprintf("The Git server's SSH host key is not the one seen before. If the server really changed it, delete %s.",
			filepath.Join(root, sshDir, knownHosts))
	case strings.Contains(s, "Could not resolve host"), strings.Contains(s, "Could not resolve hostname"):
		hint = "The repository's host name was not found."
	case strings.Contains(s, "signal: killed"), strings.Contains(s, "context deadline exceeded"):
		hint = fmt.Sprintf("It took longer than %d minutes.", int(fetchTimeout.Minutes()))
	}
	msg := fmt.Sprintf("Could not fetch branch %s from %s.", g.Branch, g.Repo)
	if hint != "" {
		msg += " " + hint
	}
	return errors.New(msg)
}
