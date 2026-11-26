package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/usererr"
)

// Image is one local image.
type Image struct {
	ID        string     `json:"id"`  // short ID
	Ref       string     `json:"ref"` // what to remove: repo:tag, or the full ID for untagged images
	Repo      string     `json:"repo"`
	Tag       string     `json:"tag"`
	Size      string     `json:"size"`
	Created   string     `json:"created"` // as Docker says it, such as "2 weeks ago"
	CreatedAt *time.Time `json:"created_at,omitempty"`
	InUse     bool       `json:"in_use"`
	UsedBy    []string   `json:"used_by"` // names of the containers using it, running or not
}

// imageRefRe allows the references Docker prints: repo[:tag], a registry
// with a port, repo@digest and sha256:ID. It never starts with "-", so a
// reference cannot be read as an option.
var imageRefRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/:@-]{0,254}$`)

// stdout runs docker and returns only what it printed on standard output.
// Warnings on standard error would otherwise break the JSON.
func (m *Manager) stdout(ctx context.Context, args ...string) (string, error) {
	var buf bytes.Buffer
	err := m.run.Pipe(ctx, nil, &buf, "docker", args...)
	return buf.String(), err
}

// eachJSON calls fn for each object of a JSON array or of a list with one
// JSON object per line. Docker and Compose print either, depending on the
// command and the version.
func eachJSON(out string, fn func([]byte) error) error {
	out = strings.TrimSpace(out)
	if strings.HasPrefix(out, "[") {
		var list []json.RawMessage
		if err := json.Unmarshal([]byte(out), &list); err != nil {
			return err
		}
		for _, b := range list {
			if err := fn(b); err != nil {
				return err
			}
		}
		return nil
	}
	return eachLine(out, fn)
}

func shortID(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	return id[:min(12, len(id))]
}

// Images lists local images and the containers that use them.
func (m *Manager) Images(ctx context.Context) ([]Image, error) {
	out, err := m.stdout(ctx, "images", "--no-trunc", "--format", "{{json .}}")
	if err != nil {
		return nil, err
	}
	used, err := m.imageUsers(ctx)
	if err != nil {
		return nil, err
	}
	list := []Image{}
	err = eachLine(out, func(b []byte) error {
		var i struct{ ID, Repository, Tag, Size, CreatedSince, CreatedAt string }
		if err := json.Unmarshal(b, &i); err != nil {
			return err
		}
		img := Image{ID: shortID(i.ID), Ref: i.ID, Repo: i.Repository, Tag: i.Tag, Size: i.Size,
			Created: i.CreatedSince, UsedBy: used[i.ID]}
		// Docker prints Go's default time format, such as
		// "2026-10-01 08:00:00 +0000 UTC".
		if at, err := time.Parse("2006-01-02 15:04:05 -0700 MST", i.CreatedAt); err == nil {
			at = at.UTC()
			img.CreatedAt = &at
		}
		if img.UsedBy == nil {
			img.UsedBy = []string{}
		}
		img.InUse = len(img.UsedBy) > 0
		if i.Repository != "" && i.Repository != "<none>" && i.Tag != "" && i.Tag != "<none>" {
			img.Ref = i.Repository + ":" + i.Tag
		}
		list = append(list, img)
		return nil
	})
	return list, err
}

// imageUsers maps full image IDs to the names of the containers made from
// them, including stopped ones.
func (m *Manager) imageUsers(ctx context.Context) (map[string][]string, error) {
	out, err := m.stdout(ctx, "ps", "-a", "-q", "--no-trunc")
	if err != nil {
		return nil, err
	}
	ids := strings.Fields(out)
	used := map[string][]string{}
	if len(ids) == 0 {
		return used, nil
	}
	out, err = m.stdout(ctx, append([]string{"container", "inspect", "--format", "{{.Image}} {{.Name}}"}, ids...)...)
	// A container removed in the meantime makes inspect fail, but it still
	// prints the others.
	if err != nil && strings.TrimSpace(out) == "" {
		return nil, err
	}
	for _, l := range strings.Split(out, "\n") {
		img, name, ok := strings.Cut(strings.TrimSpace(l), " ")
		if ok && strings.HasPrefix(img, "sha256:") {
			used[img] = append(used[img], strings.TrimPrefix(name, "/"))
		}
	}
	for _, names := range used {
		slices.Sort(names)
	}
	return used, nil
}

// RemoveImage removes an image or one of its tags. An image that a
// container uses is refused unless force is set.
func (m *Manager) RemoveImage(ctx context.Context, ref string, force bool) error {
	if !imageRefRe.MatchString(ref) {
		return usererr.New("invalid_image", ref)
	}
	if !force {
		out, err := m.stdout(ctx, "image", "inspect", "--format", "{{.Id}}", ref)
		if err != nil {
			return usererr.New("image_not_found", ref)
		}
		used, err := m.imageUsers(ctx)
		if err != nil {
			return err
		}
		if names := used[strings.TrimSpace(out)]; len(names) > 0 {
			return usererr.New("image_in_use", ref+" ("+strings.Join(names, ", ")+")")
		}
	}
	args := []string{"image", "rm"}
	if force {
		args = append(args, "--force")
	}
	_, err := m.run.Output(ctx, "docker", append(args, ref)...)
	return err
}

// PruneImages removes every image that no container uses.
func (m *Manager) PruneImages(ctx context.Context, w io.Writer) error {
	return m.run.Stream(ctx, w, "docker", "image", "prune", "--all", "--force")
}
