package appstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Env holds the values a recipe's placeholders are filled with, and where
// one installed copy of the app (an instance) keeps its files.
type Env struct {
	Name       string // instance: the website's Linux user, or the service app's ID
	Domain     string
	HTTPS      bool
	Port       int         // host port of the website's service or the service port (127.0.0.1)
	ExtraPorts map[int]int // container port -> host port (127.0.0.1)
	PublicHost string
	SiteUID    int
	SiteGID    int
	TZ         string
	AdminEmail string
	Secrets    map[string]string
	Inputs     map[string]string
	ProjectDir string // /var/lib/panel-apps/<name>
	// Bootstrap includes the first-start-only variables.
	Bootstrap bool
	// MemoryMB overrides the memory limit of every service (memory_auto).
	MemoryMB int
}

// DataDir is where the instance's volumes are.
func (e *Env) DataDir() string { return path.Join(e.ProjectDir, "data") }

// SecretsDir holds the secrets mounted as files.
func (e *Env) SecretsDir() string { return path.Join(e.ProjectDir, "secrets") }

// ProjectName is the Compose project of an instance.
func ProjectName(name string) string { return "panelapp-" + name }

// ContainerName is the name of a service's container: the main service
// keeps the name panel-<name> that the panel always used.
func ContainerName(name, service string, main bool) string {
	if main {
		return "panel-" + name
	}
	return "panel-" + name + "-" + service
}

var (
	domainRe     = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]{0,251}[a-z0-9])?$`)
	emailRe      = regexp.MustCompile(`^[A-Za-z0-9._%+-]{1,64}@[A-Za-z0-9-]{1,63}(\.[A-Za-z0-9-]{1,63})*\.[A-Za-z]{2,24}$`)
	secretValRe  = regexp.MustCompile(`^[A-Za-z0-9./$=+,_-]{1,256}$`)
	inputValRe   = regexp.MustCompile(`^[A-Za-z0-9._:/-]{0,128}$`)
	tzRe         = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_+/-]{0,63}$`)
	errNoSiteVar = errors.New("this placeholder needs a website")
)

// ValidEmail checks an administrator's e-mail address. Some apps
// (Directus) refuse reserved top-level domains such as .test, so callers
// may check those too.
func ValidEmail(s string) bool { return emailRe.MatchString(s) }

// value returns a placeholder's value after checking its characters, so no
// value can break the text it is put into (a YAML or INI file, SQL).
func (e *Env) value(name, sub string) (string, error) {
	switch name {
	case "secret":
		v, ok := e.Secrets[sub]
		if !ok || !secretValRe.MatchString(v) {
			return "", fmt.Errorf("secret %s missing or invalid", sub)
		}
		return v, nil
	case "input":
		v, ok := e.Inputs[sub]
		if !ok || !inputValRe.MatchString(v) {
			return "", fmt.Errorf("input %s missing or invalid", sub)
		}
		return v, nil
	case "domain":
		if e.Domain == "" {
			return "", errNoSiteVar
		}
		if !domainRe.MatchString(e.Domain) {
			return "", errors.New("invalid domain")
		}
		return e.Domain, nil
	case "url":
		d, err := e.value("domain", "")
		if err != nil {
			return "", err
		}
		return e.scheme() + "://" + d, nil
	case "scheme":
		return e.scheme(), nil
	case "https":
		return strconv.FormatBool(e.HTTPS), nil
	case "port":
		return strconv.Itoa(e.Port), nil
	case "public_host":
		if !domainRe.MatchString(e.PublicHost) {
			return "", errors.New("invalid public host")
		}
		return e.PublicHost, nil
	case "site_uid":
		return strconv.Itoa(e.SiteUID), nil
	case "site_gid":
		return strconv.Itoa(e.SiteGID), nil
	case "tz":
		if e.TZ == "" {
			return "UTC", nil
		}
		if !tzRe.MatchString(e.TZ) {
			return "", errors.New("invalid time zone")
		}
		return e.TZ, nil
	case "admin_email":
		if !emailRe.MatchString(e.AdminEmail) {
			return "", errors.New("invalid administrator e-mail address")
		}
		return e.AdminEmail, nil
	}
	return "", fmt.Errorf("unknown placeholder %s", name)
}

func (e *Env) scheme() string {
	if e.HTTPS {
		return "https"
	}
	return "http"
}

// Expand fills the placeholders of s.
func (e *Env) Expand(s string) (string, error) {
	var err error
	out := placeholderRe.ReplaceAllStringFunc(s, func(m string) string {
		sm := placeholderRe.FindStringSubmatch(m)
		v, verr := e.value(sm[1], sm[2])
		if verr != nil && err == nil {
			err = verr
		}
		return v
	})
	if err != nil {
		return "", err
	}
	if strings.ContainsAny(out, "\x00\n") && !strings.Contains(s, "\n") {
		return "", errors.New("a value contains a line break")
	}
	return out, nil
}

