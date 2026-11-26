// Package validate checks user input before it is used anywhere near the
// system: file paths, config files or command arguments.
//
// Rule for the whole project: every value that comes from a request must
// pass one of these whitelists first, and is then passed to programs as a
// separate argument (exec.Command(name, arg1, arg2)), never through a shell.
package validate

import (
	"errors"
	"regexp"
	"strings"
)

var (
	labelRe    = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	usernameRe = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
	allDigits  = regexp.MustCompile(`^[0-9]+$`)
	phpRe      = regexp.MustCompile(`^[5-9]\.[0-9]{1,2}$`)
)

var (
	dbNameRe     = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
	dbPasswordRe = regexp.MustCompile(`^[A-Za-z0-9!#%*+,.:=?@^_~-]{12,64}$`)
	reservedDB   = map[string]bool{
		"mysql": true, "information_schema": true, "performance_schema": true, "sys": true,
		"test": true, "root": true, "mariadb": true, "phpmyadmin": true, "debian_sys_maint": true,
	}
)

// DBName checks a database or database user name created by the panel. The
// allowed characters need no quoting in SQL, so names can never inject SQL.
func DBName(s string) error {
	if !dbNameRe.MatchString(s) {
		return errors.New("name must start with a letter and use only a-z, 0-9 and _ (max 32)")
	}
	if reservedDB[s] {
		return errors.New("this name is reserved")
	}
	return nil
}

// DBPassword checks a database password. Quotes, backslashes, spaces and $
// are not allowed, so the password is safe inside SQL, PHP and shell strings.
func DBPassword(s string) error {
	if !dbPasswordRe.MatchString(s) {
		return errors.New("password must be 12-64 characters: letters, digits and ! # % * + , . : = ? @ ^ _ ~ -")
	}
	return nil
}

// PHPVersion checks a PHP version such as "8.3".
func PHPVersion(s string) error {
	if !phpRe.MatchString(s) {
		return errors.New("PHP version must look like 8.3")
	}
	return nil
}

// Domain checks a lowercase domain name such as "example.com" or
// "xn--bcher-kva.example" (internationalised names must be in punycode).
func Domain(s string) error {
	if s == "" || len(s) > 253 {
		return errors.New("domain must be 1-253 characters")
	}
	if s != strings.ToLower(s) {
		return errors.New("domain must be lowercase")
	}
	labels := strings.Split(s, ".")
	if len(labels) < 2 {
		return errors.New("domain needs at least one dot")
	}
	for _, l := range labels {
		if !labelRe.MatchString(l) {
			return errors.New("domain contains an invalid label")
		}
	}
	if allDigits.MatchString(labels[len(labels)-1]) {
		return errors.New("domain cannot end in a number")
	}
	return nil
}

// Alias checks a site alias: a domain, or a wildcard such as "*.example.com"
// (one leading "*." label, which needs a DNS-validated certificate).
func Alias(s string) error {
	if rest, ok := strings.CutPrefix(s, "*."); ok {
		return Domain(rest)
	}
	return Domain(s)
}

// IsWildcard reports whether a name is a wildcard such as "*.example.com".
func IsWildcard(s string) bool { return strings.HasPrefix(s, "*.") }

// SystemUsername checks a Linux account name created by the panel.
func SystemUsername(s string) error {
	if !usernameRe.MatchString(s) {
		return errors.New("username must start with a letter or _, and use only a-z, 0-9, _ or - (max 32)")
	}
	return nil
}

// Password checks a new panel password. bcrypt only reads the first 72 bytes,
// so longer passwords are refused instead of silently truncated.
func Password(s string) error {
	if len(s) < 12 || len(s) > 72 {
		return errors.New("password must be 12-72 bytes")
	}
	return nil
}
