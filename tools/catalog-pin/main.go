// Command catalog-pin pins every image of the app store catalog to the
// multi-arch index digest its tag points to now, and checks that the index
// has linux/amd64 and linux/arm64 images. Run it before a release, and
// before signing a remote catalog, so the digests in the catalog are the
// images that were tested:
//
//	go run ./tools/catalog-pin                 # rewrite internal/appstore/catalog.json
//	go run ./tools/catalog-pin -check          # only report digests that moved
//	go run ./tools/catalog-pin -bump           # also raise catalog_version
//
// It reads the registries anonymously (Docker Hub and ghcr.io) and changes
// nothing but the digest fields (and catalog_version with -bump).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/appstore"
)

func main() {
	path := flag.String("catalog", "internal/appstore/catalog.json", "catalog file")
	check := flag.Bool("check", false, "report changes without writing")
	bump := flag.Bool("bump", false, "raise catalog_version (to today's YYYYMMDD01 or by one)")
	flag.Parse()
	if err := run(*path, *check, *bump); err != nil {
		fmt.Fprintln(os.Stderr, "catalog-pin:", err)
		os.Exit(1)
	}
}

func run(path string, check, bump bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	c, err := appstore.Parse(data)
	if err != nil {
		return err
	}
	ctx := context.Background()
	changed := 0
	for _, a := range c.Apps {
		if a.Docker == nil {
			continue
		}
		for name, s := range a.Docker.Services {
			digest, err := indexDigest(ctx, s.Image)
			if err != nil {
				return fmt.Errorf("%s/%s %s: %w", a.ID, name, s.Image, err)
			}
			if digest != s.Digest {
				fmt.Printf("%-14s %-8s %s: %s -> %s\n", a.ID, name, s.Image, short(s.Digest), short(digest))
				s.Digest = digest
				changed++
			}
		}
	}
	if changed == 0 {
		fmt.Println("all digests are current")
	}
	if check {
		if changed > 0 {
			return fmt.Errorf("%d digests moved", changed)
		}
		return nil
	}
	if bump {
		today := time.Now().UTC()
		v := int64(today.Year()*1000000+int(today.Month())*10000+today.Day()*100) + 1
		if v <= c.CatalogVersion {
			v = c.CatalogVersion + 1
		}
		c.CatalogVersion, c.Generated = v, today.Format("2006-01-02")
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(c); err != nil {
		return err
	}
	if _, err := appstore.Parse(buf.Bytes()); err != nil {
		return fmt.Errorf("the result does not pass the catalog checks: %w", err)
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func short(d string) string {
	if len(d) > 19 {
		return d[:19]
	}
	if d == "" {
		return "(none)"
	}
	return d
}

var client = &http.Client{Timeout: time.Minute}

// indexDigest returns the digest of the manifest list (index) an image tag
// points to, after checking it covers linux/amd64 and linux/arm64.
func indexDigest(ctx context.Context, image string) (string, error) {
	name, tag, ok := strings.Cut(image, ":")
	if !ok {
		return "", errors.New("no tag")
	}
	registry := "registry-1.docker.io"
	if i := strings.Index(name, "/"); i > 0 && strings.Contains(name[:i], ".") {
		registry, name = name[:i], name[i+1:]
	} else if !strings.Contains(name, "/") {
		name = "library/" + name
	}
	token, err := pullToken(ctx, registry, name)
	if err != nil {
		return "", err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+registry+"/v2/"+name+"/manifests/"+tag, nil)
	req.Header.Set("Accept", "application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var idx struct {
		Manifests []struct {
			Platform struct{ OS, Architecture string }
		}
	}
	if err := json.NewDecoder(resp.Body).Decode(&idx); err != nil {
		return "", err
	}
	have := map[string]bool{}
	for _, m := range idx.Manifests {
		have[m.Platform.OS+"/"+m.Platform.Architecture] = true
	}
	for _, p := range []string{"linux/amd64", "linux/arm64"} {
		if !have[p] {
			return "", fmt.Errorf("no %s image", p)
		}
	}
	d := resp.Header.Get("Docker-Content-Digest")
	if !strings.HasPrefix(d, "sha256:") || len(d) != 71 {
		return "", errors.New("no index digest in the answer")
	}
	return d, nil
}

func pullToken(ctx context.Context, registry, name string) (string, error) {
	var u string
	switch registry {
	case "registry-1.docker.io":
		u = "https://auth.docker.io/token?service=registry.docker.io&scope=" + url.QueryEscape("repository:"+name+":pull")
	case "ghcr.io":
		u = "https://ghcr.io/token?scope=" + url.QueryEscape("repository:"+name+":pull")
	default:
		return "", nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var t struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return "", err
	}
	return t.Token, nil
}
