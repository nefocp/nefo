package procs

import (
	"cmp"
	"context"
	"net/netip"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Port is a TCP or UDP port a program listens on.
type Port struct {
	Proto   string `json:"proto"`   // tcp or udp
	Address string `json:"address"` // "*", "0.0.0.0" and "::" mean every address
	Port    int    `json:"port"`
	Process string `json:"process"`
	PID     int    `json:"pid"`
	// Local ports only accept connections from this server itself.
	Local bool `json:"local"`
}

// ssUserRe matches one process in ss's last column:
// users:(("nginx",pid=118072,fd=11),("nginx",pid=81058,fd=11))
var ssUserRe = regexp.MustCompile(`\("((?:[^"\\]|\\.)*)",pid=(\d+),fd=\d+\)`)

// parseSS reads "ss -tulpnH": Netid State Recv-Q Send-Q Local Peer Process.
func parseSS(out string) []Port {
	list := []Port{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 6 || (f[0] != "tcp" && f[0] != "udp") {
			continue
		}
		local := f[4]
		i := strings.LastIndexByte(local, ':')
		if i <= 0 {
			continue
		}
		port, err := strconv.Atoi(local[i+1:])
		if err != nil || port <= 0 || port > 65535 {
			continue
		}
		p := Port{Proto: f[0], Address: strings.Trim(local[:i], "[]"), Port: port}
		host, _, _ := strings.Cut(p.Address, "%") // 127.0.0.53%lo: address on an interface
		if a, err := netip.ParseAddr(host); err == nil {
			p.Local = a.Unmap().IsLoopback()
		}
		// Several processes can share a socket (Nginx workers): show the
		// name of the first and the lowest PID, which is usually the master.
		for _, mm := range ssUserRe.FindAllStringSubmatch(strings.Join(f[6:], " "), -1) {
			pid, _ := strconv.Atoi(mm[2])
			if p.Process == "" {
				p.Process = mm[1]
			}
			if p.PID == 0 || pid < p.PID {
				p.PID = pid
			}
		}
		list = append(list, p)
	}
	slices.SortFunc(list, func(a, b Port) int {
		return cmp.Or(cmp.Compare(a.Port, b.Port), cmp.Compare(a.Proto, b.Proto), cmp.Compare(a.Address, b.Address))
	})
	return list
}

// Ports returns the listening TCP and UDP ports.
func (m *Manager) Ports(ctx context.Context) ([]Port, error) {
	out, err := m.run.Output(ctx, "ss", "-tulpnH")
	if err != nil {
		return nil, err
	}
	return parseSS(out), nil
}
