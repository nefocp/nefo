package daemons

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/nefocp/nefo/internal/store"
)

// migrationSQL is the daemons table as given to the store's migrations.
const migrationSQL = `CREATE TABLE IF NOT EXISTS daemons (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	name       TEXT    NOT NULL,
	site_id    INTEGER NOT NULL,
	command    TEXT    NOT NULL,
	dir        TEXT    NOT NULL DEFAULT '',
	env        TEXT    NOT NULL DEFAULT '',
	enabled    INTEGER NOT NULL DEFAULT 1,
	created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS daemons_site ON daemons(site_id);`

// TestStore runs the store's methods on a real database. The table is
// created here when the store's migrations do not have it (yet).
func TestStore(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	raw, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(dir, "nefo.db"))+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(migrationSQL); err != nil {
		t.Fatal(err)
	}
	raw.Close()

	var _ Store = st // the real store has every method the manager needs

	a := &store.Daemon{Name: "Queue", SiteID: 7, Command: "php artisan queue:work", Dir: "app", Env: []string{"A=1", "B=two words"}, Enabled: true}
	if err := st.SaveDaemon(ctx, a); err != nil || a.ID == 0 {
		t.Fatalf("insert: %v (id %d)", err, a.ID)
	}
	b := &store.Daemon{Name: "Other", SiteID: 8, Command: "sleep 300"}
	if err := st.SaveDaemon(ctx, b); err != nil {
		t.Fatal(err)
	}
	got, err := st.DaemonByID(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Queue" || got.SiteID != 7 || got.Dir != "app" || !got.Enabled || len(got.Env) != 2 || got.Env[1] != "B=two words" || got.CreatedAt.IsZero() {
		t.Fatalf("read back %+v", got)
	}
	if got, _ := st.DaemonByID(ctx, b.ID); got.Enabled || len(got.Env) != 0 || got.Env == nil {
		t.Fatalf("empty env and disabled: %+v", got)
	}

	got.Command, got.Enabled, got.Env = "php artisan queue:work --tries=3", false, nil
	if err := st.SaveDaemon(ctx, got); err != nil {
		t.Fatal(err)
	}
	if again, _ := st.DaemonByID(ctx, a.ID); again.Command != "php artisan queue:work --tries=3" || again.Enabled || len(again.Env) != 0 {
		t.Fatalf("after update %+v", again)
	}
	if err := st.SaveDaemon(ctx, &store.Daemon{ID: 999, Name: "x", Command: "x"}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("update of a missing row: %v", err)
	}

	all, _ := st.ListDaemons(ctx)
	forSite, _ := st.DaemonsForSite(ctx, 8)
	if len(all) != 2 || all[0].ID != a.ID || len(forSite) != 1 || forSite[0].ID != b.ID {
		t.Fatalf("list %d, for site %d", len(all), len(forSite))
	}

	// IDs are never reused: a new program must not show an old one's journal.
	if err := st.DeleteDaemon(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	c := &store.Daemon{Name: "New", SiteID: 8, Command: "sleep 1"}
	if err := st.SaveDaemon(ctx, c); err != nil || c.ID == b.ID {
		t.Fatalf("new ID %d after deleting %d: %v", c.ID, b.ID, err)
	}
	if _, err := st.DaemonByID(ctx, b.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deleted row: %v", err)
	}
}
