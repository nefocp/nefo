package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAPIKeys(t *testing.T) {
	ctx := context.Background()
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	uid, err := st.CreateUser(ctx, "admin", "x")
	if err != nil {
		t.Fatal(err)
	}

	created := time.Unix(1_900_000_000, 0)
	k := &APIKey{UserID: uid, Name: "deploy-bot", Prefix: "nefo_abcdef", Hash: "h1", Access: APIKeyFull,
		Addresses: []string{"203.0.113.7", "10.0.0.0/8"}, CreatedAt: created, ExpiresAt: created.AddDate(0, 0, 30)}
	if err := st.CreateAPIKey(ctx, k, 2); err != nil || k.ID == 0 {
		t.Fatalf("create: %v, id %d", err, k.ID)
	}
	got, err := st.APIKeyByID(ctx, k.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "deploy-bot" || got.Prefix != "nefo_abcdef" || got.Hash != "h1" || got.ReadOnly() ||
		len(got.Addresses) != 2 || got.Addresses[1] != "10.0.0.0/8" || !got.CreatedAt.Equal(created) ||
		!got.ExpiresAt.Equal(created.AddDate(0, 0, 30)) || !got.LastUsedAt.IsZero() || got.UserID != uid {
		t.Fatalf("got %+v", got)
	}

	// Names are unique, whatever their case; so are hashes.
	if err := st.CreateAPIKey(ctx, &APIKey{UserID: uid, Name: "Deploy-Bot", Hash: "h2", Access: APIKeyRead}, 2); !errors.Is(err, ErrConflict) {
		t.Fatalf("same name: %v", err)
	}
	if err := st.CreateAPIKey(ctx, &APIKey{UserID: uid, Name: "other", Hash: "h1", Access: APIKeyRead}, 2); !errors.Is(err, ErrConflict) {
		t.Fatalf("same hash: %v", err)
	}
	ro := &APIKey{UserID: uid, Name: "monitor", Hash: "h3", Access: APIKeyRead}
	if err := st.CreateAPIKey(ctx, ro, 2); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateAPIKey(ctx, &APIKey{UserID: uid, Name: "third", Hash: "h4", Access: APIKeyRead}, 2); !errors.Is(err, ErrTooManyKeys) {
		t.Fatalf("over the limit: %v", err)
	}
	if all, err := st.APIKeys(ctx); err != nil || len(all) != 2 || all[0].ID != k.ID || !all[1].ReadOnly() ||
		len(all[1].Addresses) != 0 || !all[1].ExpiresAt.IsZero() {
		t.Fatalf("list: %+v, %v", all, err)
	}

	// Last use is written at most once per interval.
	at := created.Add(time.Hour)
	if wrote, err := st.TouchAPIKey(ctx, k.ID, at, "203.0.113.7", time.Minute); err != nil || !wrote {
		t.Fatalf("first touch: %v %v", wrote, err)
	}
	if wrote, _ := st.TouchAPIKey(ctx, k.ID, at.Add(30*time.Second), "10.1.1.1", time.Minute); wrote {
		t.Fatal("touched twice within a minute")
	}
	got, _ = st.APIKeyByID(ctx, k.ID)
	if !got.LastUsedAt.Equal(at) || got.LastUsedIP != "203.0.113.7" {
		t.Fatalf("last use: %v %s", got.LastUsedAt, got.LastUsedIP)
	}
	if wrote, _ := st.TouchAPIKey(ctx, k.ID, at.Add(time.Minute), "10.1.1.1", time.Minute); !wrote {
		t.Fatal("not touched after a minute")
	}

	if err := st.DeleteAPIKey(ctx, k.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteAPIKey(ctx, k.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
	if _, err := st.APIKeyByID(ctx, k.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted key: %v", err)
	}
	// IDs are not reused, so the audit log never confuses two keys.
	again := &APIKey{UserID: uid, Name: "deploy-bot", Hash: "h5", Access: APIKeyRead}
	if err := st.CreateAPIKey(ctx, again, 2); err != nil || again.ID <= ro.ID {
		t.Fatalf("new key %d after %d: %v", again.ID, ro.ID, err)
	}
}

// A database from before migration 23 gets the empty api_keys table.
func TestAPIKeyMigration(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	uid, err := st.CreateUser(ctx, "admin", "x")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`ALTER TABLE sites DROP COLUMN proxy_url; ALTER TABLE sites DROP COLUMN proxy_opts; DROP TABLE api_keys; ALTER TABLE databases DROP COLUMN engine; ALTER TABLE sites DROP COLUMN app_memory; PRAGMA user_version = 22`); err != nil {
		t.Fatal(err)
	}
	st.Close()
	if st, err = Open(dir); err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var v int
	if err := st.db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil || v < 23 {
		t.Fatalf("user_version %d, %v", v, err)
	}
	if keys, err := st.APIKeys(ctx); err != nil || len(keys) != 0 {
		t.Fatalf("keys: %+v, %v", keys, err)
	}
	if err := st.CreateAPIKey(ctx, &APIKey{UserID: uid, Name: "ci", Hash: "h", Access: APIKeyRead}, 20); err != nil {
		t.Fatal(err)
	}
	// Deleting the user deletes their keys.
	if _, err := st.db.Exec(`DELETE FROM users WHERE id = ?`, uid); err != nil {
		t.Fatal(err)
	}
	if keys, _ := st.APIKeys(ctx); len(keys) != 0 {
		t.Fatalf("keys of a deleted user: %d", len(keys))
	}
}
