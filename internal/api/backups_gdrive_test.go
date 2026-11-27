package api

import (
	"encoding/json"
	"strings"
	"testing"
)

// The Google Drive endpoints check their input before Google is asked;
// the device flow itself is tested against a fake Google in package backup.
func TestDriveStorageEndpoints(t *testing.T) {
	e := newEnv(t)
	e.login(t)

	res, out := e.do(t, "GET", "/api/backups", nil, nil)
	if res.StatusCode != 200 || out["gdrive"] != nil || !strings.HasPrefix(out["gdrive_default_folder"].(string), "Nefo backups/") ||
		out["gdrive_builtin_client"] != false {
		t.Fatalf("list: %d %v", res.StatusCode, out)
	}
	res, out = e.do(t, "GET", "/api/backups/gdrive/connect", nil, nil)
	if res.StatusCode != 200 || out["state"] != "none" {
		t.Fatalf("status: %d %v", res.StatusCode, out)
	}
	for body, code := range map[string]string{
		`{"client_id":"nope","client_secret":"x"}`:                                          "gdrive_client_id",
		`{"client_id":"1-a.apps.googleusercontent.com"}`:                                    "gdrive_client_secret",
		`{"client_id":"1-a.apps.googleusercontent.com","client_secret":"x","folder":".."}`:  "gdrive_folder",
		`{"client_id":"1-a.apps.googleusercontent.com","client_secret":"x","encrypt":true}`: "backup_passphrase_missing",
	} {
		var req map[string]any
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			t.Fatal(err)
		}
		res, out = e.do(t, "POST", "/api/backups/gdrive/connect", req, nil)
		if res.StatusCode != 400 || out["error"] != code {
			t.Errorf("%s: %d %v", body, res.StatusCode, out)
		}
	}
	res, out = e.do(t, "PUT", "/api/backups/gdrive", map[string]string{"folder": "x"}, nil)
	if res.StatusCode != 400 || out["error"] != "gdrive_not_connected" {
		t.Fatalf("save without connection: %d %v", res.StatusCode, out)
	}
	if res, _ := e.do(t, "DELETE", "/api/backups/gdrive", nil, nil); res.StatusCode != 200 {
		t.Fatalf("remove: %d", res.StatusCode)
	}
	// S3 and SFTP check the encryption passphrase too, before connecting.
	res, out = e.do(t, "PUT", "/api/backups/storage", map[string]any{"endpoint": "https://s3.example.com", "bucket": "b",
		"access_key": "a", "secret_key": "s", "encrypt": true, "encrypt_passphrase": "short"}, nil)
	if res.StatusCode != 400 || out["error"] != "backup_passphrase_invalid" {
		t.Fatalf("S3 short passphrase: %d %v", res.StatusCode, out)
	}
	// Without a session, nothing is reachable.
	e.client.Jar = nil
	if res, _ := e.do(t, "POST", "/api/backups/gdrive/connect", map[string]string{}, nil); res.StatusCode != 401 {
		t.Fatalf("no session: %d", res.StatusCode)
	}
}
