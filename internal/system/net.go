package system

import (
	"bufio"
	"io"
	"strconv"
	"strings"
	"time"
)

// NetCounter holds an interface's byte counters since boot.
type NetCounter struct {
	Rx, Tx uint64
}

// parseNetDev reads /proc/net/dev: two header lines, then one line per
// interface, "name: rx_bytes rx_packets … (8 receive fields) tx_bytes …".
func parseNetDev(r io.Reader) map[string]NetCounter {
	out := map[string]NetCounter{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		name, rest, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue // a header line
		}
		f := strings.Fields(rest)
		if len(f) < 9 {
			continue
		}
		rx, err1 := strconv.ParseUint(f[0], 10, 64)
		tx, err2 := strconv.ParseUint(f[8], 10, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		out[strings.TrimSpace(name)] = NetCounter{Rx: rx, Tx: tx}
	}
	return out
}

// defaultRouteIfaces returns the interfaces that carry a default route, from
// /proc/net/route (IPv4: destination 00000000, mask 00000000) and
// /proc/net/ipv6_route (destination ::/0). These are the internet-facing
// interfaces: counting them, and not docker0, the containers' veth pairs or
// a WireGuard tunnel, counts each byte once.
func defaultRouteIfaces(route4, route6 io.Reader) map[string]bool {
	out := map[string]bool{}
	if route4 != nil {
		sc := bufio.NewScanner(route4)
		for sc.Scan() {
			f := strings.Fields(sc.Text())
			// Iface Destination Gateway Flags RefCnt Use Metric Mask …
			if len(f) >= 8 && f[1] == "00000000" && f[7] == "00000000" {
				out[f[0]] = true
			}
		}
	}
	if route6 != nil {
		sc := bufio.NewScanner(route6)
		for sc.Scan() {
			f := strings.Fields(sc.Text())
			// dest dest_prefix_len src src_prefix_len next_hop metric refcnt use flags iface
			if len(f) >= 10 && f[0] == strings.Repeat("0", 32) && f[1] == "00" && f[9] != "lo" {
				out[f[9]] = true
			}
		}
	}
	delete(out, "lo")
	return out
}

// pickIfaces keeps the default-route interfaces, or every interface except
// the loopback when no default route was found.
func pickIfaces(all map[string]NetCounter, def map[string]bool) map[string]NetCounter {
	out := map[string]NetCounter{}
	for name, c := range all {
		if name == "lo" {
			continue
		}
		if len(def) > 0 && !def[name] {
			continue
		}
		out[name] = c
	}
	return out
}

// NetRate turns two readings of NetCounters into bytes per second. Only
// interfaces present in both count, and a counter that went down (the
// interface was recreated) counts as zero, so a change of interfaces never
// shows as a spike.
func NetRate(prev, cur map[string]NetCounter, elapsed time.Duration) (rx, tx float64) {
	sec := elapsed.Seconds()
	if sec <= 0 {
		return 0, 0
	}
	var drx, dtx uint64
	for name, c := range cur {
		p, ok := prev[name]
		if !ok {
			continue
		}
		if c.Rx >= p.Rx {
			drx += c.Rx - p.Rx
		}
		if c.Tx >= p.Tx {
			dtx += c.Tx - p.Tx
		}
	}
	return float64(drx) / sec, float64(dtx) / sec
}
