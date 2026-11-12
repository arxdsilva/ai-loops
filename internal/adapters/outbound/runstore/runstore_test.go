package runstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

func TestConcurrentAppend(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := New(root)
			for i := 0; i < 200; i++ {
				if err := s.Append(ctx, "r", run.Event{Kind: run.KindNote, Note: "0123456789012345678901234567890123456789"}); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
	b, err := os.ReadFile(filepath.Join(New(root).Dir("r"), "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSuffix(b, []byte("\n")), []byte("\n"))
	if len(lines) != 1600 {
		t.Fatalf("lines %d", len(lines))
	}
	for _, l := range lines {
		var e run.Event
		if json.Unmarshal(l, &e) != nil {
			t.Fatalf("malformed %q", l)
		}
	}
	evs, _ := New(root).Load(ctx, "r")
	if len(evs) != 1600 {
		t.Fatal(len(evs))
	}
}

func TestLoadSkipsMalformedAndMissing(t *testing.T) {
	s := New(t.TempDir())
	ctx := context.Background()
	if evs, err := s.Load(ctx, "none"); err != nil || len(evs) != 0 {
		t.Fatal(evs, err)
	}
	_ = s.Append(ctx, "r", run.Event{Kind: run.KindRun, Run: "r"})
	f, _ := os.OpenFile(filepath.Join(s.Dir("r"), "events.jsonl"), os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString("garbage\n\n{\"t\":\n{\"kind\":\"note\"}")
	f.Close()
	evs, _ := s.Load(ctx, "r")
	if len(evs) != 2 || evs[1].Kind != run.KindNote {
		t.Fatalf("%+v", evs)
	}
}

func TestPointer(t *testing.T) {
	s := New(t.TempDir())
	if id, err := s.Current(); id != "" || err != nil {
		t.Fatal(id, err)
	}
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCurrent("abc"); err != nil {
		t.Fatal(err)
	}
	if id, _ := s.Current(); id != "abc" {
		t.Fatal(id)
	}
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	if id, _ := s.Current(); id != "" {
		t.Fatal(id)
	}
}

func TestAppendAfterTornTail(t *testing.T) {
	s := New(t.TempDir())
	ctx := context.Background()
	_ = s.Append(ctx, "r", run.Event{Kind: run.KindRun})
	f, _ := os.OpenFile(filepath.Join(s.Dir("r"), "events.jsonl"), os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString(`{"kind":"no`)
	f.Close()
	_ = s.Append(ctx, "r", run.Event{Kind: run.KindEnd})
	evs, _ := s.Load(ctx, "r")
	if len(evs) != 2 || evs[1].Kind != run.KindEnd {
		t.Fatalf("%+v", evs)
	}
}

func TestCreateExclusive(t *testing.T) {
	s := New(t.TempDir())
	ctx := context.Background()
	if err := s.Create(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if err := s.Create(ctx, "a"); !errors.Is(err, fs.ErrExist) {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if New(s.root).Create(ctx, "race") == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatal(wins)
	}
}

// Every way the store creates .loops leaves a .gitignore that keeps it out
// of git.
func TestStoreKeepsLoopsOutOfGit(t *testing.T) {
	ctx := context.Background()
	for name, do := range map[string]func(s *Store) error{
		"create":      func(s *Store) error { return s.Create(ctx, "r1") },
		"append":      func(s *Store) error { return s.Append(ctx, "r1", run.Event{Kind: run.KindNote}) },
		"set current": func(s *Store) error { return s.SetCurrent("r1") },
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := do(New(root)); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(filepath.Join(root, ".loops", ".gitignore"))
			if err != nil || !bytes.Contains(b, []byte("\n*\n")) {
				t.Fatalf("%q %v", b, err)
			}
		})
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".loops"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := New(root).Create(ctx, "r1"); err == nil {
		t.Fatal(".loops as a file accepted")
	}
}
