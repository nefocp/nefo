package api

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// wpDBBase turns a domain into a database name such as "wp_shop_example_com".
func wpDBBase(domain string) string {
	var b strings.Builder
	b.WriteString("wp_")
	for _, r := range domain {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == '-':
			b.WriteByte('_')
		}
	}
	s := b.String()
	if len(s) > 28 {
		s = s[:28]
	}
	return strings.TrimRight(s, "_")
}

// createDBWithFreeName creates base, or base_2, base_3… if the name is taken.
func (s *Server) createDBWithFreeName(ctx context.Context, base string, siteID int64) (*store.Database, error) {
	for i := 1; i <= 20; i++ {
		name := base
		if i > 1 {
			name = fmt.Sprintf("%s_%d", base, i)
		}
		d, err := s.db.Create(ctx, name, "", siteID)
		var ue *usererr.Error
		if errors.As(err, &ue) && ue.Code == "db_exists" {
			continue
		}
		return d, err
	}
	return nil, fmt.Errorf("no free database name for %s", base)
}