// owner turns "site" or "uid:gid" into numbers.
func (e *Env) owner(s string) (int, int, error) {
	if s == "site" {
		if e.SiteUID <= 0 {
			return 0, 0, errNoSiteVar
		}
		return e.SiteUID, e.SiteGID, nil
	}
	a, b, ok := strings.Cut(s, ":")
	if !ok {
		return 0, 0, fmt.Errorf("invalid owner %q", s)
	}
	uid, err1 := strconv.Atoi(a)
	gid, err2 := strconv.Atoi(b)
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("invalid owner %q", s)
	}
	return uid, gid, nil
}

// ---- the folders and files of an instance ----

// Dir is a folder to create for a volume.
type Dir struct {
	Path     string
	UID, GID int
	Mode     uint32
}

// OutFile is a file to write once before the first start.
type OutFile struct {
	Path     string
	UID, GID int
	Mode     uint32
	Data     []byte
}

// Layout returns the volume folders and the files of an instance.
func Layout(app *App, e *Env) ([]Dir, []OutFile, error) {
	r := app.Docker
	seen := map[string]bool{}
	var dirs []Dir
	var files []OutFile
	for _, name := range sortedKeys(r.Services) {
		s := r.Services[name]
		for _, v := range s.Volumes {
			if seen[v.Source] {
				continue
			}
			seen[v.Source] = true
			uid, gid, err := e.owner(v.Owner)
			if err != nil {
				return nil, nil, err
			}
			mode := uint32(0o750)
			if v.Mode != "" {
				m, _ := strconv.ParseUint(v.Mode, 8, 32)
				mode = uint32(m)
			}
			dirs = append(dirs, Dir{Path: path.Join(e.DataDir(), v.Source), UID: uid, GID: gid, Mode: mode})
		}
		for _, f := range s.Files {
			text, err := e.Expand(f.Text)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", f.Path, err)
			}
			uid, gid, err := e.owner(f.Owner)
			if err != nil {
				return nil, nil, err
			}
			m, _ := strconv.ParseUint(f.Mode, 8, 32)
			files = append(files, OutFile{Path: path.Join(e.DataDir(), f.Volume, f.Path),
				UID: uid, GID: gid, Mode: uint32(m), Data: []byte(text)})
		}
	}
	return dirs, files, nil
}

// SecretFiles returns the secrets to write as files (root, 0400), by name.
func SecretFiles(app *App) []string {
	var out []string
	for _, s := range app.Docker.Services {
		for _, name := range s.SecretFiles {
			if !slices.Contains(out, name) {
				out = append(out, name)
			}
		}
	}
	slices.Sort(out)
	return out
}

// ---- the Compose file ----

type composeFile struct {
	Name     string                     `json:"name"`
	Services map[string]*composeService `json:"services"`
	Networks map[string]*composeNetwork `json:"networks,omitempty"`
}

type composeService struct {
	Image         string                        `json:"image"`
	ContainerName string                        `json:"container_name"`
	Restart       string                        `json:"restart"`
	User          string                        `json:"user,omitempty"`
	Entrypoint    []string                      `json:"entrypoint,omitempty"`
	Command       []string                      `json:"command,omitempty"`
	Environment   map[string]string             `json:"environment,omitempty"`
	Ports         []composePort                 `json:"ports,omitempty"`
	Volumes       []composeVolume               `json:"volumes,omitempty"`
	MemLimit      string                        `json:"mem_limit"`
	ShmSize       string                        `json:"shm_size,omitempty"`
	Logging       composeLogging                `json:"logging"`
	DependsOn     map[string]composeDepends     `json:"depends_on,omitempty"`
	Healthcheck   *composeHealth                `json:"healthcheck,omitempty"`
	Networks      map[string]*composeServiceNet `json:"networks,omitempty"`
	Labels        map[string]string             `json:"labels"`
}

type composePort struct {
	Target    int    `json:"target"`
	Published string `json:"published"`
	HostIP    string `json:"host_ip,omitempty"`
	Protocol  string `json:"protocol"`
}

type composeVolume struct {
	Type     string `json:"type"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"read_only,omitempty"`
}

type composeLogging struct {
	Driver  string            `json:"driver"`
	Options map[string]string `json:"options"`
}

type composeDepends struct {
	Condition string `json:"condition"`
}

type composeHealth struct {
	Test        []string `json:"test"`
	Interval    string   `json:"interval"`
	Timeout     string   `json:"timeout"`
	Retries     int      `json:"retries"`
	StartPeriod string   `json:"start_period"`
}

type composeNetwork struct {
	External bool   `json:"external,omitempty"`
	Name     string `json:"name,omitempty"`
}

type composeServiceNet struct {
	Aliases []string `json:"aliases,omitempty"`
}

// esc doubles $: Compose would otherwise read "$name" in a value (bcrypt
// and argon2 hashes are full of $) as one of its own variables.
func esc(s string) string { return strings.ReplaceAll(s, "$", "$$") }

func escAll(list []string) []string {
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = esc(s)
	}
	return out
}

