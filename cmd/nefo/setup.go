package main

// "nefo setup": the setup page of the panel on the command line. It asks
// the running panel to do the installs, through the panel's local socket
// (see internal/api/local.go), so they run as one of the panel's jobs: one
// at a time with its other jobs, and with the panel's view of the server
// kept up to date. When no panel is running, it runs the same code here;
// apt and dpkg then only wait for each other's locks (run.Apt).

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/api"
	"github.com/nefocp/nefo/internal/config"
	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/setup"
	"github.com/nefocp/nefo/internal/software"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

const setupUsage = `Usage: nefo setup [options]

Sets the server up like the panel's setup page: installs a web server, PHP
versions, MariaDB and extras, one after the other. Parts that are installed
already are skipped, so it can run again after a failure. Without --web,
--php or any extra, it offers the recommended choice for this server's RAM.

  nefo setup --web nginx --php 8.3,8.4 --db mariadb --redis --yes

Options:
`

// setupExtraFlags are the flags that name an extra, and its Software ID.
var setupExtraFlags = []struct{ flag, id, usage string }{
	{"postgresql", "postgresql", "install PostgreSQL (when the Software page offers it)"},
	{"redis", "redis", "install Redis"},
	{"memcached", "memcached", "install Memcached (when the Software page offers it)"},
	{"node", "nodejs", "install Node.js (the newest LTS release)"},
	{"python", "python", "install Python for app websites (venv and pip)"},
	{"docker", "docker", "install Docker"},
	{"adminer", "adminer", "install Adminer (needs a web server)"},
	{"waf", "waf", "install the web firewall, ModSecurity (needs a web server)"},
	{"clamav", "clamav", "install ClamAV (servers with 2 GB of RAM or more)"},
}

// setupOpts are the options of "nefo setup".
type setupOpts struct {
	cfgPath string
	choice  setup.Choice
	chose   bool // a part was named; otherwise the recommended choice
	yes     bool
	skip    bool
	dryRun  bool
	check   bool
	wait    time.Duration
}

// usageError is a wrong option: the command exits with status 2.
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func parseSetupFlags(args []string, out io.Writer) (*setupOpts, error) {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(out)
	o := &setupOpts{}
	cfgPath := configFlag(fs)
	web := fs.String("web", "", "web server: nginx, apache, openlitespeed or none")
	php := fs.String("php", "", `PHP versions, such as "8.3,8.4", or "none" (the web server always brings the distribution's)`)
	db := fs.String("db", "", "database server: mariadb, mysql or none; mariadb-11.4, mariadb-11.8 or mariadb-12.3 for an LTS series from MariaDB's repository")
	mariadb := fs.Bool("mariadb", false, "the same as --db mariadb")
	extras := map[string]*bool{}
	for _, x := range setupExtraFlags {
		extras[x.id] = fs.Bool(x.flag, false, x.usage)
	}
	fs.BoolVar(&o.yes, "yes", false, "install without asking")
	fs.BoolVar(&o.yes, "y", false, "short for --yes")
	fs.BoolVar(&o.skip, "skip", false, "install nothing: only mark the setup page as skipped")
	fs.BoolVar(&o.dryRun, "dry-run", false, "show what would be installed, and stop")
	fs.BoolVar(&o.check, "check", false, "only check the options, without the configuration (for the installer)")
	fs.DurationVar(&o.wait, "wait", 0, "how long to wait for the panel to start, such as 60s")
	fs.Usage = func() {
		fmt.Fprint(out, setupUsage)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		return nil, usageError{fmt.Sprintf("unexpected argument %q: options start with --", fs.Arg(0))}
	}
	o.cfgPath = *cfgPath
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })

	switch w := strings.ToLower(strings.TrimSpace(*web)); w {
	case "", "none":
	case setup.Nginx, setup.Apache, setup.OpenLiteSpeed:
		o.choice.Web = w
	default:
		return nil, usageError{fmt.Sprintf("unknown web server %q: use nginx, apache, openlitespeed or none", *web)}
	}
	o.choice.PHP = []string{}
	if p := strings.TrimSpace(*php); p != "" && p != "none" {
		for _, v := range strings.Split(p, ",") {
			v = strings.TrimSpace(v)
			// A known version, or a newer one the PHP repository may have
			// by now: the plan checks it against what this server offers.
			if !software.PHPVersionKnownOrNewer(v) {
				return nil, usageError{fmt.Sprintf("unknown PHP version %q: use one such as %s", v, strings.Join(software.PHPVersions[len(software.PHPVersions)-3:], ", "))}
			}
			if !slices.Contains(o.choice.PHP, v) {
				o.choice.PHP = append(o.choice.PHP, v)
			}
		}
	}
	switch d := strings.ToLower(strings.TrimSpace(*db)); {
	case *mariadb && d != "" && d != "mariadb":
		return nil, usageError{"--mariadb and --db " + *db + " disagree: choose one database server"}
	case *mariadb:
		o.choice.DB = "mariadb"
	case d == "" || d == "none":
	case setup.IsDatabase(d):
		o.choice.DB = d
	default:
		return nil, usageError{fmt.Sprintf("unknown database server %q: use mariadb, mysql or none", *db)}
	}
	o.choice.Extras = []string{}
	for _, id := range setup.Extras {
		if *extras[id] {
			o.choice.Extras = append(o.choice.Extras, id)
		}
	}
	o.chose = set["web"] || set["php"] || set["db"] || set["mariadb"] || len(o.choice.Extras) > 0
	if o.skip && (o.chose || o.dryRun) {
		return nil, usageError{"--skip installs nothing: leave out the other options"}
	}
	if o.chose && o.choice.Empty() {
		return nil, usageError{"nothing to install: name a web server, PHP versions, a database or extras, or use --skip"}
	}
	return o, nil
}

