// Command nefo is the server control panel: one binary that runs the web
// UI and API, and a few admin commands for the server's root user.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nefocp/nefo/internal/api"
	"github.com/nefocp/nefo/internal/auth"
	"github.com/nefocp/nefo/internal/config"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/tlsutil"
	"github.com/nefocp/nefo/internal/update"
	"github.com/nefocp/nefo/internal/version"
)

const usageText = `%[1]s - server control panel

Usage:
  nefo serve           Run the panel (normally started by systemd)
  nefo init            Create the config file and the first admin account
  nefo info            Show the panel URL
  nefo reset-password  Set a new random password for an account
  nefo update          Install the latest release and restart the panel
  nefo setup           Install a web server, PHP, a database and extras
  nefo version         Show the version

Run "nefo <command> -h" for the options of a command.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, usageText, version.AppName)
		os.Exit(2)
	}
	var err error
	args := os.Args[2:]
	switch os.Args[1] {
	case "serve":
		err = cmdServe(args)
	case "init":
		err = cmdInit(args)
	case "info":
		err = cmdInfo(args)
	case "reset-password":
		err = cmdResetPassword(args)
	case "update":
		err = cmdUpdate(args)
	case "setup":
		err = cmdSetup(args)
	case "version", "--version", "-v":
		fmt.Printf("%s %s (commit %s)\n", version.AppName, version.Version, version.Commit)
	case "help", "--help", "-h":
		fmt.Printf(usageText, version.AppName)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		fmt.Fprintf(os.Stderr, usageText, version.AppName)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func configFlag(fs *flag.FlagSet) *string {
	def := config.DefaultPath
	if v := os.Getenv("NEFO_CONFIG"); v != "" {
		def = v
	}
	return fs.String("config", def, "config file (or set NEFO_CONFIG)")
}

func panelURL(cfg *config.Config, host string) string {
	scheme := "http"
	if cfg.TLS.Enabled {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/%s/", scheme, net.JoinHostPort(host, fmt.Sprint(cfg.Port())), cfg.Entrance)
}

func defaultHost(cfg *config.Config, host string) string {
	if host != "" {
		return host
	}
	if cfg.Dev {
		return "127.0.0.1"
	}
	if ip := firstIP(); ip != "" {
		return ip
	}
	return "SERVER_IP"
}

// firstIP returns the server's first IPv4 address that is not a loopback
// or link-local one (what the installer shows, from hostname -I), or an
// IPv6 one if it has no IPv4 address.
func firstIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	v6 := ""
	for _, a := range addrs {
		n, ok := a.(*net.IPNet)
		if !ok || !n.IP.IsGlobalUnicast() {
			continue
		}
		if n.IP.To4() != nil {
			return n.IP.String()
		}
		if v6 == "" {
			v6 = n.IP.String()
		}
	}
	return v6
}

// ---- init ----

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	cfgPath := configFlag(fs)
	dataDir := fs.String("data", "/var/lib/nefo", "data directory")
	listen := fs.String("listen", "", `listen address (default: a random port, or 127.0.0.1:8080 with -dev)`)
	dev := fs.Bool("dev", false, "development mode: plain HTTP on 127.0.0.1:8080 with entrance \"dev\"")
	host := fs.String("host", "", "host name or IP to show in the printed URL")
	fs.Parse(args)

	if _, err := os.Stat(*cfgPath); err == nil {
		return fmt.Errorf("%s already exists; refusing to overwrite it (use \"nefo info\")", *cfgPath)
	}
	absData, err := filepath.Abs(*dataDir)
	if err != nil {
		return err
	}
	cfg := &config.Config{DataDir: absData, Dev: *dev}
	if *dev {
		cfg.Listen, cfg.Entrance = "127.0.0.1:8080", "dev"
	} else {
		cfg.Listen = fmt.Sprintf(":%d", config.RandomPort())
		cfg.Entrance = config.RandomEntrance()
		cfg.TLS.Enabled = true
	}
	if *listen != "" {
		cfg.Listen = *listen
	}
	if err := config.Save(*cfgPath, cfg); err != nil {
		return err
	}

	password, err := createFirstAdmin(cfg)
	if err != nil {
		os.Remove(*cfgPath) // let the user simply run init again
		return err
	}

	fmt.Printf("\n%s is ready.\n\n", version.AppName)
	fmt.Printf("  URL:      %s\n", panelURL(cfg, defaultHost(cfg, *host)))
	if password != "" {
		fmt.Printf("  Username: admin\n  Password: %s\n\n", password)
		fmt.Println("Save this password now: it is stored only as a hash and cannot be shown again.")
		fmt.Println("After signing in, change it and turn on two-factor authentication.")
	} else {
		fmt.Println("\nThe database already has accounts; their passwords were not changed.")
		fmt.Println(`Use "nefo reset-password" if you need a new one.`)
	}
	if cfg.TLS.Enabled {
		fmt.Println("The certificate is self-signed, so your browser will warn you the first time.")
	}
	fmt.Println()
	return nil
}

// createFirstAdmin creates "admin" with a random password when the database
// has no accounts yet, and returns the password ("" if nothing was created).
func createFirstAdmin(cfg *config.Config) (string, error) {
	st, err := store.Open(cfg.DataDir)
	if err != nil {
		return "", err
	}
	defer st.Close()
	ctx := context.Background()
	n, err := st.CountUsers(ctx)
	if err != nil || n > 0 {
		return "", err
	}
	password := auth.NewPassword()
	hash, err := auth.HashPassword(password)
	if err != nil {
		return "", err
	}
	if _, err := st.CreateUser(ctx, "admin", hash); err != nil {
		return "", err
	}
	_ = st.Audit(ctx, "admin", "user_created", "cli", "first admin account")
	return password, nil
}

// ---- info ----

func cmdInfo(args []string) error {
	fs := flag.NewFlagSet("info", flag.ExitOnError)
	cfgPath := configFlag(fs)
	host := fs.String("host", "", "host name or IP to show in the URL")
	portOnly := fs.Bool("port", false, "print only the port number (for scripts)")
	fs.Parse(args)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	if *portOnly {
		fmt.Println(cfg.Port())
		return nil
	}
	if *host == "" {
		// The panel's own domain, once it has one (Settings → Panel domain).
		if domain := panelDomain(cfg); domain != "" {
			fmt.Printf("URL:    %s\n", panelURL(cfg, domain))
		}
	}
	fmt.Printf("URL:    %s\nConfig: %s\nData:   %s\n", panelURL(cfg, defaultHost(cfg, *host)), *cfgPath, cfg.DataDir)
	return nil
}

// panelDomain returns the panel's domain from its settings, or "".
func panelDomain(cfg *config.Config) string {
	if _, err := os.Stat(filepath.Join(cfg.DataDir, "nefo.db")); err != nil {
		return ""
	}
	st, err := store.Open(cfg.DataDir)
	if err != nil {
		return ""
	}
	defer st.Close()
	domain, _ := st.Setting(context.Background(), "panel_domain")
	return domain
}

// ---- reset-password ----

func cmdResetPassword(args []string) error {
	fs := flag.NewFlagSet("reset-password", flag.ExitOnError)
	cfgPath := configFlag(fs)
	user := fs.String("user", "admin", "account name")
	disable2FA := fs.Bool("disable-2fa", false, "also turn off two-factor authentication (lost phone)")
	fs.Parse(args)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	st, err := store.Open(cfg.DataDir)
	if err != nil {
		return err
	}
	defer st.Close()
	ctx := context.Background()

	u, err := st.UserByName(ctx, *user)
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("no account named %q", *user)
	}
	if err != nil {
		return err
	}
	password := auth.NewPassword()
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	if err := st.SetPassword(ctx, u.ID, hash); err != nil {
		return err
	}
	detail := "all sessions signed out"
	if *disable2FA {
		if err := st.SetTOTP(ctx, u.ID, "", false); err != nil {
			return err
		}
		detail += "; 2FA turned off"
	}
	_ = st.Audit(ctx, u.Username, "password_reset", "cli", detail)

	fmt.Printf("New password for %s: %s\n", u.Username, password)
	if *disable2FA {
		fmt.Println("Two-factor authentication is off. Turn it on again after signing in.")
	}
	return nil
}

// ---- serve ----

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	cfgPath := configFlag(fs)
	fs.Parse(args)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	st, err := store.Open(cfg.DataDir)
	if err != nil {
		return err
	}
	defer st.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := api.New(cfg, st, logger)
	// "nefo setup" asks the panel through this socket (internal/api/local.go).
	// It exists before the panel listens on its port.
	if ln, err := srv.ListenLocal(); err != nil {
		logger.Warn("local socket", "err", err)
	} else {
		go srv.ServeLocal(ctx, ln)
	}
	go srv.RefreshConfig(ctx)
	go srv.RunMaintenance(ctx)
	go srv.RunScheduler(ctx)
	go srv.RunMonitor(ctx)
	go srv.RunUptime(ctx)
	go srv.RunGitHooks(ctx) // push webhooks, through Nginx, on 127.0.0.1:7809
	go srv.RunHtaccessWatch(ctx)

	hs := &http.Server{
		Addr:              cfg.Listen,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
		// TLS handshake errors from internet scanners are noise: log them at debug level.
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelDebug),
	}

	errc := make(chan error, 1)
	if cfg.TLS.Enabled {
		// Certificates in order of preference: one set in config.json, the
		// panel's Let's Encrypt certificate, and the self-signed fallback.
		// New or renewed files are picked up without a restart.
		selfCert := filepath.Join(cfg.DataDir, "tls", "cert.pem")
		selfKey := filepath.Join(cfg.DataDir, "tls", "key.pem")
		created, err := tlsutil.EnsureSelfSigned(selfCert, selfKey)
		if err != nil {
			return fmt.Errorf("self-signed certificate: %w", err)
		}
		if created {
			logger.Info("created a self-signed certificate", "cert", selfCert)
		}
		var pairs []tlsutil.Pair
		if cfg.TLS.Cert != "" && cfg.TLS.Key != "" {
			pairs = append(pairs, tlsutil.Pair{Cert: cfg.TLS.Cert, Key: cfg.TLS.Key})
		}
		le := filepath.Join(cfg.DataDir, "certs", "_panel")
		pairs = append(pairs, tlsutil.Pair{Cert: filepath.Join(le, "fullchain.pem"), Key: filepath.Join(le, "privkey.pem")},
			tlsutil.Pair{Cert: selfCert, Key: selfKey})
		reloader, err := tlsutil.NewReloader(pairs...)
		if err != nil {
			return fmt.Errorf("load certificate: %w", err)
		}
		hs.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, GetCertificate: reloader.GetCertificate}
		go func() { errc <- hs.ListenAndServeTLS("", "") }()
	} else {
		if !cfg.Dev && !isLoopback(cfg.Listen) {
			logger.Warn("TLS is off and the panel listens on a public address: passwords travel in plain text")
		}
		go func() { errc <- hs.ListenAndServe() }()
	}
	logger.Info("panel started", "version", version.Version, "listen", cfg.Listen, "tls", cfg.TLS.Enabled)

	select {
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := hs.Shutdown(shutdownCtx)
		// Give a running job (such as apt) time to finish rather than cutting
		// it off half way. systemd waits 90 seconds before it kills the panel.
		jobsCtx, cancelJobs := context.WithTimeout(context.Background(), 75*time.Second)
		defer cancelJobs()
		srv.WaitJobs(jobsCtx)
		return err
	}
}

func isLoopback(listen string) bool {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// ---- update ----

type stdoutLogger struct{}

func (stdoutLogger) Logf(format string, args ...any) { fmt.Println(fmt.Sprintf(format, args...)) }

func cmdUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	cfgPath := configFlag(fs)
	check := fs.Bool("check", false, "only show whether an update is available")
	fs.Parse(args)
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	u, err := update.New(cfg.UpdateURL, bin, run.System{})
	if err != nil {
		return err
	}
	ctx := context.Background()
	latest, err := u.Latest(ctx)
	if err != nil {
		return err
	}
	fmt.Println("Installed:", version.Version)
	fmt.Println("Latest:   ", latest)
	if *check || latest == version.Version || latest == "v"+version.Version {
		return nil
	}
	_, err = u.Apply(ctx, stdoutLogger{})
	return err
}
