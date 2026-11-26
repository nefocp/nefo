package hosting

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/nefocp/nefo/internal/secure"
	"github.com/nefocp/nefo/internal/version"
)

// Adminer is the database web UI. It runs behind an internal server of the
// web server (Nginx, Apache or OpenLiteSpeed) that only listens on 127.0.0.1
// and answers only requests carrying a secret token,
// which the panel adds when it forwards /<entrance>/adminer/ after its own
// sign-in. So Adminer is never reachable from the internet, nor by other
// programs on the server.
const (
	AdminerVersion  = "6.3.1"
	adminerURL      = "https://github.com/vrana/adminer/releases/download/v6.3.1/adminer-6.3.1-mysql.php"
	adminerSHA256   = "97fdb49c3e941636fe4b0559b2b042de3ac105dff0daafec3c255cd6bd81d4b6"
	adminerDir      = "/usr/share/panel-adminer"
	adminerPortKey  = "adminer_port"
	adminerTokenKey = "adminer_token"
	// AdminerTokenHeader is the request header the internal server checks.
	AdminerTokenHeader = "X-Panel-Adminer"
	// AdminerHTTPSHeader tells Adminer the browser is using HTTPS.
	AdminerHTTPSHeader = "X-Panel-Https"
)

func (m *Manager) adminerConf() string { return filepath.Join(m.web().sitesDir(), "_adminer.conf") }

// adminerUserIni holds Adminer's PHP limits for Apache and OpenLiteSpeed,
// which cannot pass several PHP_VALUE lines the way Nginx does.
const adminerUserIni = `upload_max_filesize = 256M
post_max_size = 256M
max_execution_time = 600
memory_limit = 256M
`

// AdminerInstalled reports whether Adminer is installed.
func (m *Manager) AdminerInstalled() bool {
	return fileExists(filepath.Join(adminerDir, "index.php")) && fileExists(m.adminerConf())
}

// AdminerTarget returns the internal address Adminer listens on and the
// token to send with each request, or empty strings if it is not installed.
func (m *Manager) AdminerTarget(ctx context.Context) (addr, token string) {
	if !m.AdminerInstalled() {
		return "", ""
	}
	p, _ := m.store.Setting(ctx, adminerPortKey)
	t, _ := m.store.Setting(ctx, adminerTokenKey)
	if p == "" || t == "" {
		return "", ""
	}
	return "127.0.0.1:" + p, t
}

// adminerPHP picks the PHP version that runs Adminer: the distribution's
// default, or else the newest one installed.
func (m *Manager) adminerPHP(ctx context.Context) string {
	if php := m.distroPHP(ctx); m.phpInstalled(php) {
		return php
	}
	if vs := m.phpVersions(); len(vs) > 0 {
		return vs[len(vs)-1]
	}
	return ""
}

// adminerConfBytes renders the internal server for Adminer. It returns
// nil when Adminer has not been set up.
func (m *Manager) adminerConfBytes(ctx context.Context) ([]byte, error) {
	port, _ := m.store.Setting(ctx, adminerPortKey)
	token, _ := m.store.Setting(ctx, adminerTokenKey)
	if port == "" || token == "" {
		return nil, nil
	}
	php := m.adminerPHP(ctx)
	if php == "" {
		return nil, errors.New("PHP is not installed")
	}
	conf := m.web().adminerConf(port, token, php)
	if conf == nil {
		return nil, fmt.Errorf("refusing to render Adminer's server for port %q", port)
	}
	return conf, nil
}

// writeAdminerIni writes Adminer's PHP limits where Apache and
// OpenLiteSpeed need them.
func (m *Manager) writeAdminerIni() error {
	ini := filepath.Join(adminerDir, ".user.ini")
	if k := m.serverKind(); k != ServerApache && k != ServerOLS {
		return nil
	}
	if old, err := os.ReadFile(ini); err == nil && string(old) == adminerUserIni {
		return nil
	}
	return writeFile(ini, []byte(adminerUserIni), 0o644)
}

