package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nefocp/nefo/internal/software"
	"github.com/nefocp/nefo/internal/store"
)

func TestJavaVersionRefusals(t *testing.T) {
	env := newEnv(t)
	// A JDK as the openjdk package lays it out, in a folder of the test's.
	old := software.JVMDir
	software.JVMDir = filepath.Join(t.TempDir(), "jvm")
	t.Cleanup(func() { software.JVMDir = old })
	bin := filepath.Join(software.JVMDir, "java-21-openjdk-"+runtime.GOARCH, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(bin, "java"), nil, 0o755)
	ctx := context.Background()
	site := &store.Site{Domain: "app.example.com", SysUser: "app_example_com", ProxyPort: 8080, AppCommand: "java -jar app.jar", AppRuntime: "java:21"}
	if err := env.store.CreateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	if err := env.store.UpdateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	a := &authed{user: &store.User{Username: "admin"}}
	call := func(method, major string) (int, map[string]string) {
		t.Helper()
		r := httptest.NewRequest(method, "/api/java-versions/"+major, nil)
		r.SetPathValue("major", major)
		w := httptest.NewRecorder()
		env.srv.handleJavaVersion(w, r, a)
		var res map[string]string
		json.Unmarshal(w.Body.Bytes(), &res)
		return w.Code, res
	}
	for _, major := range []string{"x", "3", "1000", "-21"} {
		if code, res := call(http.MethodPost, major); code != http.StatusBadRequest || res["error"] != "invalid_java_version" {
			t.Errorf("%s: %d %v", major, code, res)
		}
	}
	if code, res := call(http.MethodDelete, "21"); code != http.StatusBadRequest || res["error"] != "java_in_use" || res["detail"] != "21" {
		t.Fatalf("in use: %d %v", code, res)
	}
	if code, res := call(http.MethodDelete, "17"); code != http.StatusBadRequest || res["error"] != "software_not_installed" {
		t.Fatalf("not installed: %d %v", code, res)
	}
	if jobs := env.srv.jobs.Recent(); len(jobs) != 0 {
		t.Fatalf("jobs started: %v", jobs)
	}
	if name, ok := softwareName("java"); !ok || name != "Java" {
		t.Fatal(name)
	}
	if name, ok := softwareName("ioncube"); !ok || name != "ionCube Loader" {
		t.Fatal(name)
	}
}
