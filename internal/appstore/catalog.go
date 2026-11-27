// Package appstore holds the app store's catalog: which applications the
// panel can install, at which exact versions, and how. Docker apps carry a
// recipe, which is data (never a shell script): the panel turns it into a
// Compose file (see render.go) that goes through the same safety checks as
// any Compose project. PHP apps name an installer compiled into the panel.
//
// The catalog is embedded in the binary. A newer catalog signed with the
// release key can replace it (see remote.go); without a key (development
// builds) only the embedded one is used.
package appstore

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
)

// Schema is the catalog format this panel understands.
const Schema = 1

// Kinds of app.
const (
	KindPHP     = "php"     // installed into an empty PHP website
	KindDocker  = "docker"  // a container behind its own website
	KindService = "service" // a container without a website, on 127.0.0.1
)

//go:embed catalog.json
var embedded []byte

// Catalog is the list of apps.
type Catalog struct {
	Schema         int    `json:"schema"`
	CatalogVersion int64  `json:"catalog_version"`
	Generated      string `json:"generated"`
	MinPanel       string `json:"min_panel"`
	Apps           []*App `json:"apps"`
}

// Text is a translated text: "en" and "vi".
type Text map[string]string

// App is one application.
type App struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Kind        string `json:"kind"`
	Version     string `json:"version"`
	Description Text   `json:"description"`
	Notes       Text   `json:"notes,omitempty"`
	License     string `json:"license"`
	// SourceAvailable marks a licence that is not OSI-approved (the UI
	// shows a badge).
	SourceAvailable bool   `json:"source_available,omitempty"`
	Homepage        string `json:"homepage"`
	Source          string `json:"source,omitempty"`
	Logo            string `json:"logo"`
	Replaces        string `json:"replaces,omitempty"`
	// MinPanel is the oldest panel version that can install this app (a
	// recipe that needs code an older panel does not have). Older panels
	// leave the app out of a downloaded catalog (see ParseRemote).
	MinPanel  string    `json:"min_panel,omitempty"`
	Resources Resources `json:"resources"`
	Requires  Requires  `json:"requires"`
	Admin     Admin     `json:"admin"`
	Updates   Updates   `json:"updates"`
	Docker    *Recipe   `json:"docker,omitempty"`
	PHP       *PHPApp   `json:"php,omitempty"`
}

// Resources are what an app needs. MinRAMMB and the disk figure are
// refusals; RecommendedRAMMB is a warning (see resources.go).
type Resources struct {
	// MemoryLimitMB is the sum of the containers' limits. MemoryAuto
	// (Ollama) sets the limit from the server's memory instead.
	MemoryLimitMB    int  `json:"memory_limit_mb,omitempty"`
	MemoryAuto       bool `json:"memory_auto,omitempty"`
	MinRAMMB         int  `json:"min_ram_mb"`
	RecommendedRAMMB int  `json:"recommended_ram_mb"`
	DiskMB           int  `json:"disk_mb"`  // data and code once installed
	ImageMB          int  `json:"image_mb"` // unpacked images
	IdleRAMMB        int  `json:"idle_ram_mb,omitempty"`
	// MinFreeDiskMB, if set, replaces ImageMB+DiskMB+1024 as the refusal.
	MinFreeDiskMB int `json:"min_free_disk_mb,omitempty"`
}

// Requires are conditions on the server.
type Requires struct {
	HTTPS    bool                `json:"https,omitempty"` // shown: the app needs a certificate to work fully
	Arch     []string            `json:"arch,omitempty"`
	CPUFlags map[string][]string `json:"cpu_flags,omitempty"` // by architecture
}

// Admin says how the first administrator is created and shown.
type Admin struct {
	// Method: "form" (PHP installers ask for it), "generated" (the panel
	// sets a generated password), "bootstrap_env", "invite", "logs" (the
	// app prints a one-time password), "signup" (create it in the app),
	// "site_password" (no accounts: the site gets a password), "none".
	Method         string `json:"method"`
	User           string `json:"user,omitempty"`
	UserSecret     string `json:"user_secret,omitempty"`
	PasswordSecret string `json:"password_secret,omitempty"`
	EmailLogin     bool   `json:"email_login,omitempty"` // signs in with the e-mail address
	URLPath        string `json:"url_path,omitempty"`
	LogRegex       string `json:"log_regex,omitempty"`
	Connection     string `json:"connection,omitempty"` // service apps: connection string
	Note           Text   `json:"note,omitempty"`
}

