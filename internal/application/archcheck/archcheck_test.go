package archcheck_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/insurgencygames/ai-loops/internal/adapters/outbound/goparse"
	"github.com/insurgencygames/ai-loops/internal/application/archcheck"
	"github.com/insurgencygames/ai-loops/internal/domain/archrule"
)

type fakeParser struct {
	m    archrule.Module
	err  error
	root string
}

func (f *fakeParser) Parse(ctx context.Context, root string) (archrule.Module, error) {
	f.root = root
	return f.m, f.err
}

func TestCheckEvaluatesTheParsedModule(t *testing.T) {
	p := &fakeParser{m: archrule.Module{Path: "example.com/m", Packages: []archrule.Package{{
		Dir: "core/x", Files: []string{"core/x/x.go"},
		Imports: []archrule.Import{{Path: "os", Pos: archrule.Pos{File: "core/x/x.go", Line: 3}}},
	}}}}
	rep, err := archcheck.New(p).Check(context.Background(), "/some/root", archrule.Layers{Domain: []string{"core"}})
	if err != nil {
		t.Fatal(err)
	}
	if p.root != "/some/root" {
		t.Errorf("parsed %q, want /some/root", p.root)
	}
	if rep.Module != "example.com/m" || len(rep.Findings) != 1 || rep.Findings[0].Rule != "imports" || rep.OK() {
		t.Fatalf("want one imports error with the given layers, got %+v", rep)
	}
}

func TestCheckPropagatesParseErrors(t *testing.T) {
	boom := errors.New("boom")
	if _, err := archcheck.New(&fakeParser{err: boom}).Check(context.Background(), ".", archrule.Layers{}); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}

const fixtures = "../../adapters/outbound/goparse/testdata"

func check(t *testing.T, root string) archrule.Report {
	t.Helper()
	rep, err := archcheck.New(goparse.New()).Check(context.Background(), root, archrule.Layers{})
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

// summary renders findings as "file:line rule severity" lines.
func summary(rep archrule.Report) []string {
	var out []string
	for _, f := range rep.Findings {
		out = append(out, fmt.Sprintf("%s:%d %s %s", f.File, f.Line, f.Rule, f.Severity))
	}
	return out
}

// The fixture modules end to end: each one trips exactly the findings below.
func TestFixturesEndToEnd(t *testing.T) {
	cases := map[string][]string{
		"clean": nil,
		"imports": {
			"internal/adapters/inbound/cli/cli.go:3 imports error",
			"internal/adapters/outbound/db/db.go:3 imports error",
			"internal/adapters/outbound/db/sqlite/sqlite.go:3 imports warn",
			"internal/application/orders/orders.go:3 imports error",
			"internal/domain/order/line/line.go:3 imports error",
			"internal/domain/order/order.go:4 imports error",
			"internal/domain/order/order.go:5 imports error",
			"internal/domain/order/order.go:7 imports error",
			"internal/domain/order/order.go:9 imports error",
			"internal/domain/order/order.go:10 imports error",
		},
		"init": {"internal/platform/reg/reg.go:8 init error"},
		"global": {
			"internal/platform/state/state.go:8 global error",
			"internal/platform/state/state.go:10 global error",
			"internal/platform/state/state.go:25 global error",
		},
		"purity": {
			"internal/domain/sim/sim.go:11 clock error",
			"internal/domain/sim/sim.go:12 clock error",
			"internal/domain/sim/sim.go:15 go error",
			"internal/domain/sim/sim.go:19 rand error",
			"internal/domain/sim/sim.go:19 rand error",
			"internal/domain/sim/sim.go:22 rand error", // var draw = rand.Intn
			"internal/domain/sim/sim.go:26 rand error", // f := r2.Float64
		},
		"cmd": {
			"cmd/bad:0 cmd error",
			"cmd/bad/main.go:5 cmd error",
			"cmd/bad/main.go:8 cmd error",
			"cmd/bad/main.go:11 cmd error",
			"cmd/bad/main.go:14 cmd error",
			"cmd/bad/main.go:14 cmd error",
			"cmd/bad/main.go:18 cmd error",
			"cmd/bad/main.go:19 cmd error",
			"cmd/long/main.go:1 cmd error",
			"cmd/nomain:0 cmd error",
			"cmd/nomain:0 cmd error", // holds tool.go, has no main.go
			"cmd/sub:0 cmd error",
		},
		"limits": {
			"internal/adapters/outbound/big/big.go:4 fields warn",
			"internal/adapters/outbound/big/big.go:4 methods warn",
			"internal/adapters/outbound/big/big.go:121 iface warn",
		},
		"skips": nil, // sub imports the module root package: not a parent import
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			rep := check(t, filepath.Join(fixtures, name))
			if got := summary(rep); !reflect.DeepEqual(got, want) {
				t.Fatalf("findings:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
			wantOK := !strings.Contains(strings.Join(want, "\n"), " error")
			if rep.OK() != wantOK {
				t.Errorf("OK() = %v, want %v", rep.OK(), wantOK)
			}
		})
	}
}

// The package-size limits end to end, on a generated module.
func TestPackageSizeLimitsEndToEnd(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/big\n\ngo 1.24\n")
	for i := range 61 {
		write(fmt.Sprintf("internal/platform/big/f%02d.go", i), "package big\n"+strings.Repeat("\n", 199))
	}
	rep := check(t, root)
	want := []string{"internal/platform/big:0 files warn", "internal/platform/big:0 lines warn"}
	if got := summary(rep); !reflect.DeepEqual(got, want) || !rep.OK() {
		t.Fatalf("findings = %v, want %v and OK", got, want)
	}
}
