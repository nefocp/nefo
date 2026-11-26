package systools

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/usererr"
)

// ---- network: TCP BBR ----

// Network is the TCP congestion control state.
type Network struct {
	Congestion string   `json:"congestion"` // "cubic", "bbr"…
	Available  []string `json:"available"`
	Qdisc      string   `json:"qdisc"` // net.core.default_qdisc
	BBR        bool     `json:"bbr"`   // BBR is in use now
	// BBRAvailable means the kernel has BBR (loaded, or as a module).
	BBRAvailable bool `json:"bbr_available"`
	// Panel means the panel's files turn BBR on at boot.
	Panel bool `json:"panel"`
}

const bbrSysctl = "# Written by the panel (System → Network): TCP BBR with the fq queue.\n" +
	"net.core.default_qdisc = fq\nnet.ipv4.tcp_congestion_control = bbr\n"

func (n Network) has(cc string) bool { return slices.Contains(n.Available, cc) }

// Network returns the current congestion control and queue settings.
func (m *Manager) Network() Network {
	n := Network{
		Congestion: readTrim(m.p.ProcCC),
		Available:  strings.Fields(readTrim(m.p.ProcCCAvail)),
		Qdisc:      readTrim(m.p.ProcQdisc),
		Panel:      exists(m.p.SysctlBBR),
	}
	if n.Available == nil {
		n.Available = []string{}
	}
	n.BBR = n.Congestion == "bbr"
	n.BBRAvailable = n.has("bbr") || m.bbrModule() != ""
	return n
}

// bbrModule returns the tcp_bbr module file of the running kernel, if any.
func (m *Manager) bbrModule() string {
	k := readTrim(m.p.OSRelease)
	if k == "" || strings.ContainsAny(k, "/*?[") {
		return ""
	}
	files, _ := filepath.Glob(filepath.Join(m.p.Modules, k, "kernel", "net", "ipv4", "tcp_bbr.ko*"))
	if len(files) == 0 {
		return ""
	}
	return files[0]
}

// EnableBBR turns on TCP BBR with the fq queue, now and at every boot: a
// modules-load.d file loads tcp_bbr, a sysctl.d file sets both values.
func (m *Manager) EnableBBR(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := m.Network()
	if !n.has("bbr") {
		if !n.BBRAvailable {
			return usererr.New("bbr_unavailable", readTrim(m.p.OSRelease))
		}
		if out, err := m.run.Output(ctx, "modprobe", "tcp_bbr"); err != nil {
			if notPermitted(out) || strings.Contains(out, "not found") {
				return usererr.New("bbr_not_permitted")
			}
			return err
		}
		if !m.Network().has("bbr") {
			return usererr.New("bbr_not_permitted")
		}
	}
	oldMod, oldSys := readOld(m.p.ModulesBBR), readOld(m.p.SysctlBBR)
	undo := func() {
		restore(m.p.ModulesBBR, oldMod)
		restore(m.p.SysctlBBR, oldSys)
	}
	if err := writeFile(m.p.ModulesBBR, []byte("# Written by the panel (System → Network).\ntcp_bbr\n"), 0o644); err != nil {
		return err
	}
	if err := writeFile(m.p.SysctlBBR, []byte(bbrSysctl), 0o644); err != nil {
		undo()
		return err
	}
	// sysctl --system fails when any setting of any file cannot be applied,
	// so what counts is whether these two took effect.
	out, err := m.run.Output(ctx, "sysctl", "--system")
	if now := m.Network(); now.Congestion != "bbr" || now.Qdisc != "fq" {
		undo()
		if notPermitted(out) {
			return usererr.New("sysctl_not_permitted")
		}
		if err != nil {
			return err
		}
		return &run.Error{Cmd: "sysctl --system", Err: os.ErrInvalid, Output: out}
	}
	return nil
}

// DisableBBR removes the panel's BBR files and goes back to the kernel's
// defaults: cubic, and the fq_codel queue of systemd's sysctl defaults.
func (m *Manager) DisableBBR(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !exists(m.p.SysctlBBR) && !exists(m.p.ModulesBBR) {
		return usererr.New("bbr_not_panel")
	}
	for _, p := range []string{m.p.SysctlBBR, m.p.ModulesBBR} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	out, err := m.run.Output(ctx, "sysctl", "-w", "net.ipv4.tcp_congestion_control=cubic", "net.core.default_qdisc=fq_codel")
	if err != nil {
		if notPermitted(out) {
			return usererr.New("sysctl_not_permitted")
		}
		return err
	}
	// Settings of the administrator's own files apply again.
	m.run.Output(ctx, "sysctl", "--system")
	return nil
}
