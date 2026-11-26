package api

import (
	"net/http"

	"github.com/nefocp/nefo/internal/database"
	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/software"
)

// features says which optional software is installed, so the menu can hide
// the pages of software that is not. Websites, Databases and Software
// always show: they offer the installs.
type features struct {
	Docker         bool `json:"docker"`
	Redis          bool `json:"redis"`
	MariaDB        bool `json:"mariadb"`
	Node           bool `json:"node"`
	Python         bool `json:"python"`
	WordPressSites bool `json:"wordpress_sites"`
	// WebServer is "nginx", "apache", "openlitespeed" or "" for none.
	WebServer string `json:"web_server"`
}

// handleFeatures answers GET /api/features. The UI asks on every page
// change, so it only looks for files and reads the panel's own database:
// no programs run.
func (s *Server) handleFeatures(w http.ResponseWriter, r *http.Request, a *authed) {
	sites, err := s.store.ListSites(r.Context())
	if err != nil {
		s.opError(w, r, err)
		return
	}
	f := features{
		Docker:    software.DockerInstalled(),
		Redis:     software.RedisInstalled(),
		MariaDB:   database.Installed(),
		Node:      software.NodeInstalled(),
		Python:    software.PythonReady(),
		WebServer: s.installedWebServer(),
	}
	for _, site := range sites {
		if site.App == hosting.AppWordPress {
			f.WordPressSites = true
			break
		}
	}
	writeJSON(w, http.StatusOK, f)
}

// installedWebServer is the "server" of GET /api/web/status: the web server
// the panel manages, "" for none.
func (s *Server) installedWebServer() string { return s.hosting.WebServer() }
