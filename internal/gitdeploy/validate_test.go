package gitdeploy

import (
	"errors"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/usererr"
)

func errCode(err error) string {
	var ue *usererr.Error
	if errors.As(err, &ue) {
		return ue.Code
	}
	if err != nil {
		return "other: " + err.Error()
	}
	return ""
}

func TestValidateRepo(t *testing.T) {
	good := []string{
		"https://github.com/octocat/Hello-World.git",
		"https://github.com/octocat/Hello-World",
		"https://github.com/octocat/Hello-World/",
		"https://gitlab.com/group/subgroup/project.git",
		"https://git.example.com:8443/team/site.git",
		"https://github.com/org/.github",
		"ssh://git@github.com/owner/repo.git",
		"ssh://git@git.example.com:2222/owner/repo.git",
		"ssh://forgejo@codeberg.org/owner/repo",
		"git@github.com:owner/repo.git",
		"git@gitlab.com:group/sub/repo.git",
		"git@git.example.com:/srv/git/site.git",
		"  git@github.com:owner/repo.git  ",
	}
	for _, r := range good {
		got, err := ValidateRepo(r)
		if err != nil {
			t.Errorf("ValidateRepo(%q) = %v", r, err)
		} else if got != strings.TrimSpace(r) {
			t.Errorf("ValidateRepo(%q) returned %q", r, got)
		}
	}

	bad := map[string]string{
		"":                                                      "invalid_git_repo",
		"--upload-pack=touch /tmp/x":                            "invalid_git_repo",
		"-oProxyCommand=sh":                                     "invalid_git_repo",
		"ext::sh -c touch% /tmp/pwned":                          "invalid_git_repo",
		"ext::sh":                                               "invalid_git_repo",
		"fd::17":                                                "invalid_git_repo",
		"file:///etc":                                           "invalid_git_repo",
		"/var/www/other.com":                                    "invalid_git_repo",
		"../other":                                              "invalid_git_repo",
		"git://github.com/owner/repo.git":                       "invalid_git_repo",
		"ftp://example.com/owner/repo":                          "invalid_git_repo",
		"https://github.com/owner/repo name":                    "invalid_git_repo",
		"https://github.com/owner/repo.git x":                   "invalid_git_repo",
		"https://github.com/owner\t/repo":                       "invalid_git_repo",
		"https://github.com/owner/repo\n":                       "", // trailing space is trimmed
		"https://github.com/owner/re\npo":                       "invalid_git_repo",
		"https://github.com/repo":                               "invalid_git_repo", // needs owner/repo
		"https://-github.com/owner/repo":                        "invalid_git_repo",
		"https://github.com/-owner/repo":                        "invalid_git_repo",
		"https://github.com/owner/..":                           "invalid_git_repo",
		"https://github.com/../..":                              "invalid_git_repo",
		"https://github.com/owner/repo?x=1":                     "invalid_git_repo",
		"https://github.com/owner/repo#frag":                    "invalid_git_repo",
		"https://github.com/owner/$(id)":                        "invalid_git_repo",
		"https://github.com/owner/`id`":                         "invalid_git_repo",
		"https://github.com/owner/repo;id":                      "invalid_git_repo",
		"https://github.com:99999/owner/repo":                   "invalid_git_repo",
		"https://github.com:0/owner/repo":                       "invalid_git_repo",
		"https://tok@github.com/owner/repo":                     "git_url_credentials",
		"https://user:ghp_x@github.com/o/r":                     "git_url_credentials",
		"HTTPS://user:pass@github.com/o/r.git":                  "git_url_credentials",
		"ssh://git:pw@github.com/owner/repo":                    "git_url_credentials",
		"http://github.com/owner/repo.git":                      "git_plain_http",
		"ssh://github.com/owner/repo.git":                       "invalid_git_repo", // needs a user
		"ssh://-oProxyCommand=x@host/o/r":                       "invalid_git_repo",
		"ssh://git@-oProxyCommand=x/o/r":                        "invalid_git_repo",
		"git@-oProxyCommand=sh:owner/repo":                      "invalid_git_repo",
		"-git@github.com:owner/repo":                            "invalid_git_repo",
		"git@github.com:-owner/repo":                            "invalid_git_repo",
		"git@github.com:--upload-pack=x":                        "invalid_git_repo",
		"git@github.com:owner/../../etc":                        "invalid_git_repo",
		"git@github.com:owner/repo name":                        "invalid_git_repo",
		"git@github.com:":                                       "invalid_git_repo",
		"github.com:owner/repo":                                 "invalid_git_repo",
		"git@github.com/owner/repo":                             "invalid_git_repo",
		"git@gith'ub.com:owner/repo":                            "invalid_git_repo",
		"https://github.com/" + strings.Repeat("a", 300) + "/r": "invalid_git_repo",
	}
	for r, want := range bad {
		_, err := ValidateRepo(r)
		if got := errCode(err); got != want {
			t.Errorf("ValidateRepo(%q) = %q, want %q", r, got, want)
		}
	}
}

