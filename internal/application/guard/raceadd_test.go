package guard_test

import (
	"testing"

	"github.com/arxdsilva/ai-loops/internal/application/guard"
)

func TestAddRace(t *testing.T) {
	for _, c := range []struct {
		cmd, want string
		ok        bool
	}{
		{"go test ./...", "go test -race ./...", true},
		{"go test -count=1 ./internal/...", "go test -race -count=1 ./internal/...", true},
		{"cd x && go test ./...", "cd x && go test -race ./...", true},
		{"go test ./a; go test ./b", "go test -race ./a; go test -race ./b", true},
		{"go test;go test", "go test -race;go test -race", true},
		{"go test ./a && go test -race ./b", "go test -race ./a && go test -race ./b", true},
		{"go test -race ./a && go test ./b", "go test -race ./a && go test -race ./b", true},
		{"/usr/local/go/bin/go test ./...", "/usr/local/go/bin/go test -race ./...", true},
		{"go -C sub test ./...", "go -C sub test -race ./...", true},
		{"(cd a; go test ./...)", "(cd a; go test -race ./...)", true},
		{"go test ./... 2>&1 | tail -20", "go test -race ./... 2>&1 | tail -20", true},
		{"for p in a b; do go test ./$p/...; done", "for p in a b; do go test -race ./$p/...; done", true},
		{"GOFLAGS=-p=2 go test ./...", "GOFLAGS=-p=2 go test -race ./...", true},
		{"go test", "go test -race", true},
		// Nothing to add, or not fixable: unchanged, ok false.
		{"go test -race ./...", "go test -race ./...", false},
		{"go build ./...", "go build ./...", false},
		{"go test -race=false ./...", "go test -race=false ./...", false},
		{"gotestsum -- ./...", "gotestsum -- ./...", false},
		{"sh -c 'go test ./...'", "sh -c 'go test -race ./...'", true},
	} {
		got, ok := guard.AddRace(c.cmd)
		if got != c.want || ok != c.ok {
			t.Errorf("AddRace(%q) = %q, %v; want %q, %v", c.cmd, got, ok, c.want, c.ok)
		}
		if ok && guard.CheckRace(got) != "" {
			t.Errorf("AddRace(%q) result still denied", c.cmd)
		}
	}
}
