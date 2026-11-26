package database

import (
	"os"
	"testing"
)

// The tests see a server with no database server of its own, whatever the
// machine they run on has installed (GitHub's runners have MySQL): a test
// that needs MySQL sets the Manager's detect hook.
func TestMain(m *testing.M) {
	hostEngine = func() Engine { return "" }
	os.Exit(m.Run())
}
