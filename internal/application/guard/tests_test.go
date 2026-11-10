package guard_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/application/guard"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
)

var (
	swiftPack = pack.Pack{ID: "swift", Name: "Swift",
		TestCommands: []pack.TestCommand{{ArgvPrefix: []string{"swift", "test"}}},
		Race:         pack.Race{Mode: pack.RaceFlag, Flags: []string{"--sanitize=thread"}},
		Checks:       []pack.Check{{Name: "build", Argv: []string{"swift", "build"}, Tool: "swift"}},
		CacheArgv:    map[string][]string{"swift": {"--scratch-path", "{run}/swift/build"}}}
	envPack = pack.Pack{ID: "fake", Name: "Fake",
		TestCommands: []pack.TestCommand{{ArgvPrefix: []string{"fk", "check"}}},
		Race:         pack.Race{Mode: pack.RaceFlag, Env: map[string]string{"FK_FLAGS": "--race"}}}
	nonePack = pack.Pack{ID: "python", Name: "Python",
		TestCommands: []pack.TestCommand{{ArgvPrefix: []string{"pytest"}}},
		Race:         pack.Race{Mode: pack.RaceNone, Checks: []string{"test"}}}
	buildPack = pack.Pack{ID: "cpp", Name: "C++",
		TestCommands: []pack.TestCommand{{ArgvPrefix: []string{"ctest"}}},
		Race:         pack.Race{Mode: pack.RaceBuild, Checks: []string{"test-tsan"}}}
	goPack = pack.Pack{ID: "go", TestCommands: []pack.TestCommand{{ArgvPrefix: []string{"go", "test"}}},
		Race: pack.Race{Mode: pack.RaceFlag, Flags: []string{"-race"}}}
)

func TestCheckTests(t *testing.T) {
	packs := []pack.Pack{goPack, swiftPack, envPack, nonePack, buildPack}
	cases := []struct {
		cmd  string
		deny string // "" = allow; else the reason prefix
	}{
		{"swift test", "test_guard:"},
		{"swift test --parallel", "test_guard:"},
		{"/usr/bin/swift test", "test_guard:"},
		{"swift test --sanitize=thread", ""},
		{"swift test --sanitize thread", ""},
		{"swift test --sanitize=address", "test_guard:"},
		{"swift build", ""},
		{"cd pkg && swift test", "test_guard:"},
		{`sh -c "swift test"`, "test_guard:"},
		{"timeout 60 swift test", "test_guard:"},
		{"echo `swift test`", "test_guard:"},
		{`eval "swift test --sanitize=thread"`, ""},
		{"fk check", "test_guard:"},
		{"FK_FLAGS=--race fk check", ""},
		{`FK_FLAGS="-v --race" fk check`, ""},
		{"export FK_FLAGS=--race; fk check", ""},
		{"FK_FLAGS=--race fk build && fk check", "test_guard:"},
		{"env FK_FLAGS=--race fk check", ""},
		{"export FK_FLAGS=--race; env -u FK_FLAGS fk check", "test_guard:"},
		{"FK_FLAGS=-v fk check", "test_guard:"},
		{"pytest -x", ""},
		{"ctest --preset default", ""},
		{"go test ./...", "race_guard:"},
		{"go test -race ./...", ""},
	}
	for _, c := range cases {
		got := guard.CheckTests(c.cmd, packs)
		if c.deny == "" && got != "" || c.deny != "" && !strings.HasPrefix(got, c.deny) {
			t.Errorf("%q: got %q, want prefix %q", c.cmd, got, c.deny)
		}
	}
	if got := guard.CheckTests("swift test", nil); got != "" {
		t.Errorf("no packs: %q", got)
	}
	if got := guard.CheckTests("go test ./...", nil); !strings.HasPrefix(got, "race_guard:") {
		t.Errorf("go always checked: %q", got)
	}
	msg := guard.CheckTests("swift test", packs)
	if !strings.Contains(msg, "`swift test` with --sanitize=thread") || !strings.Contains(msg, "Swift pack") {
		t.Errorf("message: %q", msg)
	}
	msg = guard.CheckTests("fk check", packs)
	if !strings.Contains(msg, "with FK_FLAGS=--race") {
		t.Errorf("env message: %q", msg)
	}
	both := envPack
	both.Race.Flags = []string{"-r"}
	if msg := guard.CheckTests("fk check", []pack.Pack{both}); !strings.Contains(msg, "-r (or set FK_FLAGS=--race)") {
		t.Errorf("flags+env message: %q", msg)
	}
}

