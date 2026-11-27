package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"

	"github.com/nefocp/nefo/internal/config"
	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/store"
)

// cmdUnlockFiles gives every website whose files are locked (Websites →
// File lock) its files back, with the owner, group and mode each had before
// the lock. It is a step of removing the panel: without it, the sites'
// files would stay root's. Stop the panel first.
func cmdUnlockFiles(args []string) error {
	fs := flag.NewFlagSet("unlock-files", flag.ExitOnError)
	cfgPath := configFlag(fs)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: nefo unlock-files [-config FILE]\n\n"+
			"Gives every website whose files are locked (Websites → File lock) its files\n"+
			"back, exactly as they were before the lock. Run it before removing the panel.\n\n")
		fs.PrintDefaults()
	}
	fs.Parse(args)
	if runtime.GOOS == "linux" && os.Geteuid() != 0 {
		return errors.New("run nefo unlock-files as root")
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	return unlockFiles(context.Background(), cfg.DataDir, run.System{}, os.Stdout)
}

// unlockFiles runs UnlockAllFiles on the panel data in dataDir.
func unlockFiles(ctx context.Context, dataDir string, r run.Runner, out io.Writer) error {
	st, err := store.Open(dataDir)
	if err != nil {
		return err
	}
	defer st.Close()
	m := hosting.New(hosting.DefaultPaths(dataDir), r, st, slog.New(slog.NewTextHandler(io.Discard, nil)), "")
	return unlockAll(ctx, m, out)
}

func unlockAll(ctx context.Context, m *hosting.Manager, out io.Writer) error {
	n, err := m.UnlockAllFiles(ctx, func(domain string, err error) {
		if err != nil {
			fmt.Fprintf(out, "%s: %v\n", domain, err)
		} else {
			fmt.Fprintf(out, "Unlocked the files of %s\n", domain)
		}
	})
	if err == nil && n == 0 {
		fmt.Fprintln(out, "No website's files are locked.")
	}
	return err
}
