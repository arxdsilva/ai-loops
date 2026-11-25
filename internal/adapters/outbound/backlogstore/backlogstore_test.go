package backlogstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/domain/backlog"
)

func TestUpdateLoadRoundTrip(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	ctx := context.Background()
	if s.Exists() {
		t.Fatal("state before init")
	}
	if st, err := s.Load(ctx); err != nil || st.Version != backlog.Version || len(st.Items) != 0 {
		t.Fatalf("empty load = %+v, %v", st, err)
	}
	err := s.Update(ctx, func(st *backlog.State) error {
		st.Settings.Repo = "o/r"
		st.Items = append(st.Items, backlog.Item{Number: 3, Status: backlog.Queued})
		return nil
	})
	if err != nil || !s.Exists() {
		t.Fatalf("update: %v", err)
	}
	st, err := s.Load(ctx)
	if err != nil || st.Settings.Repo != "o/r" || len(st.Items) != 1 {
		t.Fatalf("load = %+v, %v", st, err)
	}
	if b, err := os.ReadFile(filepath.Join(root, ".loops", ".gitignore")); err != nil || !strings.Contains(string(b), "*") {
		t.Fatalf(".loops/.gitignore not created: %q %v", b, err)
	}
	// A failing fn saves nothing.
	boom := errors.New("boom")
	if err := s.Update(ctx, func(st *backlog.State) error { st.Items = nil; return boom }); !errors.Is(err, boom) {
		t.Fatalf("fn error = %v", err)
	}
	if st, _ := s.Load(ctx); len(st.Items) != 1 {
		t.Fatal("failed update was saved")
	}
}

func TestUpdateSerializes(t *testing.T) {
	s := New(t.TempDir())
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// A fresh Store per writer, as separate processes would have.
			if err := New(s.root).Update(ctx, func(st *backlog.State) error {
				st.Items = append(st.Items, backlog.Item{Number: i})
				return nil
			}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if st, _ := s.Load(ctx); len(st.Items) != 20 {
		t.Fatalf("lost updates: %d items", len(st.Items))
	}
}

func TestLoadErrors(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	os.MkdirAll(s.dir, 0o755)
	os.WriteFile(s.statePath(), []byte("{"), 0o644)
	if _, err := s.Load(context.Background()); err == nil {
		t.Fatal("corrupt state accepted")
	}
	os.WriteFile(s.statePath(), []byte(`{"version": 99}`), 0o644)
	if _, err := s.Load(context.Background()); err == nil || !strings.Contains(err.Error(), "update loops") {
		t.Fatalf("newer version: %v", err)
	}
}

func TestLaneLocks(t *testing.T) {
	s := New(t.TempDir())
	if s.LaneHeld(4) {
		t.Fatal("held before taken")
	}
	release, err := s.HoldLane(4)
	if err != nil {
		t.Fatal(err)
	}
	if !s.LaneHeld(4) {
		t.Fatal("not held while taken")
	}
	if _, err := s.HoldLane(4); err == nil {
		t.Fatal("second lane on the same issue")
	}
	release.Release()
	release.Release() // safe twice
	if s.LaneHeld(4) {
		t.Fatal("held after release")
	}
	if !strings.HasSuffix(s.LogPath(4), filepath.Join(".loops", "backlog", "logs", "issue-4.log")) {
		t.Fatalf("log path = %s", s.LogPath(4))
	}
}
