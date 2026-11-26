package api

import (
	"context"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/store"
)

func TestStagingLink(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	created := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	newSite := func(domain, user string, at time.Time) *store.Site {
		site := &store.Site{Domain: domain, SysUser: user, PHPVersion: "8.3", CreatedAt: at}
		if err := e.store.CreateSite(ctx, site); err != nil {
			t.Fatal(err)
		}
		return site
	}
	live := newSite("example.com", "example_com", created)
	staging := newSite("staging.example.com", "staging_example_com", created.Add(time.Hour))

	if err := e.srv.saveStagingLink(ctx, live, staging.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := e.srv.stagingLive(ctx, staging); err != nil || got == nil || got.ID != live.ID {
		t.Fatalf("live = %+v, %v", got, err)
	}
	if got, _ := e.srv.stagingLive(ctx, live); got != nil {
		t.Fatalf("the live site is not a staging copy: %+v", got)
	}

	// A site made later with the same ID and name is not the staging copy.
	if err := e.store.DeleteSite(ctx, staging.ID); err != nil {
		t.Fatal(err)
	}
	again := newSite("staging.example.com", "staging_example_com", created.Add(2*time.Hour))
	if got, _ := e.srv.stagingLive(ctx, again); got != nil {
		t.Fatalf("a new site (id %d, old id %d) was taken for the staging copy", again.ID, staging.ID)
	}

	// Deleting the staging site forgets the link.
	if err := e.srv.saveStagingLink(ctx, live, again.ID); err != nil {
		t.Fatal(err)
	}
	e.srv.forgetStagingLink(ctx, again.ID)
	if v, _ := e.store.Setting(ctx, stagingKey(again.ID)); v != "" {
		t.Fatalf("link left: %q", v)
	}
}
