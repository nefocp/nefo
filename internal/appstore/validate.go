package appstore

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Rules every catalog entry must follow. The embedded catalog is checked by
// a test; a remote catalog is refused as a whole if one entry fails.

var (
	idRe       = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,31}$`)
	logoRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)
	svcRe      = regexp.MustCompile(`^[a-z][a-z0-9-]{0,19}$`)
	volRe      = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
	envKeyRe   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]{0,79}$`)
	secretRe   = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
	ownerRe    = regexp.MustCompile(`^(site|[0-9]{1,6}:[0-9]{1,6})$`)
	modeRe     = regexp.MustCompile(`^0[0-7]{3}$`)
	digestRe   = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	sha256Re   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	phpVerRe   = regexp.MustCompile(`^[5-9]\.[0-9]{1,2}$`)
	versionRe  = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+_ ()-]{0,39}$`)
	cronRe     = regexp.MustCompile(`^[0-9*/,-]{1,20}( [0-9*/,-]{1,20}){4}$`)
	scriptRe   = regexp.MustCompile(`^[A-Za-z0-9_./-]{1,200}$`)
	phpArgRe   = regexp.MustCompile(`^-d$|^[a-z_.]{1,40}=[0-9A-Za-z]{1,20}$`)
	iniKeyRe   = regexp.MustCompile(`^[a-z_.]{1,40}$`)
	iniValueRe = regexp.MustCompile(`^[0-9A-Za-z]{1,20}$`)
	// image: optional registry host, a path, an exact tag.
	imageRe = regexp.MustCompile(`^([a-z0-9.-]+\.[a-z]{2,}/)?[a-z0-9]+([._/-][a-z0-9]+)*:[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	// floating tags: latest, stable, main… and a bare major version.
	floatingRe = regexp.MustCompile(`:(latest|stable|main|master|edge|beta|nightly|dev|rolling|lts|v?[0-9]+)$`)
	// placeholders: ${name}, ${secret.name}, ${input.name}, ${step.name}
	placeholderRe = regexp.MustCompile(`\$\{([a-z_]+)(?:\.([a-z][a-z0-9_]*))?\}`)
	headerRe      = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]{0,63}$`)
	jsonPathRe    = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}(\.[A-Za-z0-9_]{1,64}){0,7}$`)
	panelVerRe    = regexp.MustCompile(`^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$`)
)

// Vars are the plain placeholders a recipe may use.
var Vars = []string{"domain", "url", "scheme", "https", "port", "public_host", "site_uid", "site_gid", "tz", "admin_email"}

var (
	categories = []string{"cms", "ecommerce", "education", "ai", "security", "storage", "database", "devtools",
		"productivity", "analytics", "network", "business", "monitoring", "automation", "media", "communication"}
	// installers compiled into this binary (internal/hosting).
	installers = []string{"wordpress", "joomla", "nextcloud", "drupal", "laravel", "woocommerce", "opencart", "moodle"}
	// shared networks the panel creates.
	sharedNetworks = []string{"panel-ai", "panel-services"}
	adminMethods   = []string{"form", "generated", "bootstrap_env", "invite", "logs", "signup", "site_password", "none"}
)

// Installers returns the PHP installers this binary has.
func Installers() []string { return slices.Clone(installers) }

// Validate checks the whole catalog.
func (c *Catalog) Validate() error {
	if c.Schema != Schema {
		return fmt.Errorf("catalog schema %d is not supported (this panel reads %d)", c.Schema, Schema)
	}
	if c.CatalogVersion <= 0 {
		return errors.New("catalog_version missing")
	}
	if len(c.Apps) == 0 {
		return errors.New("the catalog has no apps")
	}
	seen := map[string]bool{}
	for _, a := range c.Apps {
		if a == nil {
			return errors.New("empty catalog entry")
		}
		if seen[a.ID] {
			return fmt.Errorf("app %q listed twice", a.ID)
		}
		seen[a.ID] = true
		if err := a.validate(); err != nil {
			return fmt.Errorf("app %s: %w", a.ID, err)
		}
	}
	return nil
}

