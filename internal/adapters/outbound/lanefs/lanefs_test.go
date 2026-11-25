package lanefs

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLatest(t *testing.T) {
	wt := t.TempDir()
	var l Lanes
	if lane, err := l.Latest(wt); err != nil || lane.Dir != "" {
		t.Fatalf("no runs = %+v, %v", lane, err)
	}
	if lane, err := l.Latest(""); err != nil || lane.Dir != "" {
		t.Fatalf("no worktree = %+v, %v", lane, err)
	}
	runs := filepath.Join(wt, ".loops", "runs")
	write(t, filepath.Join(runs, "20260101-old", "events.jsonl"), `{"kind":"run"}`+"\n")
	newest := filepath.Join(runs, "20260202-new")
	write(t, filepath.Join(newest, "events.jsonl"), `{"kind":"run","run":"x"}`+"\n"+`{"kind":"end","status":"done"}`+"\n")
	write(t, filepath.Join(newest, "pr.md"), "## Open follow-ups\n- a\n")
	lane, err := l.Latest(wt)
	if err != nil || lane.Dir != newest || len(lane.Events) != 2 || lane.PRBody == "" {
		t.Fatalf("latest = %+v, %v", lane, err)
	}
}

func TestPhase(t *testing.T) {
	wt := t.TempDir()
	var l Lanes
	if l.Phase(wt) != "" {
		t.Fatal("phase without status")
	}
	p := filepath.Join(wt, ".loops", "status.json")
	write(t, p, `{"active":true,"state":{"phase":"qa"}}`)
	if got := l.Phase(wt); got != "qa" {
		t.Fatalf("phase = %q", got)
	}
	write(t, p, `{"active":false,"state":{"phase":"reflect"}}`)
	if l.Phase(wt) != "" {
		t.Fatal("inactive run has no live phase")
	}
}

func TestRun(t *testing.T) {
	wt := t.TempDir()
	var l Lanes
	if _, ok := l.Run(wt); ok {
		t.Fatal("run without status")
	}
	if _, ok := l.Run(""); ok {
		t.Fatal("run without worktree")
	}
	p := filepath.Join(wt, ".loops", "status.json")
	write(t, p, `{"active":true,"state":{"phase":"implement","steps":12,"lanes":[{"agent":"a","running":true}]}}`)
	if s, ok := l.Run(wt); !ok || s.Phase != "implement" || s.Steps != 12 || len(s.Lanes) != 1 {
		t.Fatalf("run = %+v, %v", s, ok)
	}
	for _, body := range []string{`{"active":false}`, `{"active":true}`, `not json`} {
		write(t, p, body)
		if _, ok := l.Run(wt); ok {
			t.Fatalf("%s reads as a live run", body)
		}
	}
}

func TestReadRef(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "findings", "a.md"), "details")
	var l Lanes
	if s, ok := l.ReadRef(dir, "findings/a.md"); !ok || s != "details" {
		t.Fatalf("ref = %q %v", s, ok)
	}
	for _, bad := range []string{"../escape", "/etc/passwd", "missing.md"} {
		if _, ok := l.ReadRef(dir, bad); ok {
			t.Fatalf("%s read", bad)
		}
	}
	if _, ok := l.ReadRef("", "a.md"); ok {
		t.Fatal("no run dir read")
	}
	// Symlinks out of the run dir, to a file or through a directory, are refused;
	// one that stays inside is followed.
	outside := t.TempDir()
	write(t, filepath.Join(outside, "secret.txt"), "secret")
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(dir, "findings", "leak.md")); err != nil {
		t.Skip("no symlinks here:", err)
	}
	os.Symlink(outside, filepath.Join(dir, "out"))
	os.Symlink("a.md", filepath.Join(dir, "findings", "same.md"))
	for _, bad := range []string{"findings/leak.md", "out/secret.txt"} {
		if s, ok := l.ReadRef(dir, bad); ok {
			t.Fatalf("%s read through a symlink: %q", bad, s)
		}
	}
	if s, ok := l.ReadRef(dir, "findings/same.md"); !ok || s != "details" {
		t.Fatalf("inner symlink = %q %v", s, ok)
	}
}
