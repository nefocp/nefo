package update

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A key and signature made with OpenSSL 3.0 ("openssl pkeyutl -sign -rawin"),
// as the release workflow and install.sh use it.
const opensslPub = `-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEAw7mj1gqL0Sq/pn/zF5uxh539Qzt4F74UsQSafRxCOJM=
-----END PUBLIC KEY-----`
const opensslSig = "Um7S9yEpt3B5I7Z+J6JZmOKDbmhSLD64gFBKCrRrx2yhjbFUTahOYqGIp34Evw7Fxy4gZoGA88AQbJc0l69VBw=="

func withKey(t *testing.T, pemKey []byte) {
	old := releasePub
	releasePub = pemKey
	t.Cleanup(func() { releasePub = old })
}

func TestOpenSSLSignatureVerifies(t *testing.T) {
	withKey(t, []byte(opensslPub))
	pub, err := signingKey()
	if err != nil || pub == nil {
		t.Fatalf("signingKey: %v", err)
	}
	sig, _ := base64.StdEncoding.DecodeString(opensslSig)
	if err := checkSignature(pub, []byte("abc123  panel-linux-amd64\n"), sig); err != nil {
		t.Fatal(err)
	}
	if checkSignature(pub, []byte("abc124  panel-linux-amd64\n"), sig) == nil {
		t.Fatal("tampered data accepted")
	}
}

type fakeRun struct{}

func (fakeRun) Output(ctx context.Context, name string, args ...string) (string, error) {
	return "Nefo 9.9.9", nil
}
func (fakeRun) Stream(context.Context, io.Writer, string, ...string) error          { return nil }
func (fakeRun) Input(context.Context, []byte, string, ...string) (string, error)    { return "", nil }
func (fakeRun) Pipe(context.Context, io.Reader, io.Writer, string, ...string) error { return nil }

type nopLog struct{}

func (nopLog) Logf(string, ...any) {}

func TestApplyRequiresSignature(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	der, _ := x509.MarshalPKIXPublicKey(pub)
	withKey(t, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))

	binary := []byte("new panel binary")
	sum := sha256.Sum256(binary)
	sums := hex.EncodeToString(sum[:]) + "  nefo-linux-" + goarch() + "\n"
	files := map[string][]byte{
		"VERSION":                []byte("9.9.9\n"),
		"checksums.txt":          []byte(sums),
		"checksums.txt.sig":      ed25519.Sign(priv, []byte(sums)),
		"nefo-linux-" + goarch(): binary,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, ok := files[strings.TrimPrefix(r.URL.Path, "/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(b)
	}))
	defer srv.Close()

	dir := t.TempDir()
	bin := filepath.Join(dir, "panel")
	os.WriteFile(bin, []byte("old"), 0o755)
	u, err := New(srv.URL, bin, fakeRun{})
	if err != nil {
		t.Fatal(err)
	}

	// A signature over different content is refused and nothing changes.
	files["checksums.txt.sig"] = ed25519.Sign(priv, []byte("something else"))
	if _, err := u.Apply(context.Background(), nopLog{}); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("bad signature: err = %v", err)
	}
	if b, _ := os.ReadFile(bin); string(b) != "old" {
		t.Fatal("binary replaced despite a bad signature")
	}
	delete(files, "checksums.txt.sig")
	if _, err := u.Apply(context.Background(), nopLog{}); err == nil {
		t.Fatal("missing signature accepted")
	}

	files["checksums.txt.sig"] = ed25519.Sign(priv, []byte(sums))
	if v, err := u.Apply(context.Background(), nopLog{}); err != nil || v != "9.9.9" {
		t.Fatalf("Apply = %q, %v", v, err)
	}
	if b, _ := os.ReadFile(bin); string(b) != string(binary) {
		t.Fatal("binary not replaced")
	}
}