// textOK: a catalog text needs English and Vietnamese, and may have other
// languages (the interface falls back to English; a language this panel
// doesn't speak is simply not shown).
func textOK(t Text, max int) bool {
	for _, lang := range []string{"en", "vi"} {
		if v := t[lang]; strings.TrimSpace(v) == "" || len(v) > max {
			return false
		}
	}
	for _, v := range t {
		// Chinese takes 3 bytes a character, Russian 2: len counts bytes.
		if len(v) > 3*max || strings.ContainsAny(v, "\x00") {
			return false
		}
	}
	return true
}

func optionalText(t Text, max int) bool { return t == nil || textOK(t, max) }

func (a *App) validate() error {
	switch {
	case !idRe.MatchString(a.ID):
		return errors.New("invalid id")
	case a.Name == "" || len(a.Name) > 60:
		return errors.New("invalid name")
	case !slices.Contains(categories, a.Category):
		return fmt.Errorf("unknown category %q", a.Category)
	case !versionRe.MatchString(a.Version):
		return fmt.Errorf("invalid version %q", a.Version)
	case !textOK(a.Description, 400):
		return errors.New("description needs en and vi")
	case !optionalText(a.Notes, 600):
		return errors.New("notes need en and vi")
	case a.License == "" || len(a.License) > 120:
		return errors.New("licence missing")
	case !strings.HasPrefix(a.Homepage, "https://"):
		return errors.New("homepage must be https")
	case !logoRe.MatchString(a.Logo):
		return errors.New("invalid logo key")
	case !LogoExists(a.Logo):
		return fmt.Errorf("no logo %q", a.Logo)
	case a.Resources.MinRAMMB <= 0 || a.Resources.RecommendedRAMMB < a.Resources.MinRAMMB:
		return errors.New("resources: min_ram_mb and recommended_ram_mb needed")
	case !slices.Contains(adminMethods, a.Admin.Method):
		return fmt.Errorf("unknown admin method %q", a.Admin.Method)
	case !optionalText(a.Admin.Note, 400) || !optionalText(a.Updates.Note, 400):
		return errors.New("admin/update notes need en and vi")
	case a.Updates.Breaking != "" && a.Updates.Breaking != "major" && a.Updates.Breaking != "minor":
		return errors.New("updates.breaking must be major or minor")
	case a.MinPanel != "" && !panelVerRe.MatchString(a.MinPanel):
		return errors.New("min_panel must be a version such as 0.24.0")
	}
	if a.Admin.LogRegex != "" {
		re, err := regexp.Compile(a.Admin.LogRegex)
		if err != nil || re.NumSubexp() != 1 {
			return errors.New("admin.log_regex must compile with one group")
		}
	}
	for arch, flags := range a.Requires.CPUFlags {
		if arch != "amd64" && arch != "arm64" {
			return fmt.Errorf("cpu_flags: unknown architecture %q", arch)
		}
		for _, f := range flags {
			if !regexp.MustCompile(`^[a-z0-9_]{1,20}$`).MatchString(f) {
				return fmt.Errorf("cpu_flags: invalid flag %q", f)
			}
		}
	}
	switch a.Kind {
	case KindPHP:
		if a.PHP == nil || a.Docker != nil {
			return errors.New("a php app needs php and no docker")
		}
		return a.PHP.validate()
	case KindDocker, KindService:
		if a.Docker == nil || a.PHP != nil {
			return errors.New("a docker app needs docker and no php")
		}
		return a.Docker.validate(a)
	}
	return fmt.Errorf("unknown kind %q", a.Kind)
}

// ---- Docker recipes ----