func TestPackScope(t *testing.T) {
	rust := pack.Pack{ID: "rust",
		Checks:       []pack.Check{{Name: "clippy", Argv: []string{"cargo", "clippy"}, Tool: "cargo"}, {Name: "arch", Internal: "arch"}},
		TestCommands: []pack.TestCommand{{ArgvPrefix: []string{"cargo", "test"}}},
		CacheEnv:     map[string]string{"CARGO_TARGET_DIR": "cache", "TMPDIR": "tmp"},
		Env:          map[string]string{"CARGO_INCREMENTAL": "0", "TMPDIR": "/ignored"}}
	s := guard.NewPackScope(rust, "/r")
	if !reflect.DeepEqual(s.Env, []string{"CARGO_TARGET_DIR=/r/rust/cache", "TMPDIR=/r/rust/tmp", "CARGO_INCREMENTAL=0"}) {
		t.Fatalf("env %v", s.Env)
	}
	if !reflect.DeepEqual(s.Tools, []string{"cargo"}) {
		t.Fatalf("tools %v", s.Tools)
	}
	cases := []struct{ in, want string }{
		{"cargo test", "CARGO_TARGET_DIR='/r/rust/cache' TMPDIR='/r/rust/tmp' CARGO_INCREMENTAL='0' cargo test"},
		{"cd x && cargo build", "export CARGO_TARGET_DIR='/r/rust/cache' TMPDIR='/r/rust/tmp' CARGO_INCREMENTAL='0'; cd x && cargo build"},
		{"ls", ""},
		{"CARGO_TARGET_DIR=/t cargo test", ""},
		{"loops exec cargo test", ""},
		{`eval "$(loops run env)"; cargo test`, ""},
		{`echo "cargo test"`, ""},
		{`cargo "unterminated`, ""},
		{"", ""},
	}
	for _, c := range cases {
		got, ok := s.Scope(c.in)
		if ok != (c.want != "") || got != c.want {
			t.Errorf("%q: got %q %v want %q", c.in, got, ok, c.want)
		}
	}

	sw := guard.NewPackScope(swiftPack, "/r")
	if len(sw.Env) != 0 || !reflect.DeepEqual(sw.Argv["swift"], []string{"--scratch-path", "/r/swift/build"}) {
		t.Fatalf("swift scope %+v", sw)
	}
	for in, want := range map[string]string{
		"swift build":                            "swift '--scratch-path' '/r/swift/build' build",
		"/usr/bin/swift test --sanitize=thread":  "/usr/bin/swift '--scratch-path' '/r/swift/build' test --sanitize=thread",
		`FOO="a b" swift build`:                  `FOO="a b" swift '--scratch-path' '/r/swift/build' build`,
		"swift build --scratch-path /mine":       "",
		"swift build && swift test":              "", // no env to export; flags only on simple lines
		"swift\tbuild":                           "swift '--scratch-path' '/r/swift/build'\tbuild",
		`swi\ft build`:                           `swi\ft '--scratch-path' '/r/swift/build' build`,
		"FOO=1":                                  "",
		"echo hi | swift build >/dev/null; true": "",
	} {
		got, ok := sw.Scope(in)
		if ok != (want != "") || got != want {
			t.Errorf("swift %q: got %q %v want %q", in, got, ok, want)
		}
	}

	multi := guard.PackScope{Tools: []string{"swift"}, Argv: map[string][]string{"swift": {"-a"}, "swift build": {"-b"}}}
	if got, _ := multi.Scope("swift build"); got != "swift build '-b'" {
		t.Errorf("longest key first: %q", got)
	}
	if got, ok := (guard.PackScope{Tools: []string{"x"}, Argv: map[string][]string{"x": {"{bad}"}}}).Scope("x"); !ok || got != "x '{bad}'" {
		t.Errorf("raw argv: %q", got)
	}
	bad := pack.Pack{ID: "b", CacheArgv: map[string][]string{"b": {"{nope}"}}}
	if s := guard.NewPackScope(bad, "/r"); len(s.Argv) != 0 || !reflect.DeepEqual(s.Tools, []string{"b"}) {
		t.Errorf("unknown placeholder kept: %+v", s)
	}
}
