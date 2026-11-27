package procs

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSSHJournal(t *testing.T) {
	m, r := newTest()
	// OpenSSH 10 (sshd-session) on Debian 13: the same messages.
	r.outputs["journalctl"] = `{"__CURSOR":"s=a;i=1","__REALTIME_TIMESTAMP":"1795554000000000","MESSAGE":"Server listening on 0.0.0.0 port 22.","_PID":"1"}
{"__CURSOR":"s=a;i=2","__REALTIME_TIMESTAMP":"1795554100000000","MESSAGE":"Invalid user sb from 109.160.32.138 port 38148","_PID":"2346"}
{"__CURSOR":"s=a;i=3","__REALTIME_TIMESTAMP":"1795554104000000","MESSAGE":"Failed password for invalid user sb from 109.160.32.138 port 38148 ssh2","_PID":"2346"}
{"__CURSOR":"s=a;i=4","__REALTIME_TIMESTAMP":"1795554200000000","MESSAGE":"Accepted publickey for root from 203.0.113.35 port 55671 ssh2: ED25519 SHA256:abc","_PID":"7"}
{"__CURSOR":"s=a;i=5","__REALTIME_TIMESTAMP":"1795554201000000","MESSAGE":"pam_unix(sshd:session): session opened for user root(uid=0) by root(uid=0)","_PID":"7"}
`
	list, cur, err := m.SSHJournal(context.Background(), "", time.Unix(1795553000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if cur != "s=a;i=5" {
		t.Fatalf("cursor %q", cur)
	}
	got := []string{}
	for _, l := range list {
		got = append(got, l.Result+" "+l.User+" "+l.IP+" "+l.Method)
	}
	want := "invalid_user sb 109.160.32.138 password\naccepted root 203.0.113.35 publickey"
	if strings.Join(got, "\n") != want {
		t.Fatalf("got\n%s", strings.Join(got, "\n"))
	}
	if !strings.Contains(r.calls[0], "--since=@1795553000") || !strings.Contains(r.calls[0], "-u ssh.service -u sshd.service") || strings.Contains(r.calls[0], "-g") {
		t.Fatalf("call %s", r.calls[0])
	}

	r.calls = nil
	r.outputs["journalctl"] = ""
	list, cur, err = m.SSHJournal(context.Background(), "s=a;i=5", time.Time{})
	if err != nil || len(list) != 0 || cur != "" {
		t.Fatalf("nothing new: %v %q %v", list, cur, err)
	}
	if !strings.Contains(r.calls[0], "--after-cursor=s=a;i=5") || strings.Contains(r.calls[0], "--since") {
		t.Fatalf("call %s", r.calls[0])
	}
}

func TestSSHSignIns(t *testing.T) {
	m, r := newTest()
	r.outputs["journalctl"] = `{"__CURSOR":"a","__REALTIME_TIMESTAMP":"1795554000000000","MESSAGE":"Failed password for root from 192.0.2.1 port 1 ssh2","_PID":"1"}
{"__CURSOR":"b","__REALTIME_TIMESTAMP":"1795554100000000","MESSAGE":"Accepted publickey for root from 203.0.113.35 port 55671 ssh2: ED25519 SHA256:abc","_PID":"7"}
`
	list, err := m.SSHSignIns(context.Background())
	if err != nil || len(list) != 1 || list[0].IP != "203.0.113.35" {
		t.Fatalf("%v %v", list, err)
	}
	if !strings.Contains(r.calls[0], "-g ^Accepted") || !strings.Contains(r.calls[0], "--since=-30d") {
		t.Fatalf("call %s", r.calls[0])
	}
}
