package backup

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/version"
)

// S3Config describes S3-compatible storage: Amazon S3, Cloudflare R2,
// Backblaze B2, Wasabi, MinIO…
type S3Config struct {
	Endpoint  string `json:"endpoint"` // such as https://s3.eu-central-1.amazonaws.com
	Region    string `json:"region"`   // such as eu-central-1, or "auto" for R2
	Bucket    string `json:"bucket"`
	Prefix    string `json:"prefix"` // optional folder inside the bucket
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key,omitempty"`
}

// Validate checks the settings.
func (c *S3Config) Validate() error {
	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.Path != "" && u.Path != "/" {
		return usererr.New("invalid_s3_endpoint")
	}
	c.Endpoint = "https://" + u.Host
	if c.Region == "" {
		c.Region = "us-east-1"
	}
	for _, v := range []string{c.Region, c.Bucket, c.AccessKey} {
		if v == "" || len(v) > 128 || strings.ContainsAny(v, " /\\?#\n\r\t") {
			return usererr.New("invalid_s3_settings")
		}
	}
	if c.SecretKey == "" || len(c.SecretKey) > 256 {
		return usererr.New("invalid_s3_settings")
	}
	c.Prefix = strings.Trim(c.Prefix, "/")
	if c.Prefix != "" {
		if strings.Contains(c.Prefix, "..") || strings.ContainsAny(c.Prefix, "\\?#\n\r") || len(c.Prefix) > 200 {
			return usererr.New("invalid_s3_settings")
		}
		c.Prefix += "/"
	}
	return nil
}

type s3Client struct {
	cfg  S3Config
	http *http.Client
	now  func() time.Time
}

func newS3(cfg S3Config) *s3Client {
	return &s3Client{cfg: cfg, http: &http.Client{Timeout: 6 * time.Hour}, now: time.Now}
}

// Describe names an object for job logs.
func (c *s3Client) Describe(key string) string {
	return "S3 " + c.cfg.Bucket + "/" + c.cfg.Prefix + key
}

// Close does nothing: requests do not share a connection that needs closing.
func (c *s3Client) Close() error { return nil }

// objectURL uses path-style addressing (https://endpoint/bucket/key), which
// every S3-compatible service accepts.
func (c *s3Client) objectURL(key string) string {
	return c.cfg.Endpoint + "/" + uriEncode(c.cfg.Bucket, false) + "/" + uriEncode(c.cfg.Prefix+key, false)
}

const emptySHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func (c *s3Client) do(ctx context.Context, method, key string, body io.Reader, size int64, payloadHash string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.objectURL(key), body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.ContentLength = size
	}
	req.Header.Set("User-Agent", version.AppName+"/"+version.Version)
	signV4(req, c.cfg.AccessKey, c.cfg.SecretKey, c.cfg.Region, "s3", payloadHash, c.now())
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		defer resp.Body.Close()
		return nil, s3Error(resp)
	}
	return resp, nil
}

// Put uploads a file whose SHA-256 is known.
func (c *s3Client) Put(ctx context.Context, key string, r io.Reader, size int64, sha256hex string) error {
	if size > 5<<30 {
		return fmt.Errorf("the backup is larger than 5 GB, the limit of a single upload")
	}
	resp, err := c.do(ctx, http.MethodPut, key, r, size, sha256hex)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// Get downloads an object into w.
func (c *s3Client) Get(ctx context.Context, key string, w io.Writer) error {
	resp, err := c.do(ctx, http.MethodGet, key, nil, 0, emptySHA256)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(w, resp.Body)
	return err
}

// Delete removes an object.
func (c *s3Client) Delete(ctx context.Context, key string) error {
	resp, err := c.do(ctx, http.MethodDelete, key, nil, 0, emptySHA256)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func s3Error(resp *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var e struct {
		Code    string `xml:"Code"`
		Message string `xml:"Message"`
	}
	if xml.Unmarshal(b, &e) == nil && e.Code != "" {
		return fmt.Errorf("storage answered HTTP %d: %s: %s", resp.StatusCode, e.Code, e.Message)
	}
	return fmt.Errorf("storage answered HTTP %d", resp.StatusCode)
}

// signV4 adds an AWS Signature Version 4 Authorization header. It signs the
// host, every x-amz-* header and Range, Content-Type and Content-MD5 if set.
func signV4(req *http.Request, accessKey, secretKey, region, service, payloadHash string, t time.Time) {
	t = t.UTC()
	amzDate := t.Format("20060102T150405Z")
	date := t.Format("20060102")
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	headers := map[string]string{"host": req.URL.Host}
	for name, vals := range req.Header {
		l := strings.ToLower(name)
		if strings.HasPrefix(l, "x-amz-") || l == "range" || l == "content-type" || l == "content-md5" {
			headers[l] = strings.TrimSpace(strings.Join(vals, ","))
		}
	}
	names := make([]string, 0, len(headers))
	for n := range headers {
		names = append(names, n)
	}
	sort.Strings(names)
	var canonHeaders strings.Builder
	for _, n := range names {
		canonHeaders.WriteString(n + ":" + headers[n] + "\n")
	}
	signed := strings.Join(names, ";")

	path := req.URL.EscapedPath()
	if path == "" {
		path = "/"
	}
	canonical := strings.Join([]string{req.Method, path, canonicalQuery(req.URL.Query()),
		canonHeaders.String(), signed, payloadHash}, "\n")
	scope := date + "/" + region + "/" + service + "/aws4_request"
	sum := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(sum[:])

	key := hmacSHA256([]byte("AWS4"+secretKey), date)
	key = hmacSHA256(key, region)
	key = hmacSHA256(key, service)
	key = hmacSHA256(key, "aws4_request")
	sig := hex.EncodeToString(hmacSHA256(key, toSign))
	req.Header.Set("Authorization", fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey, scope, signed, sig))
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func canonicalQuery(q url.Values) string {
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		vals := append([]string(nil), q[k]...)
		sort.Strings(vals)
		for _, v := range vals {
			parts = append(parts, uriEncode(k, true)+"="+uriEncode(v, true))
		}
	}
	return strings.Join(parts, "&")
}

// uriEncode percent-encodes everything except unreserved characters, and
// slashes unless encodeSlash is set, as SigV4 requires.
func uriEncode(s string, encodeSlash bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_', c == '.', c == '~':
			b.WriteByte(c)
		case c == '/' && !encodeSlash:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
