package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// GitDeploy is a website deployed from a Git repository.
type GitDeploy struct {
	SiteID int64
	// Repo is the repository's https or SSH address, Branch the branch to
	// deploy and Target where it goes: "root" (the site folder) or "public"
	// (the web folder).
	Repo   string
	Branch string
	Target string
	// Script runs as the site's user in the target folder after each deploy.
	Script string
	// HookSecret signs (GitHub, Gitea) or is sent with (GitLab) webhooks.
	HookSecret string
	// LastCommit is "<hash> <subject line>" of the last deployed commit.
	LastCommit   string
	LastDeployAt time.Time
	LastStatus   string // "", "ok" or "failed"
}

// GitDeploy returns a site's Git settings, or ErrNotFound.
func (s *Store) GitDeploy(ctx context.Context, siteID int64) (*GitDeploy, error) {
	var g GitDeploy
	var at int64
	err := s.db.QueryRowContext(ctx,
		`SELECT site_id, repo, branch, target, script, hook_secret, last_commit, last_deploy_at, last_status
		 FROM git_deploys WHERE site_id = ?`, siteID).
		Scan(&g.SiteID, &g.Repo, &g.Branch, &g.Target, &g.Script, &g.HookSecret, &g.LastCommit, &at, &g.LastStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if at > 0 {
		g.LastDeployAt = time.Unix(at, 0)
	}
	return &g, nil
}

// SaveGitDeploy creates or updates a site's Git settings and webhook
// secret. The result of the last deploy is kept.
func (s *Store) SaveGitDeploy(ctx context.Context, g *GitDeploy) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO git_deploys (site_id, repo, branch, target, script, hook_secret) VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(site_id) DO UPDATE SET repo = excluded.repo, branch = excluded.branch,
		   target = excluded.target, script = excluded.script, hook_secret = excluded.hook_secret`,
		g.SiteID, g.Repo, g.Branch, g.Target, g.Script, g.HookSecret)
	return err
}

// SetGitDeployResult records the outcome of a deploy.
func (s *Store) SetGitDeployResult(ctx context.Context, siteID int64, commit, status string, at time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE git_deploys SET last_commit = ?, last_status = ?, last_deploy_at = ? WHERE site_id = ?`,
		commit, status, at.Unix(), siteID)
	return err
}

// DeleteGitDeploy forgets a site's Git settings.
func (s *Store) DeleteGitDeploy(ctx context.Context, siteID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM git_deploys WHERE site_id = ?`, siteID)
	return err
}
