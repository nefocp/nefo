//go:build !linux

package system

import (
	"os"
	"runtime"

	"github.com/nefocp/nefo/internal/version"
)

// Info returns basic information. Full details are only read on Linux.
func (c *Collector) Info() Info {
	host, _ := os.Hostname()
	return Info{
		Hostname: host,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		CPUs:     runtime.NumCPU(),
		Version:  version.Version,
	}
}

// Stats is not implemented outside Linux; the UI shows a notice instead.
func (c *Collector) Stats() (Stats, error) {
	return Stats{}, nil
}

// NetCounters is only implemented on Linux.
func NetCounters() map[string]NetCounter { return nil }
