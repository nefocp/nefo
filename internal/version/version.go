// Package version holds build information and the product name.
package version

// AppName is the product name shown in the CLI, the 2FA app and the UI.
// Change it here (and in web/src/locales/*.json) when you pick a final name.
const AppName = "Nefo"

// Set at build time:
//
//	go build -ldflags "-X github.com/nefocp/nefo/internal/version.Version=v0.1.0"
var (
	Version = "dev"
	Commit  = "none"
)

// Repo is the GitHub repository releases are published to (owner/name).
var Repo = "nefocp/nefo"
