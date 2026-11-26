package systools

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
)

const meminfo = `MemTotal:         985000 kB
MemFree:           81234 kB
MemAvailable:     412000 kB
SwapTotal:       2097148 kB
SwapFree:        2000000 kB
`

const swapsHead = "Filename\t\t\t\tType\t\tSize\t\tUsed\t\tPriority\n"

// procPath escapes a path as /proc/swaps does (a temporary folder on
// Windows has backslashes).
func procPath(p string) string {
	return strings.NewReplacer(`\`, `\134`, " ", `\040`).Replace(p)
}

func TestSwapState(t *testing.T) {
	e := newTest(t)
	e.write(t, e.m.p.ProcSwaps, swapsHead+"/dev/vda2                               partition\t1048572\t\t0\t\t-2\n"+
		"/swap\\040file                           file\t\t1048576\t\t97148\t\t-3\n")
	e.write(t, e.m.p.Meminfo, meminfo)
	e.write(t, e.m.p.Swappiness, "60\n")
	s := e.m.Swap()
	if len(s.Devices) != 2 || s.Devices[1].Path != "/swap file" || s.Devices[1].UsedKB != 97148 || s.Devices[0].Type != "partition" || s.Devices[0].Priority != -2 {
		t.Fatalf("devices = %+v", s.Devices)
	}
	if s.Foreign != "/swap file" || s.FileActive || s.TotalKB != 2097148 || s.MemAvailableKB != 412000 || s.Swappiness != 60 || s.DiskFreeMB != 50<<10 {
		t.Fatalf("swap = %+v", s)
	}
}

// swapOn makes swapon and swapoff change /proc/swaps, as the kernel does.
func swapOn(t *testing.T, e *testEnv) {
	e.write(t, e.m.p.ProcSwaps, swapsHead)
	e.f.hook = func(line string) {
		switch {
		case strings.HasPrefix(line, "swapon "):
			e.write(t, e.m.p.ProcSwaps, swapsHead+procPath(e.m.p.SwapFile)+" file 2097148 0 -2\n")
		case strings.HasPrefix(line, "swapoff "):
			e.write(t, e.m.p.ProcSwaps, swapsHead)
		}
	}
}

func TestCreateAndRemoveSwap(t *testing.T) {
	e := newTest(t)
	ctx := context.Background()
	swapOn(t, e)
	e.write(t, e.m.p.Meminfo, meminfo)
	e.write(t, e.m.p.Fstab, "# /etc/fstab\nUUID=abcd / ext4 defaults 0 1\n")
	var lg testLog
	if err := e.m.CreateSwap(ctx, 2048, &lg); err != nil {
		t.Fatalf("%v\n%s", err, lg.String())
	}
	path := e.m.p.SwapFile
	for _, c := range []string{"fallocate -l 2147483648 " + path, "mkswap " + path, "swapon " + path} {
		if !e.f.ran(c) {
			t.Fatalf("%q did not run: %q", c, e.f.calls)
		}
	}
	if e.f.ran("dd ") || e.f.ran("chattr") {
		t.Fatalf("calls = %q", e.f.calls)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v", fi.Mode())
	}
	want := "# /etc/fstab\nUUID=abcd / ext4 defaults 0 1\n" + fstabEscape(path) + " none swap sw 0 0\n"
	if got := read(t, e.m.p.Fstab); got != want {
		t.Fatalf("fstab =\n%s", got)
	}
	if s := e.m.Swap(); !s.FileActive || !s.FileExists || s.Foreign != "" {
		t.Fatalf("swap = %+v", s)
	}
	// A second one is refused.
	if err := e.m.CreateSwap(ctx, 1024, &lg); code(err) != "swap_panel_exists" {
		t.Fatalf("second swap file: %v", err)
	}

	e.f.reset()
	if err := e.m.RemoveSwap(ctx, &lg); err != nil {
		t.Fatal(err)
	}
	if !e.f.ran("swapoff " + path) {
		t.Fatalf("calls = %q", e.f.calls)
	}
	if exists(path) || read(t, e.m.p.Fstab) != "# /etc/fstab\nUUID=abcd / ext4 defaults 0 1\n" {
		t.Fatalf("after removal: %v\n%s", exists(path), read(t, e.m.p.Fstab))
	}
	if err := e.m.RemoveSwap(ctx, &lg); code(err) != "swap_not_found" {
		t.Fatalf("remove twice: %v", err)
	}
}

func TestCreateSwapFallbacks(t *testing.T) {
	e := newTest(t)
	ctx := context.Background()
	swapOn(t, e)
	path := e.m.p.SwapFile

	// swapon refuses the preallocated file (holes): dd writes it again.
	tries := 0
	e.f.fn["swapon "] = func(string) (string, error) {
		tries++
		if tries == 1 {
			return "swapon: " + path + ": swapon failed: Invalid argument", os.ErrInvalid
		}
		return "", nil
	}
	var lg testLog
	if err := e.m.CreateSwap(ctx, 512, &lg); err != nil {
		t.Fatalf("%v\n%s", err, lg.String())
	}
	if tries != 2 || !e.f.ran("dd if=/dev/zero of="+path+" bs=1M count=512 status=none") {
		t.Fatalf("tries %d, calls = %q", tries, e.f.calls)
	}
	os.Remove(path)
	os.Remove(e.m.p.Fstab)
	e.write(t, e.m.p.ProcSwaps, swapsHead)

	// fallocate does not work on this file system: dd at once.
	e.f.reset()
	delete(e.f.fn, "swapon ")
	e.f.fail["fallocate"] = "fallocate: fallocate failed: Operation not supported"
	if err := e.m.CreateSwap(ctx, 512, &lg); err != nil {
		t.Fatal(err)
	}
	if !e.f.ran("dd if=/dev/zero") {
		t.Fatalf("calls = %q", e.f.calls)
	}
	os.Remove(path)
	os.Remove(e.m.p.Fstab)
	e.write(t, e.m.p.ProcSwaps, swapsHead)

	// btrfs: the file is marked not copy-on-write first.
	e.f.reset()
	delete(e.f.fail, "fallocate")
	e.fsType = "btrfs"
	if err := e.m.CreateSwap(ctx, 512, &lg); err != nil || !e.f.ran("chattr +C "+path) {
		t.Fatalf("btrfs: %v %q", err, e.f.calls)
	}
}

func TestCreateSwapRefused(t *testing.T) {
	e := newTest(t)
	ctx := context.Background()
	e.write(t, e.m.p.ProcSwaps, swapsHead)
	e.write(t, e.m.p.Fstab, "UUID=abcd / ext4 defaults 0 1\n")
	var lg testLog
	path := e.m.p.SwapFile

	if err := e.m.CreateSwap(ctx, 3000, &lg); code(err) != "swap_size_invalid" {
		t.Fatalf("odd size: %v", err)
	}
	e.free = 2 << 30
	if err := e.m.CreateSwap(ctx, 2048, &lg); code(err) != "swap_no_space" {
		t.Fatalf("small disk: %v", err)
	}
	e.free = 50 << 30
	e.fsType = "zfs"
	if err := e.m.CreateSwap(ctx, 1024, &lg); code(err) != "swap_unsupported_fs" {
		t.Fatalf("zfs: %v", err)
	}
	e.fsType = "ext4"

	// A container: swapon is not allowed. Nothing is left behind.
	e.f.fail["swapon"] = "swapon: " + path + ": swapon failed: Operation not permitted"
	if err := e.m.CreateSwap(ctx, 1024, &lg); code(err) != "swap_not_permitted" {
		t.Fatalf("container: %v", err)
	}
	if exists(path) || read(t, e.m.p.Fstab) != "UUID=abcd / ext4 defaults 0 1\n" || e.f.ran("dd ") {
		t.Fatalf("left behind: %v %q", exists(path), e.f.calls)
	}

	// A swap file someone else made: no second one next to it. A partition is fine.
	delete(e.f.fail, "swapon")
	e.write(t, e.m.p.ProcSwaps, swapsHead+"/swapfile file 1048576 0 -2\n")
	if err := e.m.CheckCreateSwap(1024); code(err) != "swap_exists" {
		t.Fatalf("foreign file: %v", err)
	}
	e.write(t, e.m.p.ProcSwaps, swapsHead+"/dev/vda2 partition 1048576 0 -2\n")
	if err := e.m.CheckCreateSwap(1024); err != nil {
		t.Fatalf("partition: %v", err)
	}
}

func TestRemoveSwapInUse(t *testing.T) {
	e := newTest(t)
	e.write(t, e.m.p.SwapFile, "")
	e.write(t, e.m.p.ProcSwaps, swapsHead+procPath(e.m.p.SwapFile)+" file 2097148 900000 -2\n")
	e.write(t, e.m.p.Meminfo, meminfo) // 412000 kB available
	if err := e.m.RemoveSwap(context.Background(), &testLog{}); code(err) != "swap_in_use" {
		t.Fatalf("in use: %v", err)
	}
	if e.f.ran("swapoff") {
		t.Fatal("swapoff ran")
	}
}

func TestSwappiness(t *testing.T) {
	e := newTest(t)
	e.write(t, e.m.p.Swappiness, "60\n")
	if err := e.m.SetSwappiness(101); code(err) != "swappiness_invalid" {
		t.Fatalf("101: %v", err)
	}
	if err := e.m.SetSwappiness(10); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, e.m.p.SysctlSwap), "vm.swappiness = 10\n") || read(t, e.m.p.Swappiness) != "10\n" {
		t.Fatalf("files: %q %q", read(t, e.m.p.SysctlSwap), read(t, e.m.p.Swappiness))
	}
	if s := e.m.Swap(); s.Swappiness != 10 || !s.SwappinessSet {
		t.Fatalf("swap = %+v", s)
	}
}

func TestUnescapeOctal(t *testing.T) {
	for in, want := range map[string]string{`/a\040b`: "/a b", `/plain`: "/plain", `/end\04`: `/end\04`, `\134x`: `\x`} {
		if got := unescapeOctal(in); got != want {
			t.Errorf("unescape(%q) = %q, want %q", in, got, want)
		}
	}
}