// Updates says how an app is updated.
type Updates struct {
	// Strategy: "recreate" (the panel recreates the containers), "builtin"
	// (the app's own updater), "manual" or "user" (the user's project).
	Strategy string `json:"strategy"`
	// Breaking ("major" or "minor") blocks a one-click update when that
	// part of the version changes.
	Breaking string `json:"breaking,omitempty"`
	Note     Text   `json:"note,omitempty"`
}

// ---- Docker recipes ----

// Recipe describes the containers of a Docker or service app.
type Recipe struct {
	Web        *Web                   `json:"web,omitempty"`     // docker apps: what the website forwards to
	Service    *ServicePort           `json:"service,omitempty"` // service apps: the published port
	ExtraPorts []ExtraPort            `json:"extra_ports,omitempty"`
	Networks   []string               `json:"networks,omitempty"` // shared networks
	Services   map[string]*Service    `json:"services"`
	Secrets    map[string]*SecretSpec `json:"secrets,omitempty"`
	Inputs     map[string]*Input      `json:"inputs,omitempty"`
	// BeforeStart run in one-off containers once the dependencies of the
	// main service are up (database set-up); AfterStart once the app
	// answers.
	BeforeStart []Step     `json:"before_start,omitempty"`
	AfterStart  []Step     `json:"after_start,omitempty"`
	Backup      BackupSpec `json:"backup"`
	Legacy      *Legacy    `json:"legacy,omitempty"`
}

// Web is the service the app's website forwards to.
type Web struct {
	Service string `json:"service"`
	Port    int    `json:"port"`
	Health  string `json:"health"` // path that answers below 500 once the app runs
	// UploadMB raises the site's body size limit.
	UploadMB int `json:"upload_mb,omitempty"`
	// BasicAuth: the app has no accounts; its site always gets a password.
	BasicAuth bool `json:"basic_auth,omitempty"`
}

// ServicePort is the port a service app publishes on 127.0.0.1.
type ServicePort struct {
	Service    string `json:"service"`
	Port       int    `json:"port"`
	PreferPort int    `json:"prefer_port,omitempty"` // used when free
	Alias      string `json:"alias,omitempty"`       // name on the shared networks
	Health     string `json:"health,omitempty"`      // HTTP path, if the port speaks HTTP
}

// ExtraPort is another port published on 127.0.0.1 (RustFS's S3 API).
type ExtraPort struct {
	Service string `json:"service"`
	Port    int    `json:"port"`
	Label   string `json:"label"`
}

