package goproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func client(module string, env map[string]string) *Client {
	c := New(module)
	c.getenv = func(k string) string { return env[k] }
	return c
}

func TestLatest(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.Write([]byte(`{"Version":"v0.4.0","Time":"2026-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()
	c := client("github.com/Arx/ai-loops", map[string]string{"GOPROXY": "off|" + srv.URL + "/,direct"})
	v, err := c.Latest(context.Background())
	if err != nil || v != "v0.4.0" {
		t.Fatalf("v=%q err=%v", v, err)
	}
	if gotPath != "/github.com/!arx/ai-loops/@latest" {
		t.Fatalf("path=%q", gotPath)
	}
}

func TestLatestErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/m/@latest":
			http.NotFound(w, r)
		case "/junk/@latest":
			w.Write([]byte("<html>"))
		case "/empty/@latest":
			w.Write([]byte(`{}`))
		case "/slow/@latest":
			time.Sleep(2 * time.Second)
		}
	}))
	defer srv.Close()
	for _, c := range []struct {
		name, mod string
		env       map[string]string
		want      string
	}{
		{"404", "m", map[string]string{"GOPROXY": srv.URL}, "404"},
		{"junk", "junk", map[string]string{"GOPROXY": srv.URL}, "bad response"},
		{"empty", "empty", map[string]string{"GOPROXY": srv.URL}, "empty version"},
		{"off", "m", map[string]string{"GOPROXY": "off"}, "no http"},
		{"direct", "m", map[string]string{"GOPROXY": "direct"}, "no http"},
		{"private", "m", map[string]string{"GOPROXY": srv.URL, "GOPRIVATE": "m"}, "private"},
		{"unreachable", "m", map[string]string{"GOPROXY": "http://127.0.0.1:1"}, ""},
		{"bad url", "m", map[string]string{"GOPROXY": "http://[::1"}, ""},
		{"timeout", "slow", map[string]string{"GOPROXY": srv.URL}, ""},
	} {
		start := time.Now()
		_, err := client(c.mod, c.env).Latest(context.Background())
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err=%v want %q", c.name, err, c.want)
		}
		if c.name == "timeout" && time.Since(start) > 1900*time.Millisecond {
			t.Errorf("timeout not enforced: %v", time.Since(start))
		}
	}
}

func TestDefaultProxy(t *testing.T) {
	p, err := client("m", nil).proxy()
	if err != nil || p != "https://proxy.golang.org" {
		t.Fatalf("p=%q err=%v", p, err)
	}
}

func TestPrivate(t *testing.T) {
	m := "github.com/arxdsilva/ai-loops"
	for _, c := range []struct {
		env  map[string]string
		want bool
	}{
		{nil, false},
		{map[string]string{"GOPRIVATE": "github.com/arxdsilva"}, true},
		{map[string]string{"GOPRIVATE": "github.com/arxdsilva/ai-loops"}, true},
		{map[string]string{"GONOPROXY": "*.com"}, true},
		{map[string]string{"GONOPROXY": "github.*/arxdsilva"}, true},
		{map[string]string{"GOPRIVATE": "gitlab.com/x, github.com/arxdsilva"}, true},
		{map[string]string{"GOPRIVATE": "github.com/other"}, false},
		{map[string]string{"GOPRIVATE": "github.com/arxdsilva/ai-loops/sub/deep"}, false},
		{map[string]string{"GOPRIVATE": "github.co"}, false},
	} {
		if got := client(m, c.env).private(); got != c.want {
			t.Errorf("%v: got %v want %v", c.env, got, c.want)
		}
	}
}
