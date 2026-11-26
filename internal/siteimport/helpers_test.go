package siteimport

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/database"
	"github.com/nefocp/nefo/internal/store"
)

type testLog struct {
	mu sync.Mutex
	bytes.Buffer
}

func (l *testLog) Logf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(&l.Buffer, format+"\n", args...)
}

func (l *testLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.Buffer.String()
}

// te is an archive entry for the builders.
type te struct {
	name string
	body string
	typ  byte // tar type; 0 for a regular file
	mode int64
	link string
}

func file(name, body string) te        { return te{name: name, body: body} }
func dir(name string) te               { return te{name: name, typ: tar.TypeDir, mode: 0o755} }
func link(name, target string) te      { return te{name: name, typ: tar.TypeSymlink, link: target} }
func modeFile(name string, m int64) te { return te{name: name, body: "x", mode: m} }

func writeTar(t *testing.T, w io.Writer, entries []te) {
	t.Helper()
	tw := tar.NewWriter(w)
	for _, e := range entries {
		h := &tar.Header{Name: e.name, Mode: e.mode, Typeflag: e.typ, Linkname: e.link, ModTime: time.Unix(1700000000, 0)}
		if h.Typeflag == 0 {
			h.Typeflag = tar.TypeReg
		}
		if h.Mode == 0 {
			h.Mode = 0o644
		}
		if h.Typeflag == tar.TypeReg {
			h.Size = int64(len(e.body))
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			io.WriteString(tw, e.body)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
}

// makeTarGz writes a .tar.gz in dir and returns its path.
func makeTarGz(t *testing.T, dir, name string, entries []te) string {
	t.Helper()
	p := filepath.Join(dir, name)
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	writeTar(t, gz, entries)
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return p
}

func makeZip(t *testing.T, dir, name string, entries []te) string {
	t.Helper()
	p := filepath.Join(dir, name)
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		mode := os.FileMode(e.mode)
		if mode == 0 {
			mode = 0o644
		}
		if e.typ == tar.TypeSymlink {
			mode |= os.ModeSymlink
		}
		h.SetMode(mode)
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if e.typ == tar.TypeSymlink {
			io.WriteString(w, e.link)
		} else {
			io.WriteString(w, e.body)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return p
}

func writeGz(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	io.WriteString(gz, body)
	gz.Close()
	if err := os.WriteFile(p, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func openArchive(t *testing.T, p string, sql bool) (*os.File, *Archive) {
	t.Helper()
	f, a, err := Open(p, sql)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f, a
}

// selfSigned makes a certificate for names with its key, valid from
// notBefore for a year.
func selfSigned(t *testing.T, names []string, notBefore time.Time) (certPEM, keyPEM string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(42), Subject: pkix.Name{CommonName: names[0]}, DNSNames: names,
		NotBefore: notBefore, NotAfter: notBefore.AddDate(1, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	kder, _ := x509.MarshalECPrivateKey(key)
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
		string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kder}))
}

// fakeHost is a panel in a temporary folder that records what an import
// asks of it.
type fakeHost struct {
	t       *testing.T
	base    string
	mu      sync.Mutex
	nextID  int64
	sites   map[string]*store.Site
	dbs     map[string]*store.Database
	newDBs  []database.NewDB
	dumps   map[string]string // database -> SQL received
	steps   []string
	notes   map[string]string
	failDB  string // ImportDB fails for this database
	blockDB string // ImportDB waits for the context to end
}

func newFakeHost(t *testing.T) *fakeHost {
	return &fakeHost{t: t, base: t.TempDir(), sites: map[string]*store.Site{}, dbs: map[string]*store.Database{},
		dumps: map[string]string{}, notes: map[string]string{}}
}

func (f *fakeHost) root(domain string) string { return filepath.Join(f.base, "www", domain) }

func (f *fakeHost) step(format string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.steps = append(f.steps, fmt.Sprintf(format, args...))
}

func (f *fakeHost) has(prefix string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.steps {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

func (f *fakeHost) host() Host {
	return Host{
		Root:      f.root,
		LookupIDs: func(string) (int, int, error) { return -1, -1, nil },
		CreateSite: func(ctx context.Context, domain string, aliases []string, php string) (*store.Site, error) {
			f.mu.Lock()
			f.nextID++
			id := f.nextID
			f.mu.Unlock()
			for _, d := range []string{"public", "logs", "tmp"} {
				if err := os.MkdirAll(filepath.Join(f.root(domain), d), 0o750); err != nil {
					return nil, err
				}
			}
			os.WriteFile(filepath.Join(f.root(domain), "public", "index.html"), []byte("placeholder"), 0o640)
			s := &store.Site{ID: id, Domain: domain, Aliases: aliases, SysUser: strings.ReplaceAll(domain, ".", "_"), PHPVersion: php, Disabled: true}
			f.mu.Lock()
			f.sites[domain] = s
			f.mu.Unlock()
			f.step("create site %s php=%s aliases=%s", domain, php, strings.Join(aliases, ","))
			return s, nil
		},
		DeleteSite: func(ctx context.Context, s *store.Site) error {
			f.mu.Lock()
			delete(f.sites, s.Domain)
			f.mu.Unlock()
			os.RemoveAll(f.root(s.Domain))
			f.step("delete site %s", s.Domain)
			return nil
		},
		Suspend: func(ctx context.Context, s *store.Site, on bool) error {
			f.step("suspend %s %v", s.Domain, on)
			return nil
		},
		SetNote: func(ctx context.Context, s *store.Site, note string) error {
			f.mu.Lock()
			f.notes[s.Domain] = note
			f.mu.Unlock()
			return nil
		},
		MarkWordPress: func(ctx context.Context, s *store.Site, lg Logger) error {
			f.step("wordpress %s", s.Domain)
			return nil
		},
		CreateDB: func(ctx context.Context, n database.NewDB) (*store.Database, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.nextID++
			d := &store.Database{ID: f.nextID, Name: n.Name, Username: cmpOr(n.User, n.Name), Password: n.Password, SiteID: n.SiteID}
			if n.Password == "" && n.Hash == "" {
				d.Password = "Generated-Pass-2026"
			}
			f.dbs[n.Name] = d
			f.newDBs = append(f.newDBs, n)
			f.steps = append(f.steps, "create db "+n.Name)
			return d, nil
		},
		DeleteDB: func(ctx context.Context, d *store.Database) error {
			f.mu.Lock()
			delete(f.dbs, d.Name)
			f.mu.Unlock()
			f.step("delete db %s", d.Name)
			return nil
		},
		ImportDB: func(ctx context.Context, name string, r io.Reader) error {
			if name == f.blockDB {
				<-ctx.Done()
				return ctx.Err()
			}
			b, err := io.ReadAll(r)
			if err != nil {
				return err
			}
			f.mu.Lock()
			f.dumps[name] = string(b)
			f.mu.Unlock()
			if name == f.failDB {
				return errors.New("ERROR 1064 (42000) at line 1: syntax error")
			}
			return nil
		},
		ReplaceInDB: func(ctx context.Context, s *store.Site, from, to string, lg Logger) error {
			f.step("replace %s %s -> %s", s.Domain, from, to)
			return nil
		},
		InstallCert: func(ctx context.Context, s *store.Site, chain, key []byte) error {
			if !bytes.Contains(chain, []byte("BEGIN CERTIFICATE")) || !bytes.Contains(key, []byte("PRIVATE KEY")) {
				return errors.New("bad PEM")
			}
			f.step("cert %s", s.Domain)
			return nil
		},
		AddCron: func(ctx context.Context, s *store.Site, name, schedule, command string) error {
			f.step("cron %s %s %s", s.SysUser, schedule, command)
			return nil
		},
		CheckSpace: func(files, dbs int64) error { return nil },
		Now:        func() time.Time { return time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC) },
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// A self-signed website certificate that "openssl req -x509" marked as a
// CA is still the website's; a CA that another one signed is the chain's.
func TestCertSelfSignedCA(t *testing.T) {
	now := time.Now()
	mk := func(name string, parent *x509.Certificate, parentKey *ecdsa.PrivateKey) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
		key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		tmpl := &x509.Certificate{SerialNumber: big.NewInt(now.UnixNano()), Subject: pkix.Name{CommonName: name}, DNSNames: []string{name},
			NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(0, 3, 0), IsCA: true, BasicConstraintsValid: true,
			KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign}
		if parent == nil {
			parent, parentKey = tmpl, key
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &key.PublicKey, parentKey)
		if err != nil {
			t.Fatal(err)
		}
		c, _ := x509.ParseCertificate(der)
		kder, _ := x509.MarshalPKCS8PrivateKey(key)
		return c, key, append(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: kder}), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})...)
	}
	_, _, own := mk("example.com", nil, nil)
	p := &certPool{}
	p.add("apache_tls/example.com", own)
	if pk := p.best("example.com", []string{"example.com"}, now); pk == nil || !pk.summary.Usable {
		t.Fatalf("self-signed CA-marked certificate: %+v", pk)
	}
	root, rootKey, _ := mk("root.example.net", nil, nil)
	_, _, inter := mk("other.com", root, rootKey)
	p = &certPool{}
	p.add("apache_tls/other.com", inter)
	if pk := p.best("other.com", []string{"other.com"}, now); pk != nil {
		t.Fatalf("a CA signed by another was taken as the website's: %+v", pk.summary)
	}
}
