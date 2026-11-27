package api

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/database"
	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/siteimport"
	"github.com/nefocp/nefo/internal/usererr"
)

// The database server card of the Software page: upgrading MariaDB to a
// newer series from MariaDB's repository (database/upgrade.go), switching
// between MariaDB and MySQL (database/switch.go), and the dumps both keep.

// dbFixer adapts the dumps of a switch for the other server, as the
// website import adapts dumps from other panels.
func dbFixer(r io.Reader, to database.Engine) io.Reader {
	return siteimport.FixDump(r, to == database.MySQL)
}

// dbNamesFixer adapts the collation names of SQL imports and backup
// restores from the other engine, and leaves every statement in.
func dbNamesFixer(r io.Reader, to database.Engine) io.Reader {
	return siteimport.FixCollations(r, to == database.MySQL)
}

// dbUpgradeOption is a series the installed MariaDB can be upgraded to.
type dbUpgradeOption struct {
	Series  string  `json:"series"`
	Until   string  `json:"until"`
	Blocked *reason `json:"blocked,omitempty"`
}

// handleDBEngine answers GET /api/db/engine: the installed server (with
// its series and where it comes from), the newer MariaDB series it can be
// upgraded to, the engine it can switch to, and the folders of dumps.
func (s *Server) handleDBEngine(w http.ResponseWriter, r *http.Request, a *authed) {
	ctx := r.Context()
	st := s.db.Status(ctx)
	res := struct {
		database.Status
		DistroSeries  string            `json:"distro_series"`
		Upgrades      []dbUpgradeOption `json:"upgrades"`
		SwitchTo      string            `json:"switch_to"`
		SwitchBlocked *reason           `json:"switch_blocked"`
		Dumps         []database.Dump   `json:"dumps"`
	}{Status: st, DistroSeries: s.db.DistroMariaDBSeries(), Upgrades: []dbUpgradeOption{}}
	if st.Installed && st.Engine == database.MariaDB {
		for _, sr := range database.MariaDBSeriesList() {
			if database.CompareSeries(sr.Series, st.Series) <= 0 {
				continue // older series are refused (see CheckUpgrade)
			}
			res.Upgrades = append(res.Upgrades, dbUpgradeOption{Series: sr.Series, Until: sr.Until, Blocked: reasonOf(s.db.SeriesAvailable(sr.Series))})
		}
	}
	if st.Installed {
		res.SwitchTo = string(database.MySQL)
		if st.Engine == database.MySQL {
			res.SwitchTo = string(database.MariaDB)
		} else {
			res.SwitchBlocked = reasonOf(s.db.MySQLAvailable())
		}
		if res.SwitchBlocked == nil && !st.Running {
			res.SwitchBlocked = &reason{Code: "db_not_running"}
		}
	}
	dumps, err := s.db.ListDumps()
	if err != nil {
		s.opError(w, r, err)
		return
	}
	res.Dumps = dumps
	writeJSON(w, http.StatusOK, res)
}

// handleDBUpgradePlan answers GET /api/db/upgrade?series=11.4: what an
// upgrade would do, and why it can't run now ("blocked", or an error when
// there is no plan at all).
func (s *Server) handleDBUpgradePlan(w http.ResponseWriter, r *http.Request, a *authed) {
	plan, err := s.db.CheckUpgrade(r.Context(), r.URL.Query().Get("series"))
	if plan == nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": plan, "blocked": reasonOf(err)})
}

// handleDBUpgrade upgrades MariaDB: {"series": "11.4"}, as a job.
func (s *Server) handleDBUpgrade(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Series string `json:"series"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	user, ip := a.user.Username, clientIP(r)
	if _, err := s.db.CheckUpgrade(r.Context(), req.Series); err != nil {
		s.opError(w, r, err)
		return
	}
	job, err := s.jobs.Start("upgrade_db", "MariaDB "+req.Series, 2*time.Hour, func(ctx context.Context, j *jobs.Job) error {
		err := s.db.UpgradeMariaDB(ctx, req.Series, j)
		if err == nil {
			s.audit(ctx, user, "db_upgraded", ip, "mariadb "+req.Series)
		}
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

// handleDBSwitchPlan answers GET /api/db/switch?to=mysql: what a switch
// would carry over, and its problems.
func (s *Server) handleDBSwitchPlan(w http.ResponseWriter, r *http.Request, a *authed) {
	plan, err := s.db.CheckSwitch(r.Context(), database.Engine(r.URL.Query().Get("to")))
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

// handleDBSwitch replaces the database server with the other one:
// {"to": "mysql", "confirm": "mysql"}, as a job. It is refused before the
// job starts while the plan has problems.
func (s *Server) handleDBSwitch(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		To      string `json:"to"`
		Confirm string `json:"confirm"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	to := database.Engine(req.To)
	if strings.TrimSpace(req.Confirm) != req.To {
		s.opError(w, r, usererr.New("confirm_mismatch"))
		return
	}
	plan, err := s.db.CheckSwitch(r.Context(), to)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	if len(plan.Problems) > 0 {
		s.opError(w, r, usererr.New(plan.Problems[0].Code, plan.Problems[0].Detail))
		return
	}
	user, ip := a.user.Username, clientIP(r)
	job, err := s.jobs.Start("switch_db", to.Name(), 3*time.Hour, func(ctx context.Context, j *jobs.Job) error {
		err := s.db.Switch(ctx, to, dbFixer, j)
		if err == nil {
			s.audit(ctx, user, "db_switched", ip, string(plan.From)+" -> "+string(to))
		}
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

// handleDeleteDBDump deletes a folder of dumps (DELETE
// /api/db/dumps/{name}).
func (s *Server) handleDeleteDBDump(w http.ResponseWriter, r *http.Request, a *authed) {
	name := r.PathValue("name")
	if err := s.db.DeleteDump(name); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "db_dump_deleted", clientIP(r), name)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