// Service is one container.
type Service struct {
	Image  string `json:"image"`
	Digest string `json:"digest,omitempty"` // multi-arch index digest
	// User: "" (the image's), "site" (the website's Linux user) or "uid:gid".
	User       string            `json:"user,omitempty"`
	MemoryMB   int               `json:"memory_mb"`
	ShmMB      int               `json:"shm_mb,omitempty"`
	Entrypoint []string          `json:"entrypoint,omitempty"`
	Command    []string          `json:"command,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	// EnvBootstrap is only set for the first start (initial passwords). A
	// variable that is also in Env takes this value on the first start and
	// the Env value afterwards (sign-ups open only while the panel creates
	// the first account).
	EnvBootstrap map[string]string `json:"env_bootstrap,omitempty"`
	// SecretFiles mounts secrets as read-only files: container path ->
	// secret name.
	SecretFiles map[string]string `json:"secret_files,omitempty"`
	Volumes     []Volume          `json:"volumes,omitempty"`
	Files       []File            `json:"files,omitempty"`
	Health      []string          `json:"health,omitempty"` // command for Compose's healthcheck
	DependsOn   []string          `json:"depends_on,omitempty"`
}

// Volume is a folder of the app's data folder mounted into a container.
type Volume struct {
	Source   string `json:"source"` // folder name under the app's data folder
	Target   string `json:"target"`
	Owner    string `json:"owner"` // "site" or "uid:gid"
	Mode     string `json:"mode,omitempty"`
	ReadOnly bool   `json:"read_only,omitempty"`
	// Site puts the folder in the website's own folder (<site root>/<source>)
	// instead of the app's data folder, owned by the site's user, so that
	// files can be added to it with SFTP or the file manager (a music or
	// video library). Only for apps with a website; the owner must be "site".
	Site bool `json:"site,omitempty"`
}

// File is a file written into a volume before the first start.
type File struct {
	Volume string `json:"volume"`
	Path   string `json:"path"`
	Mode   string `json:"mode"`
	Owner  string `json:"owner"`
	Text   string `json:"text"`
}

// SecretSpec is a value the panel generates at install.
type SecretSpec struct {
	// Gen: "random" (Chars, Length), "hex" (Length characters),
	// "bcrypt" or "argon2id" (a hash of the secret From).
	Gen    string `json:"gen"`
	Chars  string `json:"chars,omitempty"` // "alnum" (default) or "lower-alnum"
	Length int    `json:"length,omitempty"`
	From   string `json:"from,omitempty"`
	Cost   int    `json:"cost,omitempty"`
	Show   bool   `json:"show,omitempty"` // shown with the credentials
	Label  Text   `json:"label,omitempty"`
}

// Input is a value the user enters on the install form.
type Input struct {
	Type     string `json:"type"` // "port", "regex", "country", "currency"
	Default  string `json:"default,omitempty"`
	Pattern  string `json:"pattern,omitempty"`
	Optional bool   `json:"optional,omitempty"`
	Label    Text   `json:"label,omitempty"`
}

// Step is one set-up step: exactly one of the fields is set.
type Step struct {
	HTTP *HTTPStep `json:"http,omitempty"`
	Exec *ExecStep `json:"exec,omitempty"` // in the running container
	Run  *ExecStep `json:"run,omitempty"`  // in a one-off container of the service
}

// HTTPStep sends a request to the app on 127.0.0.1.
type HTTPStep struct {
	Method string            `json:"method"` // GET, POST, PUT or PATCH
	Path   string            `json:"path"`
	Form   map[string]string `json:"form,omitempty"`
	// JSON is the request body: placeholders are filled in its strings, at
	// any depth; numbers, booleans and nulls are sent as they are.
	JSON map[string]any `json:"json,omitempty"`
	// Headers are sent with the request ("Host" sets the Host header, for
	// apps that check it). Values may use placeholders.
	Headers     map[string]string `json:"headers,omitempty"`
	KeepCookies bool              `json:"keep_cookies,omitempty"`
	Expect      []int             `json:"expect"`
	// Save keeps strings from the JSON answer for the next steps, as
	// ${step.NAME}: name -> dotted path in the answer ("accessToken",
	// "data.token"). They are kept in memory for this set-up only.
	Save map[string]string `json:"save,omitempty"`
}

// ExecStep runs a program in a container; Stdin carries secrets.
type ExecStep struct {
	Service      string   `json:"service"`
	Argv         []string `json:"argv"`
	Stdin        string   `json:"stdin,omitempty"`
	ExpectStdout string   `json:"expect_stdout,omitempty"`
	TimeoutS     int      `json:"timeout_s,omitempty"`
}

// BackupSpec says how to back up the app's data consistently.
type BackupSpec struct {
	// Consistency: "pause" (freeze the containers while copying), "dump"
	// (database dumps; the volumes in Exclude are not copied) or "none".
	Consistency string   `json:"consistency"`
	Dumps       []Dump   `json:"dumps,omitempty"`
	Exclude     []string `json:"exclude,omitempty"`
}

// Dump writes a database dump to stdout; RestoreArgv reads it on stdin.
type Dump struct {
	Service     string   `json:"service"`
	Argv        []string `json:"argv"`
	File        string   `json:"file"`
	RestoreArgv []string `json:"restore_argv"`
}

// Legacy describes installs made before the catalog (docker run, data in
// <site root>/data), which the first update converts.
type Legacy struct {
	Volume string   `json:"volume"` // receives <site root>/data
	Images []string `json:"images"` // image names used then
}

// ---- PHP apps ----

// PHPApp is an app installed into a PHP website by a Go installer.
type PHPApp struct {
	Installer  string            `json:"installer"`
	MinPHP     string            `json:"min_php"`
	MaxPHP     string            `json:"max_php,omitempty"`
	Download   *Download         `json:"download,omitempty"`
	Variants   []Variant         `json:"variants,omitempty"` // first match wins
	Plugin     *Download         `json:"plugin,omitempty"`   // WooCommerce
	Composer   *ComposerProject  `json:"composer,omitempty"`
	Extensions []string          `json:"extensions,omitempty"`
	INI        map[string]string `json:"ini,omitempty"`
	MemoryMB   int               `json:"memory_mb,omitempty"`
	UploadMB   int               `json:"upload_mb,omitempty"`
	DBPrefix   string            `json:"db_prefix"`
	Rewrite    string            `json:"rewrite"`
	Admin      bool              `json:"admin"` // asks for an administrator account
	Inputs     map[string]*Input `json:"inputs,omitempty"`
	Cron       *Cron             `json:"cron,omitempty"`
}

// Download is an archive with its pinned SHA-256.
type Download struct {
	Version string `json:"version,omitempty"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
	Format  string `json:"format"` // tgz, tbz2, zip
	Strip   string `json:"strip,omitempty"`
}

