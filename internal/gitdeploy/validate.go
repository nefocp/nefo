package gitdeploy

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/nefocp/nefo/internal/usererr"
)

// Repository addresses are checked strictly, because git and ssh read some
// strings as options or as other transports ("--upload-pack=…", "ext::sh
// …", "-oProxyCommand=…"). Only three forms are accepted:
//
//	https://host[:port]/owner/repo[.git]
//	ssh://user@host[:port]/owner/repo[.git]
//	user@host:owner/repo[.git]            (the scp-like form GitHub shows)
//
// Every part starts with a letter or digit, so nothing can start with "-",
// and spaces, quotes and other shell characters are not allowed anywhere.
const (
	hostPart = `[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)*`
	userPart = `[A-Za-z0-9_][A-Za-z0-9._-]{0,63}`
	segPart  = `[A-Za-z0-9._~][A-Za-z0-9._~-]{0,99}`
	portPart = `(?::([0-9]{1,5}))?`

	maxRepoLen   = 300
	maxBranchLen = 100
	// MaxScript is the longest deploy script, in characters.
	MaxScript = 2000
)

var (
	httpsRepoRe = regexp.MustCompile(`^https://` + hostPart + portPart + `((?:/` + segPart + `){2,})/?$`)
	sshRepoRe   = regexp.MustCompile(`^ssh://` + userPart + `@` + hostPart + portPart + `((?:/` + segPart + `)+)$`)
	scpRepoRe   = regexp.MustCompile(`^` + userPart + `@` + hostPart + `:(/?` + segPart + `(?:/` + segPart + `)*)$`)
	branchRe    = regexp.MustCompile(`^[A-Za-z0-9._/-]{1,100}$`)
)

// ValidateRepo checks a repository address and returns it without
// surrounding spaces.
func ValidateRepo(repo string) (string, error) {
	repo = strings.TrimSpace(repo)
	short := repo
	if len(short) > 80 {
		short = short[:80] + "…"
	}
	if repo == "" || len(repo) > maxRepoLen {
		return "", usererr.New("invalid_git_repo", short)
	}
	lower := strings.ToLower(repo)
	if strings.HasPrefix(lower, "http://") {
		return "", usererr.New("git_plain_http")
	}
	// A password or token in the address would show up in logs and in
	// .git/config: private repositories use the deploy key instead.
	if rest, ok := strings.CutPrefix(lower, "https://"); ok {
		authority, _, _ := strings.Cut(rest, "/")
		if strings.Contains(authority, "@") {
			return "", usererr.New("git_url_credentials")
		}
	}
	if rest, ok := strings.CutPrefix(lower, "ssh://"); ok {
		authority, _, _ := strings.Cut(rest, "/")
		if user, _, found := strings.Cut(authority, "@"); found && strings.Contains(user, ":") {
			return "", usererr.New("git_url_credentials")
		}
	}

	var m []string
	switch {
	case strings.HasPrefix(repo, "https://"):
		m = httpsRepoRe.FindStringSubmatch(repo)
	case strings.HasPrefix(repo, "ssh://"):
		m = sshRepoRe.FindStringSubmatch(repo)
	default:
		if sm := scpRepoRe.FindStringSubmatch(repo); sm != nil {
			m = []string{sm[0], "", sm[1]}
		}
	}
	if m == nil {
		return "", usererr.New("invalid_git_repo", short)
	}
	if port := m[1]; port != "" {
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return "", usererr.New("invalid_git_repo", short)
		}
	}
	for _, seg := range strings.Split(strings.Trim(m[2], "/"), "/") {
		if strings.Trim(seg, ".") == "" { // "." and ".."
			return "", usererr.New("invalid_git_repo", short)
		}
	}
	return repo, nil
}

// IsSSH reports whether a (valid) repository address uses SSH, and so
// needs the deploy key.
func IsSSH(repo string) bool { return !strings.HasPrefix(repo, "https://") }

// ValidateBranch checks a branch name: a subset of what git allows
// (git check-ref-format), without anything git could read as an option or
// a revision expression.
func ValidateBranch(branch string) error {
	bad := !branchRe.MatchString(branch) ||
		strings.HasPrefix(branch, "-") || strings.HasPrefix(branch, "/") || strings.HasSuffix(branch, "/") ||
		strings.HasSuffix(branch, ".") || strings.Contains(branch, "..") || strings.Contains(branch, "//") ||
		branch == "HEAD"
	if !bad {
		for _, part := range strings.Split(branch, "/") {
			if strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
				bad = true
			}
		}
	}
	if bad {
		return usererr.New("invalid_git_branch")
	}
	return nil
}

// ValidateTarget checks where the repository goes.
func ValidateTarget(target string) error {
	if target != TargetRoot && target != TargetPublic {
		return usererr.New("invalid_git_target")
	}
	return nil
}

// CleanScript checks a deploy script and returns it with Unix line endings
// and without surrounding blank space. The administrator writes it, as with
// cron jobs; it runs as the site's user.
func CleanScript(script string) (string, error) {
	script = strings.TrimSpace(strings.ReplaceAll(script, "\r\n", "\n"))
	if !utf8.ValidString(script) || utf8.RuneCountInString(script) > MaxScript {
		return "", usererr.New("invalid_git_script")
	}
	for _, r := range script {
		if (r < 0x20 && r != '\n' && r != '\t') || r == 0x7f {
			return "", usererr.New("invalid_git_script")
		}
	}
	return script, nil
}
