package run

import "testing"

func TestSystemdUnreachable(t *testing.T) {
	for _, out := range []string{
		"Synchronizing state of postgresql.service with SysV service script with /usr/lib/systemd/systemd-sysv-install.\n" +
			"Executing: /usr/lib/systemd/systemd-sysv-install enable postgresql\n" +
			"Failed to connect to system scope bus via local transport: Connection refused\n",
		"Failed to connect to bus: No such file or directory\n",
		"Failed to get properties: Transport endpoint is not connected\n",
	} {
		if !systemdUnreachable(out) {
			t.Errorf("not seen as unreachable: %q", out)
		}
	}
	for _, out := range []string{
		"Failed to enable unit: Unit file nope.service does not exist.\n",
		"Job for nginx.service failed because the control process exited with error code.\n",
		"",
	} {
		if systemdUnreachable(out) {
			t.Errorf("seen as unreachable: %q", out)
		}
	}
}