// Variant is a version of a PHP app picked by the server's MariaDB.
type Variant struct {
	Version    string    `json:"version"`
	MinMariaDB string    `json:"min_mariadb"`
	Download   *Download `json:"download"`
}

// ComposerProject is a Composer create-project.
type ComposerProject struct {
	Package    string `json:"package"`
	Constraint string `json:"constraint,omitempty"`
}

// Cron is a job the installer adds, run as the site's user.
type Cron struct {
	Schedule string `json:"schedule"`
	// Script is relative to the site's folder.
	Script string   `json:"script"`
	PHPArg []string `json:"php_args,omitempty"`
}

// ---- loading ----

var (
	mu      sync.RWMutex
	current *Catalog
	source  = "embedded"
	skipped []Skipped // apps of the remote catalog in use that this panel left out
)

// Embedded returns the catalog built into the binary.
func Embedded() *Catalog {
	c, err := Parse(embedded)
	if err != nil {
		panic("appstore: embedded catalog.json is invalid: " + err.Error())
	}
	return c
}

// Current returns the catalog in use: the embedded one, or a newer signed
// remote one.
func Current() *Catalog {
	mu.RLock()
	c := current
	mu.RUnlock()
	if c != nil {
		return c
	}
	mu.Lock()
	defer mu.Unlock()
	if current == nil {
		current = Embedded()
	}
	return current
}

// Source says where the current catalog came from: "embedded" or "remote".
func Source() string {
	mu.RLock()
	defer mu.RUnlock()
	return source
}

// SkippedApps lists the entries of the remote catalog in use that this
// panel left out (see ParseRemote).
func SkippedApps() []Skipped {
	mu.RLock()
	defer mu.RUnlock()
	return slices.Clone(skipped)
}

func setCurrent(c *Catalog, src string, sk []Skipped) {
	mu.Lock()
	current, source, skipped = c, src, sk
	mu.Unlock()
}