// Render returns the Compose file (JSON) of an instance.
func Render(app *App, e *Env) ([]byte, error) {
	r := app.Docker
	if r == nil {
		return nil, errors.New("not a Docker app")
	}
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,40}$`).MatchString(e.Name) || !strings.HasPrefix(e.ProjectDir, "/") {
		return nil, errors.New("invalid instance")
	}
	main := r.MainService()
	cf := &composeFile{Name: ProjectName(e.Name), Services: map[string]*composeService{}}
	for _, n := range r.Networks {
		if cf.Networks == nil {
			cf.Networks = map[string]*composeNetwork{"default": {}}
		}
		cf.Networks[n] = &composeNetwork{External: true, Name: n}
	}
	for _, name := range sortedKeys(r.Services) {
		s := r.Services[name]
		mem := s.MemoryMB
		if e.MemoryMB > 0 {
			mem = e.MemoryMB
		}
		cs := &composeService{
			Image:         s.ImageRef(),
			ContainerName: ContainerName(e.Name, name, name == main),
			Restart:       "unless-stopped",
			MemLimit:      strconv.Itoa(mem) + "m",
			Logging:       composeLogging{Driver: "json-file", Options: map[string]string{"max-size": "10m", "max-file": "3"}},
			Labels:        map[string]string{"panel.app": app.ID, "panel.instance": e.Name, "panel.service": name},
		}
		switch s.User {
		case "":
		case "site":
			if e.SiteUID <= 0 {
				return nil, errNoSiteVar
			}
			cs.User = fmt.Sprintf("%d:%d", e.SiteUID, e.SiteGID)
		default:
			cs.User = s.User
		}
		if s.ShmMB > 0 {
			cs.ShmSize = strconv.Itoa(s.ShmMB) + "m"
		}
		for _, list := range []struct {
			in  []string
			out *[]string
		}{{s.Entrypoint, &cs.Entrypoint}, {s.Command, &cs.Command}} {
			for _, v := range list.in {
				x, err := e.Expand(v)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", name, err)
				}
				*list.out = append(*list.out, esc(x))
			}
		}
		env := map[string]string{}
		add := func(m map[string]string) error {
			for k, v := range m {
				x, err := e.Expand(v)
				if err != nil {
					return fmt.Errorf("%s %s: %w", name, k, err)
				}
				env[k] = esc(x)
			}
			return nil
		}
		if err := add(s.Env); err != nil {
			return nil, err
		}
		if e.Bootstrap {
			if err := add(s.EnvBootstrap); err != nil {
				return nil, err
			}
		}
		if len(env) > 0 {
			cs.Environment = env
		}
		for _, v := range s.Volumes {
			cs.Volumes = append(cs.Volumes, composeVolume{Type: "bind", Source: path.Join(e.DataDir(), v.Source),
				Target: v.Target, ReadOnly: v.ReadOnly})
		}
		for _, p := range sortedKeys(s.SecretFiles) {
			cs.Volumes = append(cs.Volumes, composeVolume{Type: "bind",
				Source: path.Join(e.SecretsDir(), s.SecretFiles[p]), Target: p, ReadOnly: true})
		}
		// A long start period: failures in it do not count, and the first
		// success ends it. First starts run migrations for minutes on one CPU,
		// and "up --wait" gives up at the first "unhealthy".
		if len(s.Health) > 0 {
			cs.Healthcheck = &composeHealth{Test: append([]string{"CMD"}, escAll(s.Health)...), Interval: "5s",
				Timeout: "5s", Retries: 60, StartPeriod: "5m"}
		}
		for _, d := range s.DependsOn {
			cond := "service_started"
			if len(r.Services[d].Health) > 0 {
				cond = "service_healthy"
			}
			if cs.DependsOn == nil {
				cs.DependsOn = map[string]composeDepends{}
			}
			cs.DependsOn[d] = composeDepends{Condition: cond}
		}
		if name == main {
			switch {
			case r.Web != nil:
				cs.Ports = append(cs.Ports, composePort{Target: r.Web.Port, Published: strconv.Itoa(e.Port), HostIP: "127.0.0.1", Protocol: "tcp"})
			case r.Service != nil:
				cs.Ports = append(cs.Ports, composePort{Target: r.Service.Port, Published: strconv.Itoa(e.Port), HostIP: "127.0.0.1", Protocol: "tcp"})
			}
			if len(r.Networks) > 0 {
				cs.Networks = map[string]*composeServiceNet{"default": {}}
				for _, n := range r.Networks {
					sn := &composeServiceNet{}
					if r.Service != nil && r.Service.Alias != "" {
						sn.Aliases = []string{r.Service.Alias}
					}
					cs.Networks[n] = sn
				}
			}
		}
		for _, p := range r.ExtraPorts {
			if p.Service == name {
				host, ok := e.ExtraPorts[p.Port]
				if !ok {
					return nil, fmt.Errorf("no host port for %s", p.Label)
				}
				cs.Ports = append(cs.Ports, composePort{Target: p.Port, Published: strconv.Itoa(host), HostIP: "127.0.0.1", Protocol: "tcp"})
			}
		}
		cf.Services[name] = cs
	}
	return json.MarshalIndent(cf, "", "  ")
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
