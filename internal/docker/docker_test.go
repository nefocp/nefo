package docker

import (
	"context"
	"io"
	"strings"
	"testing"
)

type fake struct{ last []string }

func (f *fake) Output(ctx context.Context, name string, args ...string) (string, error) {
	f.last = append([]string{name}, args...)
	return "", nil
}
func (f *fake) Stream(ctx context.Context, w io.Writer, name string, args ...string) error {
	f.last = append([]string{name}, args...)
	return nil
}
func (f *fake) Input(context.Context, []byte, string, ...string) (string, error) { return "", nil }
func (f *fake) Pipe(context.Context, io.Reader, io.Writer, string, ...string) error {
	return nil
}

func TestRunPublishesOnLocalhostOnly(t *testing.T) {
	f := &fake{}
	m := New(f)
	err := m.Run(context.Background(), RunRequest{Image: "ghcr.io/org/app:1.2", Name: "app", HostPort: 8081, ContainerPort: 80, Env: []string{"TZ=UTC"}}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(f.last, " ")
	if got != "docker run -d --name app --restart unless-stopped -p 127.0.0.1:8081:80 -e TZ=UTC ghcr.io/org/app:1.2" {
		t.Fatalf("ran %q", got)
	}
	for _, r := range []RunRequest{
		{Image: "nginx; rm -rf /", Name: "x"},
		{Image: "nginx", Name: "--privileged"},
		{Image: "nginx", Name: "x", HostPort: 80, ContainerPort: 80},
		{Image: "nginx", Name: "x", Env: []string{"--volume=/:/host"}},
	} {
		if m.Run(context.Background(), r, io.Discard) == nil {
			t.Errorf("accepted %+v", r)
		}
	}
	if m.Action(context.Background(), "-rf", "remove") == nil || m.Action(context.Background(), "abc", "exec") == nil {
		t.Error("bad action accepted")
	}
}
