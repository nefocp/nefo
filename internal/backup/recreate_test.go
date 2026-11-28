package backup

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/store"
)

type nopLog struct{}

func (nopLog) Write(p []byte) (int, error) { return len(p), nil }
func (nopLog) Logf(string, ...any)         {}

func TestSiteManifest(t *testing.T) {
	// A Node.js app website: its program's settings, not a proxy address.
	app := &store.Site{Domain: "node.test", ProxyPort: 31100, AppCommand: "node server.js", AppRuntime: "node:22", AppMemory: 512,
		Aliases: []string{"www.node.test"}}
	man := siteManifest(app, "")
	if man.ProxyPort != 31100 || man.AppCommand != "node server.js" || man.AppRuntime != "node:22" || man.AppMemory != 512 ||
		man.ProxyURL != "" || man.ProxyOpts != nil || man.Kind != KindSite || man.Aliases[0] != "www.node.test" {
		t.Fatalf("app manifest %+v", man)
	}
	// A reverse proxy website: the address and its options.
	proxy := &store.Site{Domain: "px.test", ProxyURL: "https://192.168.1.20:8443/app",
		ProxyOpts: store.ProxyOptions{UpstreamHost: true, WebSocket: true, Timeout: 120, Insecure: true}}
	man = siteManifest(proxy, LabelBeforeDelete)
	if man.ProxyURL != proxy.ProxyURL || man.ProxyOpts == nil || *man.ProxyOpts != proxy.ProxyOpts || man.ProxyPort != 0 || man.Label != LabelBeforeDelete {
		t.Fatalf("proxy manifest %+v", man)
	}
	// The JSON names, read back.
	b, _ := json.Marshal(siteManifest(app, ""))
	for _, k := range []string{`"proxy_port":31100`, `"app_command":"node server.js"`, `"app_runtime":"node:22"`, `"app_memory":512`} {
		if !strings.Contains(string(b), k) {
			t.Errorf("app manifest JSON lacks %s: %s", k, b)
		}
	}
	b, _ = json.Marshal(siteManifest(proxy, ""))
	var back manifest
	if err := json.Unmarshal(b, &back); err != nil || back.ProxyURL != proxy.ProxyURL || back.ProxyOpts == nil || *back.ProxyOpts != proxy.ProxyOpts {
		t.Fatalf("proxy manifest read back %+v (%s)", back, b)
	}
	// A PHP website: nothing more than before.
	b, _ = json.Marshal(siteManifest(&store.Site{Domain: "php.test", PHPVersion: "8.3"}, ""))
	for _, k := range []string{"proxy", "app_"} {
		if strings.Contains(string(b), k) {
			t.Errorf("PHP manifest has %s: %s", k, b)
		}
	}
}

func TestOldManifest(t *testing.T) {
	// A backup made before the app and proxy settings were recorded: its
	// website comes back as before (PHP or static).
	var man manifest
	old := `{"format":1,"kind":"site","domain":"node.test","app":"","databases":[],"created_at":"2026-11-01T00:00:00Z","panel_version":"0.24.0"}`
	if err := json.Unmarshal([]byte(old), &man); err != nil {
		t.Fatal(err)
	}
	if man.ProxyPort != 0 || man.ProxyURL != "" || man.ProxyOpts != nil || man.AppRuntime != "" {
		t.Fatalf("%+v", man)
	}
}

func TestDockerSiteNotRecreated(t *testing.T) {
	// A Docker app's website: the App store installs it, its app backup
	// brings the data back. Old and new manifests alike.
	m := &Manager{}
	_, _, err := m.recreateSite(context.Background(), manifest{Kind: KindSite, Domain: "kuma.test", App: "docker:uptime-kuma", ProxyPort: 40123}, nopLog{})
	if err == nil || !strings.Contains(err.Error(), "install uptime-kuma from the App store for kuma.test first") ||
		!strings.Contains(err.Error(), "restore the app's backup") {
		t.Fatalf("err %v", err)
	}
}