func (r *Recipe) validate(a *App) error {
	if len(r.Services) == 0 || len(r.Services) > 4 {
		return errors.New("1 to 4 services")
	}
	if a.Kind == KindDocker && (r.Web == nil || r.Service != nil) {
		return errors.New("a docker app needs web and no service port")
	}
	if a.Kind == KindService && (r.Service == nil || r.Web != nil) {
		return errors.New("a service app needs a service port and no web")
	}
	main := r.MainService()
	if _, ok := r.Services[main]; !ok {
		return fmt.Errorf("main service %q missing", main)
	}
	if r.Web != nil {
		if !validPort(r.Web.Port) || !strings.HasPrefix(r.Web.Health, "/") || r.Web.UploadMB < 0 || r.Web.UploadMB > 10240 {
			return errors.New("invalid web")
		}
	}
	if r.Service != nil {
		if !validPort(r.Service.Port) || (r.Service.PreferPort != 0 && !validPort(r.Service.PreferPort)) ||
			(r.Service.Alias != "" && !svcRe.MatchString(r.Service.Alias)) || (r.Service.Health != "" && !strings.HasPrefix(r.Service.Health, "/")) {
			return errors.New("invalid service port")
		}
	}
	for _, p := range r.ExtraPorts {
		if _, ok := r.Services[p.Service]; !ok || !validPort(p.Port) || p.Label == "" {
			return errors.New("invalid extra port")
		}
	}
	for _, n := range r.Networks {
		if !slices.Contains(sharedNetworks, n) {
			return fmt.Errorf("unknown shared network %q", n)
		}
	}
	for name, s := range r.Secrets {
		if !secretRe.MatchString(name) || s == nil {
			return fmt.Errorf("invalid secret %q", name)
		}
		switch s.Gen {
		case "random", "hex":
			if s.Length < 8 || s.Length > 128 || (s.Chars != "" && s.Chars != "alnum" && s.Chars != "lower-alnum") {
				return fmt.Errorf("secret %s: invalid length or chars", name)
			}
		case "bcrypt", "argon2id":
			src, ok := r.Secrets[s.From]
			if !ok || (src.Gen != "random" && src.Gen != "hex") {
				return fmt.Errorf("secret %s: from must name a random secret", name)
			}
		default:
			return fmt.Errorf("secret %s: unknown gen %q", name, s.Gen)
		}
		if !optionalText(s.Label, 80) {
			return fmt.Errorf("secret %s: label needs en and vi", name)
		}
	}
	for name, in := range r.Inputs {
		if err := in.validate(name); err != nil {
			return err
		}
	}
	for name, s := range r.Services {
		if !svcRe.MatchString(name) {
			return fmt.Errorf("invalid service name %q", name)
		}
		if err := s.validate(r); err != nil {
			return fmt.Errorf("service %s: %w", name, err)
		}
	}
	saved := map[string]bool{} // ${step.*} values saved by the steps so far
	for _, list := range [][]Step{r.BeforeStart, r.AfterStart} {
		for _, st := range list {
			if err := st.validate(r, saved); err != nil {
				return err
			}
		}
	}
	for name, s := range r.Services {
		for _, v := range s.Volumes {
			// The site's user can replace the folder with a link, which
			// Docker would follow: only a container that runs as that same
			// user may mount it.
			if v.Site && (a.Kind != KindDocker || v.Owner != "site" || s.User != "site") {
				return fmt.Errorf("service %s: a site volume needs an app with a website, owner site and user site", name)
			}
		}
	}
	switch r.Backup.Consistency {
	case "pause", "dump", "none":
	default:
		return errors.New("backup.consistency must be pause, dump or none")
	}
	for _, d := range r.Backup.Dumps {
		if _, ok := r.Services[d.Service]; !ok || len(d.Argv) == 0 || len(d.RestoreArgv) == 0 ||
			!regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`).MatchString(d.File) {
			return errors.New("invalid backup dump")
		}
	}
	if r.Backup.Consistency == "dump" && len(r.Backup.Dumps) == 0 {
		return errors.New("backup.consistency dump without dumps")
	}
	if r.Legacy != nil && (!r.hasVolume(r.Legacy.Volume) || len(r.Legacy.Images) == 0) {
		return errors.New("invalid legacy")
	}
	// Every placeholder must be known.
	return r.checkPlaceholders(a)
}

func (r *Recipe) hasVolume(name string) bool {
	for _, s := range r.Services {
		for _, v := range s.Volumes {
			if v.Source == name {
				return true
			}
		}
	}
	return false
}

func validPort(p int) bool { return p > 0 && p < 65536 }

func (s *Service) validate(r *Recipe) error {
	switch {
	case !imageRe.MatchString(s.Image):
		return fmt.Errorf("image %q must have an exact tag", s.Image)
	case floatingRe.MatchString(s.Image):
		return fmt.Errorf("image %q has a floating tag", s.Image)
	case s.Digest != "" && !digestRe.MatchString(s.Digest):
		return errors.New("invalid digest")
	case s.User != "" && s.User != "site" && !ownerRe.MatchString(s.User):
		return fmt.Errorf("invalid user %q", s.User)
	case s.MemoryMB < 32 || s.MemoryMB > 65536:
		return errors.New("memory_mb must be 32 to 65536")
	case s.ShmMB < 0 || s.ShmMB > 2048:
		return errors.New("invalid shm_mb")
	}
	for _, list := range [][]string{s.Entrypoint, s.Command} {
		for _, v := range list {
			if strings.Contains(v, "${secret.") {
				return errors.New("a secret may not be on a command line")
			}
			if strings.ContainsAny(v, "\x00\n") {
				return errors.New("invalid command")
			}
		}
	}
	for _, env := range []map[string]string{s.Env, s.EnvBootstrap} {
		for k, v := range env {
			if !envKeyRe.MatchString(k) || strings.ContainsAny(v, "\x00\n") || len(v) > 4096 {
				return fmt.Errorf("invalid variable %q", k)
			}
		}
	}
	for p, name := range s.SecretFiles {
		if !absPath(p) {
			return fmt.Errorf("invalid secret file path %q", p)
		}
		if _, ok := r.Secrets[name]; !ok {
			return fmt.Errorf("secret file %s: unknown secret %q", p, name)
		}
	}
	vols := map[string]bool{}
	for _, v := range s.Volumes {
		if !volRe.MatchString(v.Source) || !absPath(v.Target) || !ownerRe.MatchString(v.Owner) ||
			(v.Mode != "" && !modeRe.MatchString(v.Mode)) {
			return fmt.Errorf("invalid volume %q", v.Source)
		}
		vols[v.Source] = true
	}
	for _, f := range s.Files {
		clean := path.Clean(f.Path)
		if !vols[f.Volume] || clean != f.Path || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "..") ||
			!modeRe.MatchString(f.Mode) || !ownerRe.MatchString(f.Owner) || f.Text == "" || len(f.Text) > 32<<10 {
			return fmt.Errorf("invalid file %q", f.Path)
		}
	}
	for _, h := range s.Health {
		if strings.ContainsAny(h, "\x00\n") || strings.Contains(h, "${secret.") {
			return errors.New("invalid health command")
		}
	}
	for _, d := range s.DependsOn {
		if _, ok := r.Services[d]; !ok {
			return fmt.Errorf("depends_on: unknown service %q", d)
		}
	}
	return nil
}

func absPath(p string) bool {
	return strings.HasPrefix(p, "/") && path.Clean(p) == p && p != "/" && !strings.ContainsAny(p, "\x00\n:")
}

func (in *Input) validate(name string) error {
	if !secretRe.MatchString(name) || in == nil {
		return fmt.Errorf("invalid input %q", name)
	}
	switch in.Type {
	case "port", "country", "currency":
	case "regex":
		if in.Pattern == "" || !strings.HasPrefix(in.Pattern, "^") || !strings.HasSuffix(in.Pattern, "$") {
			return fmt.Errorf("input %s: pattern must be anchored", name)
		}
		if _, err := regexp.Compile(in.Pattern); err != nil {
			return fmt.Errorf("input %s: %w", name, err)
		}
	default:
		return fmt.Errorf("input %s: unknown type %q", name, in.Type)
	}
	if !optionalText(in.Label, 120) {
		return fmt.Errorf("input %s: label needs en and vi", name)
	}
	return nil
}

func (st Step) validate(r *Recipe, saved map[string]bool) error {
	n := 0
	for _, set := range []bool{st.HTTP != nil, st.Exec != nil, st.Run != nil} {
		if set {
			n++
		}
	}
	if n != 1 {
		return errors.New("a step needs exactly one of http, exec, run")
	}
	if h := st.HTTP; h != nil {
		if !slices.Contains([]string{"GET", "POST", "PUT", "PATCH"}, h.Method) || !strings.HasPrefix(h.Path, "/") ||
			strings.ContainsAny(h.Path, " \n\x00") || len(h.Expect) == 0 || (h.Form != nil && h.JSON != nil) {
			return errors.New("invalid http step")
		}
		for k, v := range h.Headers {
			if !headerRe.MatchString(k) || strings.ContainsAny(v, "\r\n\x00") || len(v) > 4096 {
				return fmt.Errorf("http step: invalid header %q", k)
			}
		}
		if err := checkJSONValue(h.JSON, 0); err != nil {
			return err
		}
		// ${step.*} must come from an earlier step.
		var bad error
		h.each(func(v string) {
			for _, m := range placeholderRe.FindAllStringSubmatch(v, -1) {
				if m[1] == "step" && !saved[m[2]] && bad == nil {
					bad = fmt.Errorf("http step: ${step.%s} is not saved by an earlier step", m[2])
				}
			}
		})
		if bad != nil {
			return bad
		}
		for name, p := range h.Save {
			if !secretRe.MatchString(name) || !jsonPathRe.MatchString(p) {
				return fmt.Errorf("http step: invalid save %q", name)
			}
			saved[name] = true
		}
		return nil
	}
	e := st.Exec
	if e == nil {
		e = st.Run
	}
	if _, ok := r.Services[e.Service]; !ok || len(e.Argv) == 0 || e.TimeoutS < 0 || e.TimeoutS > 3600 {
		return errors.New("invalid exec step")
	}
	for _, a := range e.Argv {
		if strings.Contains(a, "${secret.") || strings.ContainsAny(a, "\x00\n") {
			return errors.New("a secret may not be on a command line")
		}
	}
	return nil
}

// checkJSONValue accepts what encoding/json makes of a JSON body: strings,
// numbers, booleans, null, and lists and objects of them, not too deep.
func checkJSONValue(v any, depth int) error {
	if depth > 8 {
		return errors.New("http step: json nested too deep")
	}
	switch x := v.(type) {
	case nil, string, float64, bool:
	case map[string]any:
		for _, e := range x {
			if err := checkJSONValue(e, depth+1); err != nil {
				return err
			}
		}
	case []any:
		for _, e := range x {
			if err := checkJSONValue(e, depth+1); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("http step: unexpected json value %T", v)
	}
	return nil
}

// each calls f with every string of the step that may hold placeholders.
func (h *HTTPStep) each(f func(string)) {
	f(h.Path)
	for _, v := range h.Form {
		f(v)
	}
	for _, v := range h.Headers {
		f(v)
	}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case string:
			f(x)
		case map[string]any:
			for _, e := range x {
				walk(e)
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	if h.JSON != nil {
		walk(h.JSON)
	}
}

// checkPlaceholders makes sure every ${…} in the recipe names a known
// variable, secret or input (${step.*}: a value an earlier HTTP step saved,
// checked with the steps).
func (r *Recipe) checkPlaceholders(a *App) error {
	var bad error
	inStep := false
	check := func(s string) {
		if bad != nil {
			return
		}
		rest := s
		for _, m := range placeholderRe.FindAllStringSubmatch(s, -1) {
			rest = strings.Replace(rest, m[0], "", 1)
			switch m[1] {
			case "secret":
				if _, ok := r.Secrets[m[2]]; !ok {
					bad = fmt.Errorf("unknown secret %q", m[2])
				}
			case "input":
				if _, ok := r.Inputs[m[2]]; !ok {
					bad = fmt.Errorf("unknown input %q", m[2])
				}
			case "step":
				if !inStep {
					bad = fmt.Errorf("${step.%s} outside an http step", m[2])
				}
			default:
				if m[2] != "" || !slices.Contains(Vars, m[1]) {
					bad = fmt.Errorf("unknown placeholder %q", m[0])
				}
			}
		}
		if strings.Contains(rest, "${") {
			bad = fmt.Errorf("malformed placeholder in %q", s)
		}
	}
	for _, s := range r.Services {
		for _, v := range s.Env {
			check(v)
		}
		for _, v := range s.EnvBootstrap {
			check(v)
		}
		for _, v := range s.Command {
			check(v)
		}
		for _, v := range s.Entrypoint {
			check(v)
		}
		for _, f := range s.Files {
			check(f.Text)
		}
	}
	for _, list := range [][]Step{r.BeforeStart, r.AfterStart} {
		for _, st := range list {
			switch {
			case st.HTTP != nil:
				inStep = true
				st.HTTP.each(check)
				inStep = false
			case st.Exec != nil:
				check(st.Exec.Stdin)
				for _, v := range st.Exec.Argv {
					check(v)
				}
			case st.Run != nil:
				check(st.Run.Stdin)
				for _, v := range st.Run.Argv {
					check(v)
				}
			}
		}
	}
	check(a.Admin.Connection)
	return bad
}

// ---- PHP apps ----

func (p *PHPApp) validate() error {
	switch {
	case !slices.Contains(installers, p.Installer):
		return fmt.Errorf("installer %q is not in this panel", p.Installer)
	case !phpVerRe.MatchString(p.MinPHP) || (p.MaxPHP != "" && !phpVerRe.MatchString(p.MaxPHP)):
		return errors.New("invalid min_php/max_php")
	case !regexp.MustCompile(`^[a-z]{2,4}$`).MatchString(p.DBPrefix):
		return errors.New("invalid db_prefix")
	case p.MemoryMB < 0 || p.MemoryMB > 4096 || p.UploadMB < 0 || p.UploadMB > 10240:
		return errors.New("invalid memory/upload")
	}
	for _, d := range []*Download{p.Download, p.Plugin} {
		if d != nil {
			if err := d.validate(); err != nil {
				return err
			}
		}
	}
	for _, v := range p.Variants {
		if !versionRe.MatchString(v.Version) || !regexp.MustCompile(`^[0-9]+\.[0-9]+(\.[0-9]+)?$`).MatchString(v.MinMariaDB) || v.Download == nil {
			return errors.New("invalid variant")
		}
		if err := v.Download.validate(); err != nil {
			return err
		}
	}
	for k, v := range p.INI {
		if !iniKeyRe.MatchString(k) || !iniValueRe.MatchString(v) {
			return fmt.Errorf("invalid ini %s", k)
		}
	}
	for name, in := range p.Inputs {
		if err := in.validate(name); err != nil {
			return err
		}
	}
	if c := p.Cron; c != nil {
		if !cronRe.MatchString(c.Schedule) || !scriptRe.MatchString(c.Script) || strings.Contains(c.Script, "..") {
			return errors.New("invalid cron")
		}
		for _, a := range c.PHPArg {
			if !phpArgRe.MatchString(a) {
				return fmt.Errorf("invalid cron php arg %q", a)
			}
		}
	}
	return nil
}

func (d *Download) validate() error {
	switch {
	case !strings.HasPrefix(d.URL, "https://") || strings.ContainsAny(d.URL, " \n\x00"):
		return errors.New("downloads must be https")
	case !sha256Re.MatchString(d.SHA256):
		return errors.New("download needs a sha256")
	case d.Format != "tgz" && d.Format != "tbz2" && d.Format != "zip":
		return fmt.Errorf("unknown format %q", d.Format)
	case d.Strip != "" && (!strings.HasSuffix(d.Strip, "/") || strings.Contains(d.Strip, "..")):
		return errors.New("strip must be a folder prefix")
	}
	return nil
}

// versionAtLeast compares dotted versions numerically ("10.11.14-MariaDB"
// counts as 10.11.14).
func versionAtLeast(have, want string) bool {
	h, w := versionParts(have), versionParts(want)
	for i := 0; i < max(len(h), len(w)); i++ {
		var a, b int
		if i < len(h) {
			a = h[i]
		}
		if i < len(w) {
			b = w[i]
		}
		if a != b {
			return a > b
		}
	}
	return true
}

// VersionAtLeast is versionAtLeast for other packages.
func VersionAtLeast(have, want string) bool { return versionAtLeast(have, want) }

func versionParts(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexFunc(v, func(r rune) bool { return (r < '0' || r > '9') && r != '.' }); i >= 0 {
		v = v[:i]
	}
	var out []int
	for _, p := range strings.Split(v, ".") {
		n, err := strconv.Atoi(p)
		if err != nil {
			break
		}
		out = append(out, n)
	}
	return out
}
