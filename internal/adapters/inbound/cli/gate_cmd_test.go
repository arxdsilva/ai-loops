package cli

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/arxdsilva/ai-loops/internal/application/gate"
)

// profGate records the options and whether the coverprofile path was a
// real, writable file while the gate ran.
type profGate struct {
	rcGate
	existed bool
}

func (g *profGate) Run(ctx context.Context, o gate.Options) (gate.Result, error) {
	_, err := os.Stat(o.CoverProfile)
	g.existed = err == nil
	return g.rcGate.Run(ctx, o)
}

func TestGateCmdCoverage(t *testing.T) {
	e := rcSetup(t, "")
	g := &profGate{}
	e.app.Gate = g
	g.res = gate.Result{OK: true, Checks: []gate.CheckResult{
		{Name: "test", OK: true, Duration: time.Second},
		{Name: "coverage", OK: true, Output: "base main\nfile  stmts\ntotal 3\n"},
	}}
	if code := e.do("gate", "--base", "v0.3.0", "--only", "test,coverage", "--pkgs", "./internal/x/..."); code != 0 {
		t.Fatal(code, e.err)
	}
	o := g.opts
	if o.Base != "v0.3.0" || o.CoverProfile == "" || !g.existed {
		t.Fatalf("%+v existed=%v", o, g.existed)
	}
	if _, err := os.Stat(o.CoverProfile); !os.IsNotExist(err) {
		t.Fatalf("coverprofile %s left behind: %v", o.CoverProfile, err)
	}
	want := "✓ test 1.0s\n✓ coverage 0.0s\n    base main\n    file  stmts\n    total 3\n"
	if e.out.String() != want {
		t.Fatalf("%q", e.out)
	}
}

func TestGateCmdCoverageIsRequired(t *testing.T) {
	e := rcSetup(t, "")
	if code := e.do("gate", "--skip", "coverage"); code != 2 || !strings.Contains(e.err.String(), "coverage is required") {
		t.Fatal(code, e.err)
	}
	if code := e.do("gate", "--skip", "test,coverage"); code != 2 {
		t.Fatal(code)
	}
}

func TestGateCmdTempFileError(t *testing.T) {
	e := rcSetup(t, "")
	t.Setenv("TMPDIR", "/definitely/not/a/dir")
	if code := e.do("gate"); code != 1 || !strings.Contains(e.err.String(), "coverprofile") {
		t.Fatal(code, e.err)
	}
}
