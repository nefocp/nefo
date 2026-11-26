package siteimport

import (
	"errors"
	"io/fs"
	"os"
	"strconv"
	"strings"
)

// fixedFiles are the files at the top of a website that may name its old
// folder: PHP settings (auto_prepend_file, open_basedir), Wordfence's
// firewall loader, the web server's .htaccess and wp-config.php.
var fixedFiles = []string{".user.ini", "php.ini", "wordfence-waf.php", ".htaccess", "wp-config.php"}

const (
	cpHandlerBegin = "# php -- BEGIN cPanel-generated handler, do not edit"
	cpHandlerEnd   = "# php -- END cPanel-generated handler, do not edit"
)

// fixFiles rewrites the files of fixedFiles in a website's public folder:
// the old folder becomes the new one, cPanel's PHP handler lines leave
// .htaccess (with Apache they could make it serve PHP files as text), and
// rewrite (when set) adapts wp-config.php. It reports whether the website
// has a wp-config.php, and what could not be done.
func fixFiles(x *extractor, oldRoot, newRoot string, cpanel bool, rewrite func(string) (string, error), lg Logger) (bool, []string) {
	var warnings []string
	wordpress := false
	for _, name := range fixedFiles {
		fi, err := x.root.Lstat(name)
		if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxConfig {
			continue
		}
		b, err := x.root.ReadFile(name)
		if err != nil {
			warnings = append(warnings, name+": "+err.Error())
			continue
		}
		s := string(b)
		orig := s
		if oldRoot != "" && oldRoot != newRoot {
			s = replacePath(s, oldRoot, newRoot)
		}
		if name == ".htaccess" && cpanel {
			s = stripBlock(s, cpHandlerBegin, cpHandlerEnd)
		}
		if name == "wp-config.php" {
			wordpress = true
			if rewrite != nil {
				out, err := rewrite(s)
				if err != nil {
					warnings = append(warnings, err.Error())
				} else {
					s = out
				}
			}
		}
		if s == orig {
			continue
		}
		if err := x.replace(name, []byte(s), fi.Mode().Perm()); err != nil {
			warnings = append(warnings, name+": "+err.Error())
			continue
		}
		if s != orig && name != "wp-config.php" {
			lg.Logf("Updated %s for this server", name)
		}
	}
	return wordpress, warnings
}

// stripBlock removes the lines from begin to end (included).
func stripBlock(s, begin, end string) string {
	i := strings.Index(s, begin)
	if i < 0 {
		return s
	}
	j := strings.Index(s[i:], end)
	if j < 0 {
		return s
	}
	j += i + len(end)
	if j < len(s) && s[j] == '\r' {
		j++
	}
	if j < len(s) && s[j] == '\n' {
		j++
	}
	return s[:i] + s[j:]
}

// replace writes a file of the website again: a new file next to it,
// renamed over it, owned by the site's user. The website's own user could
// have put a link there meanwhile; the rename replaces the link itself.
func (x *extractor) replace(name string, data []byte, mode fs.FileMode) error {
	var tmp string
	var f *os.File
	for i := 0; ; i++ {
		tmp = ".nefo-import-" + strconv.Itoa(os.Getpid()) + "-" + strconv.Itoa(i)
		var err error
		f, err = x.root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			break
		}
		if !errors.Is(err, fs.ErrExist) || i > 100 {
			return err
		}
	}
	_, err := f.Write(data)
	if err == nil {
		err = f.Chmod(fileMode(mode))
	}
	if err == nil && x.uid >= 0 {
		err = f.Chown(x.uid, x.gid)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = x.root.Rename(tmp, name)
	}
	if err != nil {
		x.root.Remove(tmp)
	}
	return err
}
