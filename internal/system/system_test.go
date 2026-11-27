package system

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestParseCPULine(t *testing.T) {
	idle, total, err := parseCPULine("cpu  100 0 50 800 50 0 0 0 30 0")
	if err != nil {
		t.Fatal(err)
	}
	if idle != 850 || total != 1000 {
		t.Errorf("idle=%d total=%d, want 850 1000", idle, total)
	}
	if _, _, err := parseCPULine("intr 1 2 3"); err == nil {
		t.Error("accepted a non-cpu line")
	}
}

func TestCPUPercent(t *testing.T) {
	if got := cpuPercent(800, 1000, 850, 1100); math.Abs(got-50) > 1e-9 {
		t.Errorf("got %v, want 50", got)
	}
	if got := cpuPercent(0, 1000, 0, 1000); got != 0 {
		t.Errorf("no time passed: got %v", got)
	}
}

func TestParseLoadavgUptimeOSRelease(t *testing.T) {
	l1, l5, l15, err := parseLoadavg("0.52 0.58 0.59 1/389 12345\n")
	if err != nil || l1 != 0.52 || l5 != 0.58 || l15 != 0.59 {
		t.Errorf("loadavg: %v %v %v %v", l1, l5, l15, err)
	}
	if up, err := parseUptime("3600.55 7000.00\n"); err != nil || up != 3600 {
		t.Errorf("uptime: %v %v", up, err)
	}
	osr := "NAME=\"Ubuntu\"\nPRETTY_NAME=\"Ubuntu 24.04.1 LTS\"\nID=ubuntu\n"
	if got := parseOSRelease(strings.NewReader(osr)); got != "Ubuntu 24.04.1 LTS" {
		t.Errorf("os-release: %q", got)
	}
}

func TestNetCounters(t *testing.T) {
	dev := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 9000      10    0    0    0     0          0         0     9000      10    0    0    0     0       0          0
  eth0: 1000000   900    0    0    0     0          0         0   250000     700    0    0    0     0       0          0
docker0: 5000     50    0    0    0     0          0         0     7000      60    0    0    0     0       0          0
vethab12: 7000    60    0    0    0     0          0         0     5000      50    0    0    0     0       0          0
`
	all := parseNetDev(strings.NewReader(dev))
	if len(all) != 4 || all["eth0"] != (NetCounter{Rx: 1000000, Tx: 250000}) || all["docker0"].Tx != 7000 {
		t.Fatalf("parseNetDev = %v", all)
	}
	route4 := "Iface\tDestination\tGateway \tFlags\tRefCnt\tUse\tMetric\tMask\t\tMTU\tWindow\tIRTT\n" +
		"eth0\t00000000\t0100A8C0\t0003\t0\t0\t100\t00000000\t0\t0\t0\n" +
		"eth0\t0000A8C0\t00000000\t0001\t0\t0\t100\t00FFFFFF\t0\t0\t0\n" +
		"docker0\t000011AC\t00000000\t0001\t0\t0\t0\t0000FFFF\t0\t0\t0\n"
	route6 := "00000000000000000000000000000000 00 00000000000000000000000000000000 00 fe800000000000000000000000000001 00000400 00000001 00000000 00000003 eth1\n" +
		"00000000000000000000000000000000 00 00000000000000000000000000000000 00 00000000000000000000000000000000 ffffffff 00000001 00000000 00200200 lo\n" +
		"fe800000000000000000000000000000 40 00000000000000000000000000000000 00 00000000000000000000000000000000 00000100 00000001 00000000 00000001 docker0\n"
	def := defaultRouteIfaces(strings.NewReader(route4), strings.NewReader(route6))
	if len(def) != 2 || !def["eth0"] || !def["eth1"] {
		t.Fatalf("defaultRouteIfaces = %v", def)
	}
	picked := pickIfaces(all, def)
	if len(picked) != 1 || picked["eth0"].Rx != 1000000 {
		t.Errorf("pickIfaces = %v, want eth0 only", picked)
	}
	fb := pickIfaces(all, nil)
	if _, lo := fb["lo"]; len(fb) != 3 || lo {
		t.Errorf("pickIfaces without a default route = %v, want all but lo", fb)
	}

	prev := map[string]NetCounter{"eth0": {Rx: 1000, Tx: 500}, "wg0": {Rx: 10, Tx: 10}}
	cur := map[string]NetCounter{"eth0": {Rx: 61000, Tx: 6500}, "eth1": {Rx: 9e9, Tx: 9e9}, "wg0": {Rx: 5, Tx: 20}}
	rx, tx := NetRate(prev, cur, time.Minute)
	// eth0: +60000/+6000; eth1 is new (ignored); wg0's rx went down (zero), tx +10.
	if math.Abs(rx-1000) > 1e-9 || math.Abs(tx-(6010.0/60)) > 1e-9 {
		t.Errorf("NetRate = %v, %v", rx, tx)
	}
	if rx, tx := NetRate(prev, cur, 0); rx != 0 || tx != 0 {
		t.Error("NetRate with no elapsed time is not zero")
	}
}
