package hosting

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/software"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// fakeJava makes a JDK the way the openjdk packages lay it out.
func fakeJava(t *testing.T, major string) string {
	t.Helper()
	old := software.JVMDir
	software.JVMDir = filepath.Join(t.TempDir(), "jvm")
	t.Cleanup(func() { software.JVMDir = old })
	arch := map[string]string{"amd64": "amd64", "arm64": "arm64"}[runtime.GOARCH]
	if arch == "" {
		arch = runtime.GOARCH
	}
	home := filepath.Join(software.JVMDir, "java-"+major+"-openjdk-"+arch)
	if err := os.MkdirAll(filepath.Join(home, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "bin", "java"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	return home
}

func rtCode(err error) string {
	var ue *usererr.Error
	if errors.As(err, &ue) {
		return ue.Code + ":" + ue.Detail
	}
	if err != nil {
		return err.Error()
	}
	return ""
}

func TestJavaRuntime(t *testing.T) {
	home := fakeJava(t, "21")
	m := newTestManager(t)
	site := &store.Site{Domain: "app.example.com", SysUser: "app_example_com", ProxyPort: 8080, AppCommand: "java -jar app.jar", AppRuntime: "java:21", AppMemory: 512}

	if err := checkRuntime("java:21"); err != nil {
		t.Fatalf("java:21: %v", err)
	}
	if got := rtCode(checkRuntime("java:17")); got != "java_not_installed:17" {
		t.Fatalf("java:17: %s", got)
	}
	for _, rt := range []string{"java:", "java:21;x", "java", "jav:21", "java:1234"} {
		if got := rtCode(checkRuntime(rt)); !strings.HasPrefix(got, "invalid_runtime") {
			t.Errorf("%q: %s", rt, got)
		}
	}
	if p := m.runtimePath(site); p != filepath.Join(home, "bin")+":/usr/local/bin:/usr/bin:/bin" {
		t.Fatalf("PATH = %q", p)
	}
	if env := m.RuntimeEnv(site); len(env) != 1 || env[0] != "JAVA_HOME="+home {
		t.Fatalf("env = %v", env)
	}
	if env := strings.Join(serviceEnv(site), " "); env != "SERVER_PORT=8080 SERVER_ADDRESS=127.0.0.1 JDK_JAVA_OPTIONS=-Xmx512m" {
		t.Fatalf("service env = %v", env)
	}
	// Without a limit the JVM picks its own (a quarter of the memory).
	site.AppMemory = 0
	if env := strings.Join(serviceEnv(site), " "); env != "SERVER_PORT=8080 SERVER_ADDRESS=127.0.0.1" {
		t.Fatalf("service env = %v", env)
	}
	// Other runtimes get no Java variables, and no memory option.
	node := &store.Site{Domain: "n.example.com", AppRuntime: "", AppMemory: 256}
	if len(m.RuntimeEnv(node)) != 0 || len(serviceEnv(node)) != 0 {
		t.Fatal("Java variables for a Node.js app")
	}
	// The Java version was removed by hand: the system PATH, no JAVA_HOME.
	gone := &store.Site{Domain: "g.example.com", AppRuntime: "java:25"}
	if m.runtimePath(gone) != "/usr/local/bin:/usr/bin:/bin" || len(m.RuntimeEnv(gone)) != 0 {
		t.Fatal("missing Java version")
	}
}

func TestAppMemoryLimits(t *testing.T) {
	for mb, ok := range map[int]bool{0: true, 32: true, 512: true, 65536: true, 31: false, -1: false, 65537: false} {
		if err := checkAppMemory(mb); (err == nil) != ok {
			t.Errorf("%d MB: %v", mb, err)
		}
	}
}

func TestUnitEnv(t *testing.T) {
	got := unitEnv([]string{"JAVA_HOME=/usr/lib/jvm/java-21-openjdk-amd64", "JDK_JAVA_OPTIONS=-Xmx512m", "BAD=a b", "X=\n[Service]", "lower=x"})
	want := "Environment=JAVA_HOME=/usr/lib/jvm/java-21-openjdk-amd64\nEnvironment=JDK_JAVA_OPTIONS=-Xmx512m\n"
	if got != want {
		t.Fatalf("%q", got)
	}
}
