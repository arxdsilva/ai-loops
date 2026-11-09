package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/adapters/outbound/portfs"
	"github.com/arxdsilva/ai-loops/internal/application/release"
)

type fakeRel struct {
	res  release.Result
	err  error
	got  release.Options
	call int
}

func (f *fakeRel) Release(_ context.Context, o release.Options) (release.Result, error) {
	f.got, f.call = o, f.call+1
	return f.res, f.err
}

func TestReleaseCmd(t *testing.T) {
	ok := release.Result{Current: "0.3.0", Next: "0.4.0", Level: "minor", Summary: "3 feat, 2 fix"}
	tests := []struct {
		name    string
		args    []string
		res     release.Result
		err     error
		code    int
		out     string
		level   string
		dry, pu bool
	}{
		{"default auto", nil, ok, nil, 0, "release: 0.3.0 → 0.4.0 (minor: 3 feat, 2 fix)", "auto", false, false},
		{"level flags", []string{"patch", "--dry-run", "--push"}, func() release.Result { r := ok; r.DryRun = true; return r }(), nil, 0, "[dry run]", "patch", true, true},
		{"flags first", []string{"--push", "major"}, ok, nil, 0, "released v0.4.0", "major", false, true},
		{"skipped", nil, release.Result{Skipped: true, Reason: "HEAD is a release commit"}, nil, 0, "release: skipped: HEAD is a release commit", "auto", false, false},
		{"error", nil, release.Result{}, errors.New("dirty"), 1, "", "auto", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeRel{res: tt.res, err: tt.err}
			var o, e bytes.Buffer
			a := &App{Release: f, Out: &o, Err: &e}
			if c := a.Run(context.Background(), append([]string{"release"}, tt.args...)); c != tt.code {
				t.Fatalf("exit %d: %s", c, e.String())
			}
			if !strings.Contains(o.String(), tt.out) || f.got.Level != tt.level || f.got.DryRun != tt.dry || f.got.Push != tt.pu {
				t.Fatalf("%q %+v", o.String(), f.got)
			}
			if tt.err != nil && !strings.Contains(e.String(), "dirty") {
				t.Fatalf("err %q", e.String())
			}
		})
	}
}

func TestReleaseCmdUsage(t *testing.T) {
	for _, args := range [][]string{{"huge"}, {"patch", "minor"}, {"--bogus"}} {
		f := &fakeRel{}
		var o, e bytes.Buffer
		a := &App{Release: f, Out: &o, Err: &e}
		if c := a.Run(context.Background(), append([]string{"release"}, args...)); c != 2 || f.call != 0 {
			t.Errorf("%v: exit %d", args, c)
		}
	}
	var o, e bytes.Buffer
	a := &App{Out: &o, Err: &e}
	if c := a.Run(context.Background(), []string{"release"}); c != 1 || !strings.Contains(e.String(), "unavailable") {
		t.Fatalf("nil releaser: %d %q", c, e.String())
	}
}

func TestRegeneratePackagesReadsDisk(t *testing.T) {
	root := t.TempDir()
	for name, f := range portSource() {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, f.Data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A version only on disk must reach the generated packages.
	pj := filepath.Join(root, ".claude-plugin/plugin.json")
	data, _ := os.ReadFile(pj)
	data = bytes.Replace(data, []byte(`"version":"0.3.0"`), []byte(`"version":"7.7.7"`), 1)
	if err := os.WriteFile(pj, data, 0o644); err != nil {
		t.Fatal(err)
	}
	var o, e bytes.Buffer
	if err := RegeneratePackages(root, portfs.New(), &o, &e); err != nil {
		t.Fatalf("%v: %s", err, e.String())
	}
	if _, err := os.Stat(filepath.Join(root, "platforms", "codex")); err != nil {
		t.Fatal(err)
	}
	found := false
	_ = filepath.WalkDir(filepath.Join(root, "platforms"), func(p string, d os.DirEntry, _ error) error {
		if d != nil && !d.IsDir() {
			if b, _ := os.ReadFile(p); bytes.Contains(b, []byte("7.7.7")) {
				found = true
			}
		}
		return nil
	})
	if !found {
		t.Fatal("disk version 7.7.7 not in generated packages")
	}
	if err := RegeneratePackages(t.TempDir(), portfs.New(), &o, &e); err == nil {
		t.Fatal("empty root accepted")
	}
}
