package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The example "GET Object" request from the AWS Signature Version 4
// documentation for Amazon S3.
func TestSignV4AWSExample(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://examplebucket.s3.amazonaws.com/test.txt", nil)
	req.Header.Set("Range", "bytes=0-9")
	signV4(req, "AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "us-east-1", "s3",
		emptySHA256, time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC))
	want := "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request, " +
		"SignedHeaders=host;range;x-amz-content-sha256;x-amz-date, " +
		"Signature=f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41"
	if got := req.Header.Get("Authorization"); got != want {
		t.Fatalf("Authorization =\n%s\nwant\n%s", got, want)
	}
}

func TestS3ClientAgainstFakeServer(t *testing.T) {
	objects := map[string]string{}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=AK/") {
			w.WriteHeader(http.StatusForbidden)
			io.WriteString(w, `<Error><Code>AccessDenied</Code><Message>bad signature</Message></Error>`)
			return
		}
		switch r.Method {
		case http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			sum := sha256.Sum256(b)
			if hex.EncodeToString(sum[:]) != r.Header.Get("X-Amz-Content-Sha256") {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			objects[r.URL.Path] = string(b)
		case http.MethodGet:
			v, ok := objects[r.URL.Path]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				io.WriteString(w, `<Error><Code>NoSuchKey</Code><Message>missing</Message></Error>`)
				return
			}
			io.WriteString(w, v)
		case http.MethodDelete:
			delete(objects, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	cfg := S3Config{Endpoint: srv.URL, Region: "auto", Bucket: "bkt", Prefix: "/panel/", AccessKey: "AK", SecretKey: "SK"}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	c := newS3(cfg)
	c.http = srv.Client()
	ctx := context.Background()
	body := "backup data"
	sum := sha256.Sum256([]byte(body))
	if err := c.Put(ctx, "site/example.com/a.tar.gz", strings.NewReader(body), int64(len(body)), hex.EncodeToString(sum[:])); err != nil {
		t.Fatal(err)
	}
	if _, ok := objects["/bkt/panel/site/example.com/a.tar.gz"]; !ok {
		t.Fatalf("stored keys: %v", objects)
	}
	var got strings.Builder
	if err := c.Get(ctx, "site/example.com/a.tar.gz", &got); err != nil || got.String() != body {
		t.Fatalf("Get = %q, %v", got.String(), err)
	}
	if err := c.Delete(ctx, "site/example.com/a.tar.gz"); err != nil {
		t.Fatal(err)
	}
	err := c.Get(ctx, "site/example.com/a.tar.gz", io.Discard)
	if err == nil || !strings.Contains(err.Error(), "NoSuchKey") {
		t.Fatalf("Get after delete: %v", err)
	}
}

func TestS3ConfigValidate(t *testing.T) {
	bad := []S3Config{
		{Endpoint: "http://s3.example.com", Bucket: "b", AccessKey: "a", SecretKey: "s"},
		{Endpoint: "https://s3.example.com", Bucket: "b/c", AccessKey: "a", SecretKey: "s"},
		{Endpoint: "https://s3.example.com", Bucket: "b", AccessKey: "a"},
		{Endpoint: "https://s3.example.com", Bucket: "b", AccessKey: "a", SecretKey: "s", Prefix: "../x"},
	}
	for _, c := range bad {
		if c.Validate() == nil {
			t.Errorf("accepted %+v", c)
		}
	}
	ok := S3Config{Endpoint: "https://s3.example.com/", Bucket: "b", AccessKey: "a", SecretKey: "s", Prefix: "x/y/"}
	if err := ok.Validate(); err != nil || ok.Region != "us-east-1" || ok.Prefix != "x/y/" || ok.Endpoint != "https://s3.example.com" {
		t.Fatalf("%+v %v", ok, err)
	}
}
