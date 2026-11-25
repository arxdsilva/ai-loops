package guard_test

import (
	"testing"

	"github.com/arxdsilva/ai-loops/internal/application/guard"
)

// TestScopeGoUnsafeShapes: commands that fight the run's cache cannot be
// scoped by prefixing; toolchain-env denies them (see goEnv in the hook).
func TestScopeGoUnsafeShapes(t *testing.T) {
	for _, cmd := range []string{
		"GOCACHE=/elsewhere go build ./...",
		"env -u GOCACHE go build ./...",
	} {
		if !guard.InspectGo(guard.ScopeGo(cmd, tcache, ttmp), tcache).Unscoped {
			t.Errorf("%q was scoped; it should stay denied", cmd)
		}
	}
}

// TestScopeGoShapes: the shell shapes agents really send are scopeable, so
// toolchain-env rewrites them instead of denying with go_env.
func TestScopeGoShapes(t *testing.T) {
	for _, cmd := range []string{
		"cd x && go test -race ./...",
		"GOFLAGS=-p=2 go test -race -count=1 ./internal/...",
		"go test -race ./... 2>&1 | tail -20",
		"(cd a; go vet ./...)",
		"for p in a b; do go test -race ./$p/...; done",
		"env FOO=1 go test -race ./...",
		"L=/x/bin/loops; cd /repo && go build ./...",
	} {
		t.Run(cmd, func(t *testing.T) {
			use := guard.InspectGo(cmd, tcache)
			if !use.Invokes || !use.Unscoped {
				t.Fatalf("InspectGo = %+v, want Invokes && Unscoped", use)
			}
			scoped := guard.ScopeGo(cmd, tcache, ttmp)
			if guard.InspectGo(scoped, tcache).Unscoped {
				t.Fatalf("ScopeGo left it unscoped: %s", scoped)
			}
		})
	}
}
