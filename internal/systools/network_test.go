package systools

import (
	"context"
	"path/filepath"
	"testing"
)

// kernel sets the /proc values of the test's kernel.
func kernel(t *testing.T, e *testEnv, cc, avail, qdisc string) {
	e.write(t, e.m.p.ProcCC, cc+"\n")
	e.write(t, e.m.p.ProcCCAvail, avail+"\n")
	e.write(t, e.m.p.ProcQdisc, qdisc+"\n")
	e.write(t, e.m.p.OSRelease, "6.8.0-49-generic\n")
}

// applies makes sysctl --system apply the panel's file, as on a server.
func applies(t *testing.T, e *testEnv) {
	e.f.hook = func(line string) {
		if line == "sysctl --system" && exists(e.m.p.SysctlBBR) {
			e.write(t, e.m.p.ProcCC, "bbr\n")
			e.write(t, e.m.p.ProcQdisc, "fq\n")
		}
		if line == "modprobe tcp_bbr" {
			e.write(t, e.m.p.ProcCCAvail, "reno cubic bbr\n")
		}
		if line == "sysctl -w net.ipv4.tcp_congestion_control=cubic net.core.default_qdisc=fq_codel" {
			e.write(t, e.m.p.ProcCC, "cubic\n")
			e.write(t, e.m.p.ProcQdisc, "fq_codel\n")
		}
	}
}

func TestBBR(t *testing.T) {
	e := newTest(t)
	ctx := context.Background()
	kernel(t, e, "cubic", "reno cubic", "fq_codel")
	n := e.m.Network()
	if n.Congestion != "cubic" || n.Qdisc != "fq_codel" || n.BBR || n.BBRAvailable || n.Panel || len(n.Available) != 2 {
		t.Fatalf("network = %+v", n)
	}
	// No module for this kernel: refused, nothing written.
	if err := e.m.EnableBBR(ctx); code(err) != "bbr_unavailable" {
		t.Fatalf("no bbr: %v", err)
	}
	if exists(e.m.p.SysctlBBR) || exists(e.m.p.ModulesBBR) || e.f.ran("sysctl") {
		t.Fatal("changed something")
	}

	// With the module: loaded, then both files, then sysctl --system.
	e.write(t, filepath.Join(e.m.p.Modules, "6.8.0-49-generic", "kernel", "net", "ipv4", "tcp_bbr.ko.zst"), "")
	applies(t, e)
	if !e.m.Network().BBRAvailable {
		t.Fatal("module not found")
	}
	if err := e.m.EnableBBR(ctx); err != nil {
		t.Fatal(err)
	}
	if !e.f.ran("modprobe tcp_bbr") || !e.f.ran("sysctl --system") {
		t.Fatalf("calls = %q", e.f.calls)
	}
	if read(t, e.m.p.SysctlBBR) != bbrSysctl || read(t, e.m.p.ModulesBBR) != "# Written by the panel (System → Network).\ntcp_bbr\n" {
		t.Fatalf("files: %q %q", read(t, e.m.p.SysctlBBR), read(t, e.m.p.ModulesBBR))
	}
	if n := e.m.Network(); !n.BBR || n.Qdisc != "fq" || !n.Panel {
		t.Fatalf("network = %+v", n)
	}

	// Off: files gone, the kernel's defaults back.
	if err := e.m.DisableBBR(ctx); err != nil {
		t.Fatal(err)
	}
	if exists(e.m.p.SysctlBBR) || exists(e.m.p.ModulesBBR) {
		t.Fatal("files left")
	}
	if n := e.m.Network(); n.BBR || n.Qdisc != "fq_codel" || n.Panel {
		t.Fatalf("network = %+v", n)
	}
	if err := e.m.DisableBBR(ctx); code(err) != "bbr_not_panel" {
		t.Fatalf("off twice: %v", err)
	}
}

func TestBBRContainer(t *testing.T) {
	e := newTest(t)
	ctx := context.Background()
	// BBR is built in, but /proc/sys is read-only.
	kernel(t, e, "cubic", "reno cubic bbr", "fq_codel")
	e.f.fail["sysctl --system"] = "* Applying /etc/sysctl.d/90-panel-bbr.conf ...\nsysctl: setting key \"net.core.default_qdisc\": Read-only file system\n"
	if err := e.m.EnableBBR(ctx); code(err) != "sysctl_not_permitted" {
		t.Fatalf("container: %v", err)
	}
	if exists(e.m.p.SysctlBBR) || exists(e.m.p.ModulesBBR) {
		t.Fatal("files left behind")
	}
	if e.f.ran("modprobe") {
		t.Fatal("modprobe ran for a built-in BBR")
	}

	// A module that cannot be loaded.
	kernel(t, e, "cubic", "reno cubic", "fq_codel")
	e.write(t, filepath.Join(e.m.p.Modules, "6.8.0-49-generic", "kernel", "net", "ipv4", "tcp_bbr.ko"), "")
	e.f.fail["modprobe"] = "modprobe: FATAL: Module tcp_bbr not found in directory /lib/modules/6.8.0-49-generic"
	if err := e.m.EnableBBR(ctx); code(err) != "bbr_not_permitted" {
		t.Fatalf("modprobe: %v", err)
	}
}
