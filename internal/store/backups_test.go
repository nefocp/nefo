package store

import (
	"context"
	"testing"
)

func TestBackupAppID(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	b := &Backup{Kind: "app", Target: "postgresql", File: "/x/a.tar.gz", Size: 1, SHA256: "aa", AppID: "postgresql"}
	if err := st.CreateBackup(ctx, b); err != nil {
		t.Fatal(err)
	}
	if got, err := st.BackupByID(ctx, b.ID); err != nil || got.AppID != "postgresql" {
		t.Fatalf("got %+v, %v", got, err)
	}

	// A database from before migration 20: app backups of installed apps
	// get their app, others stay empty.
	if err := st.CreateApp(ctx, &App{Name: "ghost_example_com", AppID: "ghost", Kind: "docker", Version: "6"}); err != nil {
		t.Fatal(err)
	}
	// The later migrations run again too: undo them first.
	if _, err := st.db.Exec(`ALTER TABLE sites DROP COLUMN hsts; ALTER TABLE sites DROP COLUMN hsts_subdomains;
		ALTER TABLE sites DROP COLUMN page_cache; DROP TABLE uptime_checks; DROP TABLE site_monitors;
		DROP TABLE api_keys; ALTER TABLE databases DROP COLUMN engine; ALTER TABLE sites DROP COLUMN app_memory;
		ALTER TABLE sites DROP COLUMN proxy_url; ALTER TABLE sites DROP COLUMN proxy_opts;`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`ALTER TABLE backups DROP COLUMN app_id; PRAGMA user_version = 19;
		INSERT INTO backups (kind, target, file, size, sha256, created_at) VALUES
			('app', 'ghost_example_com', '/x/g.tar.gz', 1, 'bb', 1),
			('app', 'gone_example_com', '/x/h.tar.gz', 1, 'cc', 2),
			('site', 'ghost_example_com', '/x/s.tar.gz', 1, 'dd', 3)`); err != nil {
		t.Fatal(err)
	}
	st.Close()
	if st, err = Open(dir); err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	list, err := st.ListBackups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"/x/g.tar.gz": "ghost", "/x/h.tar.gz": "", "/x/s.tar.gz": "", "/x/a.tar.gz": ""}
	for _, b := range list {
		if b.AppID != want[b.File] {
			t.Errorf("%s: app %q, want %q", b.File, b.AppID, want[b.File])
		}
	}
	if len(list) != len(want) {
		t.Fatalf("%d backups", len(list))
	}
}
