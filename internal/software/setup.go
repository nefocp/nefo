package software

import "os"

// PHPInstalled reports whether PHP-FPM version v is installed, as the
// Software page shows it.
func PHPInstalled(v string) bool { return phpInstalled(v) }

// PHPRepository names where the PHP versions other than the distribution's
// come from on this server: "packages.sury.org" on Ubuntu and Debian, or
// "ppa:ondrej/php" on an Ubuntu server that still has the frozen PPA (see
// MigratePHPRepository), "" elsewhere.
func PHPRepository() string {
	b, _ := os.ReadFile(osReleaseFile)
	switch osField(b, "ID") {
	case "ubuntu":
		if sury, ppa, _ := phpSources(); !sury && len(ppa) > 0 {
			return "ppa:ondrej/php"
		}
		return "packages.sury.org"
	case "debian":
		return "packages.sury.org"
	}
	return ""
}