func TestIsSSH(t *testing.T) {
	for repo, want := range map[string]bool{
		"https://github.com/o/r.git": false,
		"ssh://git@github.com/o/r":   true,
		"git@github.com:o/r.git":     true,
	} {
		if IsSSH(repo) != want {
			t.Errorf("IsSSH(%q) = %v", repo, !want)
		}
	}
}

func TestValidateBranch(t *testing.T) {
	for _, b := range []string{"main", "master", "release/1.2", "feature/new-ui", "v2.x", "a_b", strings.Repeat("b", 100)} {
		if err := ValidateBranch(b); err != nil {
			t.Errorf("ValidateBranch(%q) = %v", b, err)
		}
	}
	for _, b := range []string{
		"", "-main", "--upload-pack=x", "--force", "main..dev", "../main", "/main", "main/", "a//b",
		"main.", "main.lock", "a.lock/b", ".hidden", "a/.b", "HEAD", "main branch", "main;id",
		"$(id)", "main\n", "ma~1", "main^", "main:dev", "a@{1}", "refs/heads/*", strings.Repeat("b", 101),
	} {
		if errCode(ValidateBranch(b)) != "invalid_git_branch" {
			t.Errorf("ValidateBranch(%q) accepted", b)
		}
	}
}

func TestValidateTarget(t *testing.T) {
	if ValidateTarget("root") != nil || ValidateTarget("public") != nil {
		t.Fatal("valid target refused")
	}
	for _, s := range []string{"", "Public", "../public", "/var/www"} {
		if errCode(ValidateTarget(s)) != "invalid_git_target" {
			t.Errorf("ValidateTarget(%q) accepted", s)
		}
	}
}

func TestCleanScript(t *testing.T) {
	got, err := CleanScript("  composer install --no-dev\r\nphp artisan migrate --force\n\n")
	if err != nil || got != "composer install --no-dev\nphp artisan migrate --force" {
		t.Fatalf("CleanScript = %q, %v", got, err)
	}
	if got, err := CleanScript(""); err != nil || got != "" {
		t.Fatalf("empty script: %q, %v", got, err)
	}
	if _, err := CleanScript("npm ci &&\tnpm run build | tee log; echo $HOME"); err != nil {
		t.Fatalf("shell syntax refused: %v", err)
	}
	if _, err := CleanScript(strings.Repeat("é", MaxScript)); err != nil {
		t.Fatalf("2000 characters refused: %v", err)
	}
	for _, s := range []string{strings.Repeat("x", MaxScript+1), "echo a\x00b", "echo \x1b[31m", "bad \xff utf-8"} {
		if errCode(func() error { _, err := CleanScript(s); return err }()) != "invalid_git_script" {
			t.Errorf("CleanScript(%q) accepted", s[:min(len(s), 20)])
		}
	}
}