func cmdSetup(args []string) error {
	o, err := parseSetupFlags(args, os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		var ue usageError
		if errors.As(err, &ue) {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		os.Exit(2)
	}
	if o.check {
		return nil
	}
	if runtime.GOOS == "linux" && os.Geteuid() != 0 {
		return errors.New("run nefo setup as root")
	}
	cfg, err := config.Load(o.cfgPath)
	if err != nil {
		return err
	}
	ctx := context.Background()
	b, err := openSetup(ctx, cfg, o.wait)
	if err != nil {
		return err
	}
	defer b.close()

	if o.skip {
		if err := b.skip(ctx); err != nil {
			return setupError(err)
		}
		fmt.Println("The setup page is marked as skipped: the panel no longer opens it after signing in.")
		fmt.Println(`You can still run "nefo setup", or open it from the Software page.`)
		return nil
	}
	v, err := b.view(ctx)
	if err != nil {
		return setupError(err)
	}
	if !v.Supported {
		return errors.New("this server can't host websites: Nefo needs Debian or Ubuntu with systemd")
	}
	choice := o.choice
	if !o.chose {
		choice = v.Defaults
		fmt.Println("No parts named: this is the recommended choice for this server.")
	}
	plan, err := v.Plan(choice)
	if err != nil {
		return setupError(err)
	}
	printSetupPlan(os.Stdout, v, plan)
	if o.dryRun {
		return nil
	}
	if !o.yes {
		if fi, err := os.Stdin.Stat(); err != nil || fi.Mode()&os.ModeCharDevice == 0 {
			return errors.New("add --yes to install without asking")
		}
		fmt.Print("Install? [y/N] ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(line)); a != "y" && a != "yes" {
			fmt.Println("Nothing was installed.")
			return nil
		}
	}
	if err := b.run(ctx, choice); err != nil {
		return setupError(err)
	}
	fmt.Println("The server is set up. Add your first website in the panel: Websites → Add website.")
	return nil
}

// setupBackend is the running panel, or this process when none runs.
type setupBackend interface {
	view(ctx context.Context) (setup.View, error)
	skip(ctx context.Context) error
	run(ctx context.Context, c setup.Choice) error
	close()
}

// openSetup finds the running panel through its local socket, waiting up
// to wait for it to start. Without a panel, the setup runs here.
func openSetup(ctx context.Context, cfg *config.Config, wait time.Duration) (setupBackend, error) {
	sock := api.LocalSocket(cfg.DataDir)
	client := api.NewLocalClient(sock)
	deadline := time.Now().Add(wait)
	for {
		_, err := client.SetupView(ctx)
		if err == nil {
			return panelSetup{client}, nil
		}
		if !errors.Is(err, api.ErrNoPanel) {
			return nil, setupError(err)
		}
		if !time.Now().Before(deadline) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	// A panel of a version without the socket would not see what this
	// process installs: refuse rather than install behind its back.
	if panelListening(cfg) {
		return nil, fmt.Errorf("something answers on the panel's port (%s), but no panel answers on %s. If the panel is running, restart it (systemctl restart nefo), then try again", cfg.Listen, sock)
	}
	st, err := store.Open(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	fmt.Println("The panel is not running: the setup runs here.")
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	return &localSetup{srv: api.New(cfg, st, log), st: st}, nil
}

// panelListening reports whether something answers on the panel's port.
func panelListening(cfg *config.Config) bool {
	host, port, err := net.SplitHostPort(cfg.Listen)
	if err != nil {
		return false
	}
	if ip := net.ParseIP(host); host == "" || (ip != nil && ip.IsUnspecified()) {
		host = "127.0.0.1"
	}
	c, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), time.Second)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// panelSetup asks the running panel.
type panelSetup struct{ c *api.LocalClient }

func (p panelSetup) view(ctx context.Context) (setup.View, error) { return p.c.SetupView(ctx) }
func (p panelSetup) skip(ctx context.Context) error               { return p.c.SkipSetup(ctx) }
func (p panelSetup) close()                                       {}

func (p panelSetup) run(ctx context.Context, c setup.Choice) error {
	id, err := p.c.StartSetup(ctx, c)
	if err != nil {
		return err
	}
	fmt.Println("The setup runs as a task of the panel: you can also follow it there.")
	next, misses := 0, 0
	for {
		l, err := p.c.JobLog(ctx, id, next)
		if err != nil {
			// The panel may restart (an update): keep asking for a while.
			if misses++; misses > 60 {
				return fmt.Errorf("lost contact with the panel: %w. The setup may still be running: see the panel", err)
			}
			time.Sleep(2 * time.Second)
			continue
		}
		misses = 0
		for _, line := range l.Lines {
			fmt.Println(line)
		}
		next = l.Next
		switch l.Status {
		case "succeeded":
			return nil
		case "failed":
			return errors.New(l.Error)
		}
		time.Sleep(700 * time.Millisecond)
	}
}

// localSetup runs the setup in this process.
type localSetup struct {
	srv *api.Server
	st  *store.Store
}

func (l *localSetup) view(ctx context.Context) (setup.View, error) { return l.srv.SetupView(ctx) }
func (l *localSetup) skip(ctx context.Context) error               { return l.srv.SkipSetup(ctx, "root", "cli") }
func (l *localSetup) close()                                       { l.st.Close() }

func (l *localSetup) run(ctx context.Context, c setup.Choice) error {
	return l.srv.RunSetup(ctx, c, stdoutLog{os.Stdout})
}

// stdoutLog prints the setup's log: program output as it comes, and the
// setup's own lines.
type stdoutLog struct{ w io.Writer }

func (s stdoutLog) Write(p []byte) (int, error) { return s.w.Write(p) }
func (s stdoutLog) Logf(format string, args ...any) {
	fmt.Fprintf(s.w, format+"\n", args...)
}

// printSetupPlan shows the server and what will be installed.
func printSetupPlan(w io.Writer, v setup.View, p setup.Plan) {
	sv := v.Server
	cpus := "1 CPU"
	if sv.CPUs != 1 {
		cpus = fmt.Sprintf("%d CPUs", sv.CPUs)
	}
	if sv.MemTotalMB > 0 {
		fmt.Fprintf(w, "This server: %s, %s of RAM (%s in use), %s of free disk.\n", cpus, sizeMB(sv.MemTotalMB), sizeMB(sv.MemUsedMB), sizeMB(sv.DiskFreeMB))
	}
	fmt.Fprintln(w, "\nWhat the setup installs, one after the other:")
	width := 0
	for _, s := range p.Steps {
		width = max(width, len(s.Name))
	}
	for i, s := range p.Steps {
		var what string
		switch {
		case s.Installed:
			what = "installed already: skipped"
		default:
			ram := "no RAM while idle"
			if s.RAMMB > 0 {
				ram = "about " + sizeMB(s.RAMMB) + " of RAM"
				if s.RAMMax {
					ram = "up to " + sizeMB(s.RAMMB) + " of RAM"
				}
			}
			what = ram + ", " + sizeMB(s.DiskMB) + " of disk"
			if s.ThirdParty && v.PHPRepo != "" {
				what += ", from " + v.PHPRepo
			}
		}
		fmt.Fprintf(w, "  %d. %-*s  %s\n", i+1, width, s.Name, what)
	}
	fmt.Fprintf(w, "In all: about %s more RAM and %s of disk.\n", sizeMB(p.RAMMB), sizeMB(p.DiskMB))
	if p.AptListsMB > 0 {
		fmt.Fprintf(w, "The disk includes about %s for apt's package lists, which this server hasn't downloaded yet.\n", sizeMB(p.AptListsMB))
	}
	for _, warn := range p.Warnings {
		switch warn {
		case "ram":
			b, _ := v.Server.RAMBudgetMB()
			fmt.Fprintf(w, "\nWarning: that is more memory than this server has to spare (about %s). It installs anyway, but websites\n"+
				"may get slow, or MariaDB may be stopped, when the memory runs out. Choose less, or add swap (System → Swap).\n", sizeMB(max(b, 0)))
		case "disk":
			fmt.Fprintf(w, "\nWarning: only %s of disk is free, and 1 GB should stay free. It installs anyway, but may fail.\n", sizeMB(v.Server.DiskFreeMB))
		}
	}
	fmt.Fprintln(w)
}

// sizeMB formats megabytes: "960 MB", "1.9 GB".
func sizeMB(mb int) string {
	if mb >= 1024 {
		return fmt.Sprintf("%.1f GB", float64(mb)/1024)
	}
	return fmt.Sprintf("%d MB", mb)
}

// setupError turns the panel's error codes into sentences.
func setupError(err error) error {
	code, detail := "", ""
	var le *api.LocalError
	var ue *usererr.Error
	switch {
	case errors.As(err, &le):
		code, detail = le.Code, le.Detail
	case errors.As(err, &ue):
		code, detail = ue.Code, ue.Detail
	case errors.Is(err, hosting.ErrUnsupported):
		code = "unsupported"
	default:
		return err
	}
	msg := map[string]string{
		"busy":                "another task is running in the panel. Wait until it has finished, then try again",
		"unsupported":         "this server can't host websites: Nefo needs Debian or Ubuntu with systemd",
		"setup_empty":         "nothing to install: name a web server, PHP versions, a database or extras, or use --skip",
		"setup_db_conflict":   detail + " is installed on this server, and a server runs one database server: use --db " + strings.ReplaceAll(strings.ToLower(detail), " ", "-") + ", or none",
		"setup_needs_web":     detail + " needs a web server: add --web nginx (or apache, openlitespeed)",
		"setup_not_offered":   detail + " can't be installed on this server",
		"unknown_software":    "unknown software: " + detail,
		"invalid_web_server":  "unknown web server " + detail + ": use nginx, apache, openlitespeed or none",
		"web_server_conflict": detail + " is installed on this server, and a server runs one web server. Use --web " + detail + ", or remove it first (after deleting its websites)",
		"stack_not_ready":     "install the web server first",
	}[code]
	if msg == "" {
		return err
	}
	return errors.New(msg)
}
