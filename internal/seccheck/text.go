package seccheck

import "strings"

// titles are short English texts of the findings, for alerts (the UI has
// its own, translated, texts).
var titles = map[string]string{
	"ssh_root_password":   "root can sign in over SSH with a password",
	"ssh_password":        "SSH accepts passwords ({users})",
	"ssh_port_22":         "SSH listens on port 22",
	"firewall_missing":    "the firewall (ufw) is not installed",
	"firewall_off":        "the firewall is off",
	"fail2ban_missing":    "fail2ban is not installed",
	"fail2ban_stopped":    "fail2ban is not running",
	"fail2ban_no_sshd":    "fail2ban does not watch SSH",
	"panel_http":          "the panel is reached without HTTPS",
	"panel_self_signed":   "the panel uses a self-signed certificate",
	"panel_2fa":           "two-factor authentication is off for {users}",
	"os_updates_unknown":  "the update check is out of date",
	"os_security_updates": "{n} security updates are waiting",
	"os_auto_updates_off": "automatic security updates are off",
	"os_reboot":           "a restart is needed to finish updates",
	"uid0":                "other accounts have root's ID 0: {users}",
	"empty_password":      "accounts with an empty password: {users}",
	"service_public":      "{service} listens on {address} port {port} (firewall: {exposure})",
	"redis_no_password":   "Redis has no password ({where})",
	"docker_public":       "Docker container {container} publishes ports {ports} on every address",
	"site_no_https":       "{domain} has no HTTPS",
	"site_no_redirect":    "{domain} does not redirect to HTTPS",
	"site_cert_expired":   "the certificate of {domain} has expired",
	"site_cert_expiring":  "the certificate of {domain} expires in {days} days",
	"site_exposed":        "{domain} serves {files}",
	"site_world_writable": "{domain} has {count} world-writable files or folders",
	"php_display_errors":  "PHP {version} shows errors to visitors",
	"php_expose":          "PHP {version} announces its version",
	"wp_updates":          "WordPress updates are waiting on {domain}",
	"wp_debug":            "WordPress debug mode is on for {domain}",
}

// Title is a finding's short English text.
func Title(f Finding) string {
	t, ok := titles[f.Check]
	if !ok {
		return f.Check
	}
	for _, m := range []map[string]string{f.Params, f.Extra} {
		for k, v := range m {
			t = strings.ReplaceAll(t, "{"+k+"}", v)
		}
	}
	return t
}
