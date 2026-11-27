package api

import (
	"context"
	"net/http"
	"time"

	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/software"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/wptoolkit"
)

// The Software page's Tools group: Composer and WP-CLI. Both are installed
// by the panel when something needs them (Laravel and Drupal, the
// WordPress toolkit); the cards show them and update them to the latest
// official release, checked against its published checksum.

// toolItems are the Tools group's cards.
func (s *Server) toolItems(ctx context.Context) []software.Item {
	wp := software.Item{ID: "wpcli", Kind: "wpcli", Group: "tools", Source: "official"}
	if s.wpkit().Installed() {
		wp.Installed, wp.Updatable, wp.Version = true, true, s.wpkit().CLIVersion()
		n, err := s.wordpressSites(ctx)
		wp.Usage = map[string]int{"sites": n}
		if err == nil {
			err = wptoolkit.CLIBlocker(n)
		}
		wp.Block(err)
	}
	return []software.Item{s.software.ComposerItem(ctx), wp}
}

// wordpressSites counts the WordPress websites, which the toolkit manages
// with WP-CLI.
func (s *Server) wordpressSites(ctx context.Context) (int, error) {
	sites, err := s.store.ListSites(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, site := range sites {
		if site.App == hosting.AppWordPress && site.PHPVersion != "" {
			n++
		}
	}
	return n, nil
}

// wpcliRemovable returns why WP-CLI can't be removed now, or nil.
func (s *Server) wpcliRemovable(ctx context.Context) error {
	if !s.wpkit().Installed() {
		return usererr.New("software_not_installed", "WP-CLI")
	}
	n, err := s.wordpressSites(ctx)
	if err != nil {
		return err
	}
	return wptoolkit.CLIBlocker(n)
}

// handleSoftwareUpdate updates a tool to its latest official release:
// POST /api/software/{composer|wpcli}/update.
func (s *Server) handleSoftwareUpdate(w http.ResponseWriter, r *http.Request, a *authed) {
	id, user, ip := r.PathValue("id"), a.user.Username, clientIP(r)
	var do func(ctx context.Context, j *jobs.Job) error
	switch id {
	case "composer":
		do = func(ctx context.Context, j *jobs.Job) error { return s.software.UpdateComposer(ctx, j) }
	case "wpcli":
		do = func(ctx context.Context, j *jobs.Job) error { return s.wpkit().UpdateCLI(ctx, j) }
	case "ioncube": // the panel's pinned release (software/ioncube.go)
		do = func(ctx context.Context, j *jobs.Job) error { return s.software.InstallIoncube(ctx, j) }
	default:
		s.opError(w, r, usererr.New("unknown_software", id))
		return
	}
	if !s.hosting.Supported() {
		s.opError(w, r, hosting.ErrUnsupported)
		return
	}
	name, _ := softwareName(id)
	job, err := s.jobs.Start("update_software", name, 15*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		err := do(ctx, j)
		if err == nil {
			s.audit(ctx, user, "software_updated", ip, id)
		}
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}
