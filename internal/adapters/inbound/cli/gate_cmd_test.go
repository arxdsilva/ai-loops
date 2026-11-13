package cli

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/arxdsilva/ai-loops/internal/application/gate"
	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
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

// gateLangs is a fake LangPacks for the gate: Detect returns ds or err.
type gateLangs struct {
	ds  []langpack.Detected
	err error
}

func (g gateLangs) All() ([]pack.Pack, error)     { return nil, nil }
func (g gateLangs) Get(string) (pack.Pack, error) { return pack.Pack{}, nil }
func (g gateLangs) Detect(string, []langpack.Override) ([]langpack.Detected, error) {
	return g.ds, g.err
}

func TestGateCmdPacks(t *testing.T) {
	no := false
	py := pack.Pack{ID: "python", Checks: []pack.Check{
		{Name: "ruff", Kind: "lint", Argv: []string{"ruff"}},
		{Name: "pytest", Kind: "test", Argv: []string{"pytest"}, Required: true},
		{Name: "cov", Kind: "coverage", Internal: "coverage", Skippable: &no},
		{Name: "audit", Kind: "vuln", Argv: []string{"pip-audit"}, Default: "off"},
	}}
	e := rcSetup(t, "")
	g := &profGate{}
	e.app.Gate = g
	e.app.Langs = gateLangs{ds: []langpack.Detected{{Pack: py, ID: "python", Root: "."}, {Pack: py, ID: "python", Root: "svc"}}}
	g.res = gate.Result{OK: true, Checks: []gate.CheckResult{
		{Name: "pytest", Pack: "python@.", OK: true, Duration: time.Second},
		{Name: "pytest", Pack: "python@svc", Skipped: true, Output: "skipped: none of x present"},
	}}
	if code := e.do("gate", "--enable", "audit", "--skip", "ruff"); code != 0 {
		t.Fatal(code, e.err)
	}
	want := "✓ pytest [python@.] 1.0s\n- pytest [python@svc] (skipped)\n    skipped: none of x present\n"
	if e.out.String() != want {
		t.Fatalf("%q", e.out)
	}
	o := g.opts
	if len(o.Packs) != 2 || o.Enable[0] != "audit" || o.CoverDir == "" || o.CoverProfile == "" {
		t.Fatalf("%+v", o)
	}
	if code := e.do("gate", "--skip", "cov"); code != 2 || !strings.Contains(e.err.String(), "cov is required") {
		t.Fatal(code, e.err)
	}
	if code := e.do("gate", "--only", "vet"); code != 2 || !strings.Contains(e.err.String(), "valid: ruff, pytest, cov, audit") {
		t.Fatal(code, e.err)
	}
	e.app.Langs = gateLangs{}
	if code := e.do("gate"); code != 1 || !strings.Contains(e.err.String(), "no language pack detected") {
		t.Fatal(code, e.err)
	}
	e.app.Langs = gateLangs{err: errors.New("bad pack")}
	if code := e.do("gate"); code != 1 || !strings.Contains(e.err.String(), "bad pack") {
		t.Fatal(code, e.err)
	}
}

func TestGateCmdGateError(t *testing.T) {
	e := rcSetup(t, "")
	e.app.Gate = errGate{}
	if code := e.do("gate"); code != 1 || !strings.Contains(e.err.String(), "no language pack detected") {
		t.Fatal(code, e.err)
	}
}

type errGate struct{}

func (errGate) Run(context.Context, gate.Options) (gate.Result, error) {
	return gate.Result{}, gate.ErrNoPack
}

func TestGateCmdSecurity(t *testing.T) {
	scan := pack.Pack{ID: "go", Checks: []pack.Check{
		{Name: "vet", Kind: "lint", Argv: []string{"go", "vet"}},
		{Name: "govulncheck", Kind: "vuln", Argv: []string{"govulncheck"}, Default: "off"},
	}, Security: &pack.Security{Checks: []string{"govulncheck"}}}
	none := pack.Pack{ID: "ruby", Checks: []pack.Check{{Name: "rubocop", Kind: "lint", Argv: []string{"rubocop"}}},
		Security: &pack.Security{None: "no scanner wired"}}
	undeclared := pack.Pack{ID: "php", Checks: []pack.Check{{Name: "phpstan", Kind: "lint", Argv: []string{"phpstan"}}}}
	e := rcSetup(t, "")
	g := &profGate{}
	e.app.Gate = g
	e.app.Langs = gateLangs{ds: []langpack.Detected{
		{Pack: scan, ID: "go", Root: "."}, {Pack: none, ID: "ruby", Root: "r"}, {Pack: undeclared, ID: "php", Root: "p"},
	}}
	g.res = gate.Result{OK: true, Checks: []gate.CheckResult{{Name: "govulncheck", Pack: "go@.", OK: true, Duration: time.Second}}}
	if code := e.do("gate", "--security"); code != 0 {
		t.Fatal(code, e.err)
	}
	want := "gate: ruby: no security tools (no scanner wired)\ngate: php: security not declared\n✓ govulncheck [go@.] 1.0s\n"
	if e.out.String() != want {
		t.Fatalf("%q", e.out)
	}
	o := g.opts
	if !o.NoRecord || o.Missing != gate.MissingSkip || len(o.Enable) != 1 || o.Enable[0] != "govulncheck" ||
		len(o.PackChecks["go"]) != 1 || o.PackChecks["go"][0] != "govulncheck" ||
		len(o.PackChecks["ruby"]) != 0 || len(o.PackChecks["php"]) != 0 {
		t.Fatalf("%+v", o)
	}
	// Without --security the options are the plain ones.
	e.out.Reset()
	if code := e.do("gate"); code != 0 {
		t.Fatal(code, e.err)
	}
	if g.opts.NoRecord || g.opts.PackChecks != nil || strings.Contains(e.out.String(), "security") {
		t.Fatalf("%+v %q", g.opts, e.out)
	}
}
