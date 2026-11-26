package store

import (
	"context"
	"errors"
	"testing"
)

func TestApps(t *testing.T) {
	ctx := context.Background()
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	site := &Site{Domain: "vault.example.com", SysUser: "vault_example_com"}
	if err := st.CreateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	a := &App{Name: "vault_example_com", AppID: "vaultwarden", Kind: "docker", SiteID: site.ID, Version: "1.37.5",
		Port: 20001, ExtraPorts: map[int]int{9000: 20002}, Inputs: map[string]string{"x": "y"}, AdminEmail: "a@example.com"}
	if err := st.CreateApp(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateApp(ctx, &App{Name: "vault_example_com", AppID: "x", Kind: "docker", Version: "1"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate name: %v", err)
	}
	svc := &App{Name: "postgresql", AppID: "postgresql", Kind: "service", Version: "18.7", Port: 5432}
	if err := st.CreateApp(ctx, svc); err != nil {
		t.Fatal(err)
	}
	if err := st.SetAppSecrets(ctx, a.ID, map[string]string{"admin_token": "t1", "hash": "$argon2id$x"}); err != nil {
		t.Fatal(err)
	}
	st.SetAppSecrets(ctx, a.ID, map[string]string{"admin_token": "t2"})
	sec, _ := st.AppSecrets(ctx, a.ID)
	if sec["admin_token"] != "t2" || sec["hash"] != "$argon2id$x" {
		t.Fatalf("secrets %v", sec)
	}
	got, err := st.AppForSite(ctx, site.ID)
	if err != nil || got.Name != a.Name || got.ExtraPorts[9000] != 20002 || got.Inputs["x"] != "y" || got.State != "installing" {
		t.Fatalf("got %+v %v", got, err)
	}
	got.State, got.Version = "ready", "1.37.6"
	if err := st.UpdateApp(ctx, got); err != nil {
		t.Fatal(err)
	}
	if g, _ := st.AppByName(ctx, "vault_example_com"); g.State != "ready" || g.Version != "1.37.6" {
		t.Fatalf("update %+v", g)
	}
	if s, _ := st.AppByName(ctx, "postgresql"); s.SiteID != 0 {
		t.Fatal("service app has a site")
	}
	// Deleting the site deletes its app and secrets.
	if err := st.DeleteSite(ctx, site.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AppByName(ctx, "vault_example_com"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("app kept after its site: %v", err)
	}
	if sec, _ := st.AppSecrets(ctx, a.ID); len(sec) != 0 {
		t.Fatal("secrets kept")
	}
	list, _ := st.ListApps(ctx)
	if len(list) != 1 || list[0].Name != "postgresql" {
		t.Fatalf("list %+v", list)
	}
}