// Parse reads and validates a catalog.
func Parse(data []byte) (*Catalog, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var c Catalog
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("read catalog: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Skipped is an entry of a downloaded catalog this panel left out.
type Skipped struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// ParseRemote reads a downloaded catalog. Unlike Parse, one entry this
// panel cannot use does not refuse the whole file: an entry with a field or
// a recipe feature this panel does not know (newer panels add them), a
// min_panel above this panel's version, an installer it does not have, or
// any other failed check is left out and listed in skipped. If the
// embedded catalog has an app with the same ID, that entry is kept, so the
// app stays installable at the version this panel knows and installed
// copies keep working. The file itself is refused only when its frame is
// wrong (format, version, no usable app).
func ParseRemote(data []byte, panelVersion string) (*Catalog, []Skipped, error) {
	var frame struct {
		Schema         int               `json:"schema"`
		CatalogVersion int64             `json:"catalog_version"`
		Generated      string            `json:"generated"`
		MinPanel       string            `json:"min_panel"`
		Apps           []json.RawMessage `json:"apps"`
	}
	if err := json.Unmarshal(data, &frame); err != nil {
		return nil, nil, fmt.Errorf("read catalog: %w", err)
	}
	if frame.Schema != Schema {
		return nil, nil, fmt.Errorf("catalog schema %d is not supported (this panel reads %d)", frame.Schema, Schema)
	}
	if frame.CatalogVersion <= 0 {
		return nil, nil, errors.New("catalog_version missing")
	}
	c := &Catalog{Schema: frame.Schema, CatalogVersion: frame.CatalogVersion, Generated: frame.Generated, MinPanel: frame.MinPanel}
	var skipped []Skipped
	seen := map[string]bool{}
	skip := func(id, reason string) {
		skipped = append(skipped, Skipped{ID: id, Reason: reason})
	}
	for _, raw := range frame.Apps {
		var head struct {
			ID string `json:"id"`
		}
		json.Unmarshal(raw, &head)
		if !idRe.MatchString(head.ID) {
			head.ID = "?"
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		var a App
		if err := dec.Decode(&a); err != nil {
			skip(head.ID, "uses a field this panel does not know: "+err.Error())
			continue
		}
		if seen[a.ID] {
			skip(a.ID, "listed twice")
			continue
		}
		if a.MinPanel != "" && panelVersion != "dev" && !versionAtLeast(panelVersion, a.MinPanel) {
			skip(a.ID, "needs panel "+a.MinPanel+" or newer")
			continue
		}
		if err := a.validate(); err != nil {
			skip(a.ID, err.Error())
			continue
		}
		seen[a.ID] = true
		c.Apps = append(c.Apps, &a)
	}
	// Keep what this panel knows of the apps it had to leave out.
	emb := Embedded()
	for _, s := range skipped {
		if s.ID == "?" || seen[s.ID] {
			continue
		}
		if a, ok := emb.App(s.ID); ok {
			seen[s.ID] = true
			c.Apps = append(c.Apps, a)
		}
	}
	if len(c.Apps) == 0 {
		return nil, skipped, errors.New("the catalog has no app this panel can use")
	}
	return c, skipped, nil
}

// App returns the app with an ID.
func (c *Catalog) App(id string) (*App, bool) {
	i := slices.IndexFunc(c.Apps, func(a *App) bool { return a.ID == id })
	if i < 0 {
		return nil, false
	}
	return c.Apps[i], true
}

// Lookup returns an app of the current catalog.
func Lookup(id string) (*App, bool) { return Current().App(id) }

// ByInstaller returns the PHP app with an installer ID.
func (c *Catalog) ByInstaller(id string) (*App, bool) {
	i := slices.IndexFunc(c.Apps, func(a *App) bool { return a.PHP != nil && a.PHP.Installer == id })
	if i < 0 {
		return nil, false
	}
	return c.Apps[i], true
}

// ImageRef returns the reference the panel pulls: the tag, pinned to its
// digest when the catalog has one.
func (s *Service) ImageRef() string {
	if s.Digest != "" {
		return s.Image + "@" + s.Digest
	}
	return s.Image
}

// MainService is the service the website (or the service port) points at.
func (r *Recipe) MainService() string {
	switch {
	case r.Web != nil:
		return r.Web.Service
	case r.Service != nil:
		return r.Service.Service
	}
	return ""
}

// HasBootstrap reports whether any service has first-start-only variables.
func (r *Recipe) HasBootstrap() bool {
	for _, s := range r.Services {
		if len(s.EnvBootstrap) > 0 {
			return true
		}
	}
	return false
}

// PickDownload returns the PHP app's download for a MariaDB version: the
// first variant the server can run ("" when unknown picks the last one,
// which needs the least).
func (p *PHPApp) PickDownload(mariadb string) (*Download, string, bool) {
	if len(p.Variants) == 0 {
		if p.Download == nil {
			return nil, "", false
		}
		return p.Download, p.Download.Version, true
	}
	if mariadb == "" {
		v := p.Variants[len(p.Variants)-1]
		return v.Download, v.Version, true
	}
	for _, v := range p.Variants {
		if versionAtLeast(mariadb, v.MinMariaDB) {
			return v.Download, v.Version, true
		}
	}
	return nil, "", false
}
