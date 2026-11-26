package files

import (
	"os/user"
	"strconv"
)

// nameCache looks up user and group names once per listing.
type nameCache struct {
	users, groups map[int]string
}

func newNameCache() *nameCache {
	return &nameCache{users: map[int]string{}, groups: map[int]string{}}
}

func (c *nameCache) user(id int) string {
	if id < 0 {
		return ""
	}
	if n, ok := c.users[id]; ok {
		return n
	}
	n := strconv.Itoa(id)
	if u, err := user.LookupId(n); err == nil {
		n = u.Username
	}
	c.users[id] = n
	return n
}

func (c *nameCache) group(id int) string {
	if id < 0 {
		return ""
	}
	if n, ok := c.groups[id]; ok {
		return n
	}
	n := strconv.Itoa(id)
	if g, err := user.LookupGroupId(n); err == nil {
		n = g.Name
	}
	c.groups[id] = n
	return n
}
