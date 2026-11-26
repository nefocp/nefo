package hosting

import (
	"context"
	"fmt"
	"os/user"
	"strings"

	"github.com/nefocp/nefo/internal/validate"
)

// userBase turns a domain into a Linux account name: "shop.example.com"
// becomes "shop_example_com". Names are cut to leave room for a "_NN" suffix.
func userBase(domain string) string {
	var b strings.Builder
	for _, r := range domain {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		case r == '.':
			b.WriteByte('_')
		}
	}
	s := b.String()
	if s == "" || (s[0] >= '0' && s[0] <= '9') || s[0] == '-' {
		s = "w" + s
	}
	if len(s) > 28 {
		s = s[:28]
	}
	return strings.TrimRight(s, "_-")
}

// pickName returns base, or base_2, base_3… if the name is taken.
func pickName(base string, taken func(string) bool) (string, error) {
	for i := 1; i < 100; i++ {
		name := base
		if i > 1 {
			name = fmt.Sprintf("%s_%d", base, i)
		}
		if validate.SystemUsername(name) != nil {
			return "", fmt.Errorf("cannot make a valid user name from %q", base)
		}
		if !taken(name) {
			return name, nil
		}
	}
	return "", fmt.Errorf("no free user name for %q", base)
}

// pickUser chooses an unused Linux account name for a new site.
func (m *Manager) pickUser(ctx context.Context, domain string) (string, error) {
	return pickName(userBase(domain), func(name string) bool {
		if _, err := user.Lookup(name); err == nil {
			return true
		}
		if _, err := user.LookupGroup(name); err == nil {
			return true
		}
		taken, err := m.store.SysUserTaken(ctx, name)
		return err != nil || taken
	})
}
