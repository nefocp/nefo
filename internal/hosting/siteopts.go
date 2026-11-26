package hosting

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

// SiteOptions are smaller per-site settings: a custom 404 page, hotlink
// protection for images and media, and an expiry date after which the site
// is suspended.
type SiteOptions struct {
	Custom404    bool
	Hotlink      bool
	HotlinkAllow []string // other sites that may embed the media
	ExpiresAt    time.Time
}

const maxHotlinkAllow = 20

// SetSiteOptions changes a site's options.
func (m *Manager) SetSiteOptions(ctx context.Context, id int64, o SiteOptions) (*store.Site, error) {
	if !m.Supported() {
		return nil, ErrUnsupported
	}
	var allow []string
	for _, d := range o.HotlinkAllow {
		d = normalizeName(d)
		if d == "" || slices.Contains(allow, d) {
			continue
		}
		if validate.Alias(d) != nil {
			return nil, usererr.New("invalid_alias", d)
		}
		allow = append(allow, d)
	}
	if len(allow) > maxHotlinkAllow {
		return nil, usererr.New("too_many_hotlink_sites")
	}
	if err := m.tryLock(); err != nil {
		return nil, err
	}
	defer m.ops.Unlock()
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	old := *site
	site.Custom404, site.Hotlink, site.HotlinkAllow = o.Custom404, o.Hotlink, strings.Join(allow, " ")
	site.ExpiresAt = o.ExpiresAt
	if err := m.updateSite(ctx, site, &old); err != nil {
		return nil, err
	}
	return site, nil
}

// ExpireSites suspends sites whose expiry date has passed; report is called
// for each one. It runs from the hourly maintenance loop.
func (m *Manager) ExpireSites(ctx context.Context, report func(domain string)) {
	if !m.Supported() {
		return
	}
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		return
	}
	now := m.now()
	for _, s := range sites {
		if s.ExpiresAt.IsZero() || s.Disabled || now.Before(s.ExpiresAt) {
			continue
		}
		// Wait for the lock like certificate renewal does. At start-up the
		// configuration refresh holds it for a few seconds, and skipping
		// would leave the site up for another hour.
		m.ops.Lock()
		if ctx.Err() != nil {
			m.ops.Unlock()
			return
		}
		site, err := m.store.SiteByID(ctx, s.ID)
		if err != nil {
			m.ops.Unlock()
			continue // deleted meanwhile
		}
		if site.ExpiresAt.IsZero() || site.Disabled || now.Before(site.ExpiresAt) {
			m.ops.Unlock()
			continue // changed meanwhile
		}
		old := *site
		site.Disabled = true
		err = m.updateSite(ctx, site, &old)
		m.ops.Unlock()
		if err != nil {
			m.log.Warn("suspend expired site", "domain", s.Domain, "err", err)
			continue
		}
		report(s.Domain)
	}
}
