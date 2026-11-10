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
		// As specs/languages/swift: the flag goes after the subcommand
		// (`swift --scratch-path X build` fails).
		CacheArgv: map[string][]string{
			"swift build": {"--scratch-path", "{run}/swift/build"}, "swift test": {"--scratch-path", "{run}/swift/build"},
			"swift run": {"--scratch-path", "{run}/swift/build"}, "swift package": {"--scratch-path", "{run}/swift/build"}}}
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
	if len(sw.Env) != 0 || !reflect.DeepEqual(sw.Argv["swift build"], []string{"--scratch-path", "/r/swift/build"}) || !reflect.DeepEqual(sw.Tools, []string{"swift"}) {
		t.Fatalf("swift scope %+v", sw)
	}
	for in, want := range map[string]string{
		"swift build":                            "swift build '--scratch-path' '/r/swift/build'",
		"/usr/bin/swift test --sanitize=thread":  "/usr/bin/swift test '--scratch-path' '/r/swift/build' --sanitize=thread",
		`FOO="a b" swift build -c release`:       `FOO="a b" swift build '--scratch-path' '/r/swift/build' -c release`,
		"swift package resolve":                  "swift package '--scratch-path' '/r/swift/build' resolve",
		"swift build --scratch-path /mine":       "",
		"swift format lint .":                    "", // no key: left alone
		"swift":                                  "",
		"swift build && swift test":              "", // no env to export; flags only on simple lines
		"swift\tbuild":                           "swift\tbuild '--scratch-path' '/r/swift/build'",
		`swi\ft build`:                           `swi\ft build '--scratch-path' '/r/swift/build'`,
		"FOO=1":                                  "",
		"echo hi | swift build >/dev/null; true": "",
		"! swift build":                          "", // not simple, and no env to export
	} {
		got, ok := sw.Scope(in)
		if ok != (want != "") || got != want {
			t.Errorf("swift %q: got %q %v want %q", in, got, ok, want)
		}
	}

	multi := guard.PackScope{Tools: []string{"swift"}, Argv: map[string][]string{"swift": {"-a"}, "swift build": {"-b"}, "swift build x": {"-c"}}}
	for in, want := range map[string]string{
		"swift build":      "swift build '-b'",
		"swift build x y":  "swift build x '-c' y",
		"swift test":       "swift '-a' test",
		"swift build -b":   "", // the longest key's flag is there: no shorter key applies
		"swift build x -c": "",
	} {
		if got, ok := multi.Scope(in); got != want || ok != (want != "") {
			t.Errorf("longest key first: %q -> %q %v, want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"! cargo test", "time cargo test", "{ cargo test; }", "if cargo test; then :; fi", "FOO=1 ! cargo test"} {
		got, ok := s.Scope(in)
		if want := "export CARGO_TARGET_DIR='/r/rust/cache' TMPDIR='/r/rust/tmp' CARGO_INCREMENTAL='0'; " + in; !ok || got != want {
			t.Errorf("reserved leader %q: got %q %v", in, got, ok)
		}
	}
	empty := pack.Pack{ID: "e", CacheArgv: map[string][]string{"": {"-x"}, "  ": {"-y"}, "e run": {"-z"}}}
	if tools := guard.PackTools(empty); !reflect.DeepEqual(tools, []string{"e"}) {
		t.Errorf("empty cache_argv key: %v", tools)
	}
	if got, ok := guard.NewPackScope(empty, "/r").Scope("e run"); !ok || got != "e run '-z'" {
		t.Errorf("empty key scope: %q %v", got, ok)
	}
	if got, ok := (guard.PackScope{Tools: []string{"x"}, Argv: map[string][]string{"x": {"{bad}"}}}).Scope("x"); !ok || got != "x '{bad}'" {
		t.Errorf("raw argv: %q", got)
	}
	bad := pack.Pack{ID: "b", CacheArgv: map[string][]string{"b": {"{nope}"}}}
	if s := guard.NewPackScope(bad, "/r"); len(s.Argv) != 0 || !reflect.DeepEqual(s.Tools, []string{"b"}) {
		t.Errorf("unknown placeholder kept: %+v", s)
	}
}
