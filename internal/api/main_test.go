package api

import (
	"os"
	"testing"

	"github.com/nefocp/nefo/internal/auth"
)

// The tests sign in hundreds of times: cheap password hashes keep them fast.
func TestMain(m *testing.M) {
	auth.UseFastHashesForTests()
	os.Exit(m.Run())
}
