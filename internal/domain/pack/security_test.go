package pack

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// secBase is a valid pack with a vuln, a sast and a lint check.
func secBase(sec string) []byte {
	s := `{"schema":1,"id":"x","name":"X","versions":{"stable":"1","as_of":"2026-11"},"detect":[{"file":"x.toml"}],
"source_globs":["**/*.x"],"test_globs":["**/*_test.x"],
"checks":[
 {"name":"lint","kind":"lint","argv":["l"],"required":true,"tool":"l"},
 {"name":"test","kind":"test","argv":["t"],"required":true,"tool":"t"},
 {"name":"sast1","kind":"sast","argv":["s"],"required":false,"tool":"s","default":"off"},
 {"name":"vuln1","kind":"vuln","argv":["v"],"required":false,"tool":"v","default":"off"}
]`
	if sec != "" {
		s += `,"security":` + sec
	}
	return []byte(s + `,"test_commands":[{"argv_prefix":["t"]}],
"race":{"mode":"none","checks":["test"],"reason":"r","substitute":"s"},"arch":{"parser":"none"},
"coverage":{"format":"lcov","path":"c.lcov","produced_by":["test"]}}`)
}

func TestSecurityValidation(t *testing.T) {
	cases := []struct{ name, sec, want string }{
		{"both", `{"checks":["vuln1"],"none":"x"}`, "security: set checks or none, not both"},
		{"empty", `{}`, "security: empty (set checks or none)"},
		{"blank none", `{"none":"  "}`, "security: empty (set checks or none)"},
		{"unknown", `{"checks":["nope"]}`, `security.checks: unknown check "nope"`},
		{"kind", `{"checks":["lint"]}`, `security.checks: "lint" is kind "lint", want vuln or sast`},
		{"dup", `{"checks":["vuln1","vuln1"]}`, `security.checks: duplicate "vuln1"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(secBase(c.sec))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err %v, want %q", err, c.want)
			}
		})
	}
}

func TestSecurityOptionalAndRequire(t *testing.T) {
	p, err := Parse(secBase(""))
	if err != nil {
		t.Fatal(err)
	}
	err = RequireSecurity(p)
	if err == nil || err.Error() != "pack x: security: not declared (set checks or none)" {
		t.Fatalf("err %v", err)
	}
	for _, sec := range []string{`{"checks":["vuln1"]}`, `{"none":"no scanner"}`} {
		p, err := Parse(secBase(sec))
		if err != nil {
			t.Fatal(err)
		}
		if err := RequireSecurity(p); err != nil {
			t.Fatal(err)
		}
	}
}

func names(cs []Check) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.Name)
	}
	return out
}

func TestSecurityChecksOrderAndNil(t *testing.T) {
	p, err := Parse(secBase(`{"checks":["vuln1","sast1"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := names(p.SecurityChecks()); !slices.Equal(got, []string{"sast1", "vuln1"}) {
		t.Fatalf("got %v, want pack order", got)
	}
	if (Pack{}).SecurityChecks() != nil {
		t.Fatal("nil Security must give nil")
	}
	p, _ = Parse(secBase(`{"none":"n"}`))
	if p.SecurityChecks() != nil {
		t.Fatal("None must give nil")
	}
}

func TestSecurityChecksAfterVariant(t *testing.T) {
	p, err := Parse(secBase(`{"checks":["vuln1"]}`))
	if err != nil {
		t.Fatal(err)
	}
	p.Variants = []Variant{{Name: "v", Checks: []Check{{Name: "vuln1", Kind: "vuln", Argv: []string{"merged"}}}}}
	q, err := p.WithVariant("v")
	if err != nil {
		t.Fatal(err)
	}
	got := q.SecurityChecks()
	if len(got) != 1 || got[0].Argv[0] != "merged" {
		t.Fatalf("got %+v", got)
	}
}

// Covers all shipped packs, Go included.
func TestShippedPacksDeclareSecurity(t *testing.T) {
	files, err := filepath.Glob("../../../specs/languages/*/pack.json")
	if err != nil || len(files) < 15 {
		t.Fatalf("files %v err %v", files, err)
	}
	for _, f := range files {
		id := filepath.Base(filepath.Dir(f))
		t.Run(id, func(t *testing.T) {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			p, err := Parse(b)
			if err != nil {
				t.Fatal(err)
			}
			if err := RequireSecurity(p); err != nil {
				t.Fatal(err)
			}
			if p.Security.None == "" && len(p.SecurityChecks()) != len(p.Security.Checks) {
				t.Fatalf("checks %v not all resolved", p.Security.Checks)
			}
		})
	}
}
