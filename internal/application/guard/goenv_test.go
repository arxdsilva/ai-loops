package guard_test

import (
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/application/guard"
)

const (
	tcache = "/p/.loops/runs/r1/go/cache"
	ttmp   = "/p/.loops/runs/r1/go/tmp"
)

func TestInspectGo(t *testing.T) {
	type u = guard.GoUse
	inv := u{Invokes: true, Unscoped: true, Simple: true}              // a simple go command needing scope
	gate := u{Invokes: true, Unscoped: true, Simple: true, Gate: true} // ... that is a gate command
	compound := u{Invokes: true, Unscoped: true}                       // compound line needing scope
	for _, c := range []struct {
		cmd  string
		want guard.GoUse
	}{
		{"go build ./...", gate},
		{"go test -race ./...", gate},
		{"go vet ./...", gate},
		{"go list -m all", gate},
		{"go -C sub vet ./...", gate},
		{"/usr/local/go/bin/go build ./...", gate},
		{"go1.22.1 test -race ./...", gate},
		{"GOFLAGS=-race go test ./...", gate},
		{"go build ./...\n", gate},
		{"go run ./cmd/x", inv},
		{"go install ./cmd/loops", inv},
		{"go generate ./...", inv},
		{"go tool cover -func=c.out", inv},
		{"sh -c 'go build ./...'", inv},
		{"gotestsum -- -race ./...", inv},
		{"richgo test -race ./...", inv},
		{"env FOO=1 go build", inv},
		{"go version", u{Simple: true}},
		{"go mod tidy", u{Simple: true}},
		{"go env GOCACHE", u{Simple: true}},
		{"loops go test -race ./...", u{Simple: true}},
		{"echo 'go build'", u{Simple: true}},
		{"cd x && go test -race ./...", compound},
		{"find . -name '*.go' | xargs go vet", compound},
		{"if true; then go build; fi", compound},
		{"time go build ./...", compound},
		{"echo $(go list ./...)", compound},
		{"go build ./... && go test -race ./...", compound},
		{"GOCACHE=" + tcache + " go build ./...", u{Invokes: true, Simple: true, Gate: true}},
		{"GOCACHE='" + tcache + "/' go build ./...", u{Invokes: true, Simple: true, Gate: true}},
		{"GOCACHE=/other go build ./...", gate},
		{"export GOCACHE=" + tcache + "; go build ./...", u{Invokes: true}},
		{"export GOCACHE=" + tcache + " && cd x && go test -race ./...", u{Invokes: true}},
		{"GOCACHE=" + tcache + " go build && go vet", compound},
		{"env GOCACHE=" + tcache + " go test -race ./...", u{Invokes: true, Simple: true}},
		{"env -u GOCACHE go build", inv},
		{"export GOCACHE=" + tcache + "; env -u GOCACHE go build", u{Invokes: true, Unscoped: true}},
		{`eval "$(loops run env)"; go test -race ./...`, u{Invokes: true}},
		{"GOCACHE=" + tcache + " sh -c 'go build'", u{Invokes: true, Simple: true}},
		{"watch -n 2 go build ./...", inv},
		{"eval 'go build ./...'", inv},
		{"nice -n 5 go build ./...", inv},
		{"timeout 30 go test -race ./...", inv},
		{"xargs -n 1 go vet", inv},
		{"bash -c 'go build \"unterminated'", u{Simple: true}},
		{"export -n GOCACHE=" + tcache + "; go build", compound},
		{"export -x GOCACHE=" + tcache + "; go build", u{Invokes: true}},
		{"env -i -u GOCACHE go build", inv},
		{"go build 'unterminated", u{}},
		{"", u{}},
	} {
		if got := guard.InspectGo(c.cmd, tcache); got != c.want {
			t.Errorf("%q: got %+v want %+v", c.cmd, got, c.want)
		}
	}
}

func TestScopeGo(t *testing.T) {
	pre := "GOCACHE='" + tcache + "' GOTMPDIR='" + ttmp + "' "
	exp := "export GOCACHE='" + tcache + "' GOTMPDIR='" + ttmp + "'; "
	for _, c := range []struct{ cmd, want string }{
		{"go build ./...", pre + "go build ./..."},
		{"GOFLAGS=-race go test ./...", pre + "GOFLAGS=-race go test ./..."},
		{"cd x && go test -race ./...", exp + "cd x && go test -race ./..."},
		{"if true; then go build; fi", exp + "if true; then go build; fi"},
		{"go build ./...\n", pre + "go build ./...\n"},
	} {
		if got := guard.ScopeGo(c.cmd, tcache, ttmp); got != c.want {
			t.Errorf("%q: got %q want %q", c.cmd, got, c.want)
		}
	}
	q := guard.ScopeGo("go build", "/it's/c", "/t")
	if q != `GOCACHE='/it'\''s/c' GOTMPDIR='/t' go build` {
		t.Fatal(q)
	}
}

// The rewrite must scope every go command and never change what race-guard
// decides about the line.
func TestScopeGoKeepsRaceVerdictAndScopes(t *testing.T) {
	for _, cmd := range []string{
		"go test ./...", "go test -race ./...", "go build ./... && go test ./...", "cd x && go test -race ./...",
		"GOFLAGS=-race go test ./...", "export GOFLAGS=-race && go test ./...", "sh -c 'go test -race ./...'",
		"echo ./... | xargs go test", "go run ./cmd/x", "if true; then go vet ./...; fi", "gotestsum -- ./...",
		"go build ./... # comment",
	} {
		got := guard.ScopeGo(cmd, tcache, ttmp)
		if (guard.CheckRace(got) == "") != (guard.CheckRace(cmd) == "") {
			t.Errorf("%q: race verdict changed by rewrite %q", cmd, got)
		}
		if u := guard.InspectGo(got, tcache); u.Unscoped || !u.Invokes {
			t.Errorf("%q: rewrite %q still unscoped: %+v", cmd, got, u)
		}
		if !strings.HasSuffix(got, cmd) {
			t.Errorf("%q: rewrite %q does not keep the command", cmd, got)
		}
	}
	// a later, conflicting assignment wins over the prefix: still unscoped.
	got := guard.ScopeGo("GOCACHE=/other go build", tcache, ttmp)
	if !guard.InspectGo(got, tcache).Unscoped {
		t.Fatalf("%q should stay unscoped", got)
	}
}