// InstallAdminer downloads the pinned Adminer release (checked against its
// SHA-256) and configures the web server's internal server for it.
func (m *Manager) InstallAdminer(ctx context.Context, lg Logger) error {
	if !m.Supported() {
		return ErrUnsupported
	}
	if !fileExists(m.web().hookFile()) {
		return ErrNotReady
	}
	if m.adminerPHP(ctx) == "" {
		return errors.New("PHP is not installed")
	}
	m.ops.Lock()
	defer m.ops.Unlock()

	lg.Logf("Downloading Adminer %s", AdminerVersion)
	b, err := fetch(ctx, adminerURL, 4<<20)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != adminerSHA256 {
		return errors.New("the Adminer download does not match its pinned checksum: not installed")
	}
	if err := mkdir(adminerDir, 0o755); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(adminerDir, "index.php"), b, 0o644); err != nil {
		return err
	}
	if err := m.writeAdminerIni(); err != nil {
		return err
	}

	if p, _ := m.store.Setting(ctx, adminerPortKey); p == "" {
		if err := m.store.SetSetting(ctx, adminerPortKey, strconv.Itoa(freePort())); err != nil {
			return err
		}
	}
	if t, _ := m.store.Setting(ctx, adminerTokenKey); t == "" {
		if err := m.store.SetSetting(ctx, adminerTokenKey, hex.EncodeToString(secure.Bytes(24))); err != nil {
			return err
		}
	}
	conf, err := m.adminerConfBytes(ctx)
	if err != nil {
		return err
	}
	if err := writeFile(m.adminerConf(), conf, 0o600); err != nil {
		return err
	}
	if err := m.reloadWeb(ctx); err != nil {
		removeIfExists(m.adminerConf())
		m.reloadWeb(context.WithoutCancel(ctx))
		return err
	}
	lg.Logf("Adminer %s is ready. Open it from the Databases page.", AdminerVersion)
	return nil
}

// RemoveAdminer uninstalls Adminer: its internal server, its files and its
// settings (the port and the token). Databases are not touched.
func (m *Manager) RemoveAdminer(ctx context.Context, lg Logger) error {
	if !m.Supported() {
		return ErrUnsupported
	}
	m.ops.Lock()
	defer m.ops.Unlock()
	lg.Logf("Removing Adminer")
	conf := m.adminerConf()
	hadConf := fileExists(conf)
	if err := m.removeAdminer(ctx); err != nil {
		return err
	}
	if hadConf && fileExists(m.web().hookFile()) && m.web().info(ctx).Running {
		if err := m.reloadWeb(ctx); err != nil {
			return err
		}
	}
	lg.Logf("Adminer was removed.")
	return nil
}

// removeAdminer deletes Adminer's internal server configuration (of every
// web server), its files and its settings. The caller holds the lock and
// reloads the web server.
func (m *Manager) removeAdminer(ctx context.Context) error {
	for _, kind := range serverKinds {
		if err := removeIfExists(filepath.Join(m.server(kind).sitesDir(), "_adminer.conf")); err != nil {
			return err
		}
	}
	for _, f := range []string{"index.php", ".user.ini"} {
		if err := removeIfExists(filepath.Join(adminerDir, f)); err != nil {
			return err
		}
	}
	os.Remove(adminerDir) // only if empty
	for _, k := range []string{adminerPortKey, adminerTokenKey} {
		if err := m.store.SetSetting(ctx, k, ""); err != nil {
			return err
		}
	}
	return nil
}

// freePort picks a random unused port on 127.0.0.1.
func freePort() int {
	for i := 0; i < 50; i++ {
		p := 40000 + secure.Intn(20000)
		if l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(p)); err == nil {
			l.Close()
			return p
		}
	}
	return 40000 + secure.Intn(20000)
}

func fetch(ctx context.Context, url string, max int64) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", version.AppName+"/"+version.Version)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err == nil && int64(len(b)) > max {
		err = fmt.Errorf("download %s: larger than expected", url)
	}
	return b, err
}
