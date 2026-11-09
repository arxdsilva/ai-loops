package updatestate

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/update"
)

func TestRoundTrip(t *testing.T) {
	f := NewAt(filepath.Join(t.TempDir(), "a", "loops", "update.json"))
	if s, err := f.Load(); err != nil || s != (update.State{}) {
		t.Fatalf("missing: %+v %v", s, err)
	}
	want := update.State{LastCheck: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), Latest: "v0.4.0", LastNotified: "v0.4.0"}
	if err := f.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := f.Load()
	if err != nil || !got.LastCheck.Equal(want.LastCheck) || got.Latest != want.Latest || got.LastNotified != want.LastNotified {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestCorruptIsZero(t *testing.T) {
	p := filepath.Join(t.TempDir(), "update.json")
	os.WriteFile(p, []byte("{not json"), 0o644)
	if s, err := NewAt(p).Load(); err != nil || s != (update.State{}) {
		t.Fatalf("%+v %v", s, err)
	}
}

func TestUnreadableErrors(t *testing.T) {
	d := t.TempDir() // a directory in place of the file
	if _, err := NewAt(d).Load(); err == nil {
		t.Fatal("want error")
	}
}

func TestEmptyPathNoop(t *testing.T) {
	f := NewAt("")
	if err := f.Save(update.State{Latest: "v1.0.0"}); err != nil {
		t.Fatal(err)
	}
	if s, err := f.Load(); err != nil || s.Latest != "" {
		t.Fatalf("%+v %v", s, err)
	}
}

func TestSaveFailure(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	os.WriteFile(blocker, nil, 0o644)
	if err := NewAt(filepath.Join(blocker, "x", "update.json")).Save(update.State{}); err == nil {
		t.Fatal("want error")
	}
}

func TestDefaultPath(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/x/state")
	if f := New(); f.path != "/x/state/loops/update.json" {
		t.Fatal(f.path)
	}
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "/h")
	if f := New(); f.path != "/h/.local/state/loops/update.json" {
		t.Fatal(f.path)
	}
}
