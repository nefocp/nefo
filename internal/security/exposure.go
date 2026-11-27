package security

import "strings"

// How far the firewall lets the internet reach a TCP port (see Exposure).
const (
	ExposureOpen    = "open"    // ufw is off or missing, or a rule lets everyone in
	ExposureLimited = "limited" // only rules for some addresses let connections in
	ExposureBlocked = "blocked" // no rule lets connections in
)

// Exposure reports how far the firewall lets connections reach TCP port
// p, from the rules ufw has while it is on. It assumes ufw's default policy
// for incoming connections (deny), which the panel never changes. Docker's
// published ports bypass these rules (Docker writes its own iptables rules
// ahead of ufw's), so they are not judged here.
func (f Firewall) Exposure(p int) string {
	if !f.Installed || !f.Active {
		return ExposureOpen
	}
	limited := false
	for _, r := range f.Rules {
		if !letsIn(r) || !portIn(r.To, p) {
			continue
		}
		if strings.EqualFold(trimV6(r.From), "Anywhere") {
			return ExposureOpen
		}
		limited = true
	}
	if limited {
		return ExposureLimited
	}
	return ExposureBlocked
}
