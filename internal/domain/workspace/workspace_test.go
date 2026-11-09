package workspace

import (
	"encoding/binary"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLayout(t *testing.T) {
	run := filepath.FromSlash("/p/.loops/runs/r1")
	if GoDir(run) != filepath.Join(run, "go") || GoCache(run) != filepath.Join(run, "go", "cache") || GoTmp(run) != filepath.Join(run, "go", "tmp") {
		t.Fatal(GoDir(run), GoCache(run), GoTmp(run))
	}
	if Baseline(run) != filepath.Join(run, "baseline-untracked.txt") || BaselineDirs(run) != filepath.Join(run, "baseline-dirs.txt") {
		t.Fatal(Baseline(run), BaselineDirs(run))
	}
	want := []string{"GOCACHE=" + GoCache(run), "GOTMPDIR=" + GoTmp(run)}
	if got := Env(run); !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
}

func TestExports(t *testing.T) {
	for _, c := range []struct{ shell, cache, tmp, want string }{
		{"sh", "/a/c", "/a/t", "export GOCACHE='/a/c' GOTMPDIR='/a/t'"},
		{"", "/a b/c", "/a/t", "export GOCACHE='/a b/c' GOTMPDIR='/a/t'"},
		{"bash", "/it's/c", "/t", `export GOCACHE='/it'\''s/c' GOTMPDIR='/t'`},
		{"fish", `/it's\c`, "/t", `set -gx GOCACHE '/it\'s\\c'; set -gx GOTMPDIR '/t'`},
		{"powershell", `C:\it's\c`, `C:\t`, `$env:GOCACHE='C:\it''s\c'; $env:GOTMPDIR='C:\t'`},
		{"pwsh", "/c", "/t", `$env:GOCACHE='/c'; $env:GOTMPDIR='/t'`},
	} {
		got, err := Exports(c.shell, c.cache, c.tmp)
		if err != nil || got != c.want {
			t.Errorf("%s: %q %v, want %q", c.shell, got, err, c.want)
		}
	}
	if _, err := Exports("cmd", "/c", "/t"); err == nil {
		t.Fatal("want error for unknown shell")
	}
}

func TestShellQuote(t *testing.T) {
	if ShellQuote("a b") != "'a b'" || ShellQuote("x'y") != `'x'\''y'` || ShellQuote("") != "''" {
		t.Fatal(ShellQuote("a b"), ShellQuote("x'y"))
	}
}

func TestSafeRel(t *testing.T) {
	for _, c := range []struct {
		p  string
		ok bool
	}{
		{"a.test", true},
		{"cmd/x/x.test", true},
		{"", false},
		{".", false},
		{"/abs/x", false},
		{"../x", false},
		{"a/../../x", false},
		{"a/./b", false}, // not clean
		{"a//b", false},
		{".loops/runs/x", false},
		{".loops", false},
		{".git/config", false},
		{"sub/.git/x", false},
		{"a\nb", false},
		{"a\x00b", false},
		{"a\\b", false},
		{".loopsfoo/x", true},
		{"dir/", false},
	} {
		if got := SafeRel(c.p); got != c.ok {
			t.Errorf("SafeRel(%q) = %v", c.p, got)
		}
	}
}

func TestNewPaths(t *testing.T) {
	base := []string{"keep.test", "old/bin", ".loops/runs/a/x"}
	cur := []string{"new.test", "keep.test", "old/bin", ".loops/runs/b/y", "../evil", "z/new", "new.test", "nested/"}
	got, unsafe := NewPaths(base, cur)
	if want := []string{"nested/", "new.test", "z/new"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
	if want := []string{"../evil"}; !reflect.DeepEqual(unsafe, want) {
		t.Fatalf("unsafe %v", unsafe)
	}
}

func TestIsArtifactName(t *testing.T) {
	yes := []string{"x.test", "x.test.exe", "c.out", "cpu.prof", "m.pprof", "a.cov", "coverage.txt", "coverage-unit.txt", "cover.out", "coverprofile.out", "__debug_bin", "__debug_bin1234", "app.exe", "X.TEST"}
	no := []string{"main.go", "x_test.go", "notes.txt", "coverage.md", "README", "test", "out", "run.sh", "cover.go", "x.test.go"}
	for _, n := range yes {
		if !IsArtifactName(n) {
			t.Errorf("%s should be an artifact", n)
		}
	}
	for _, n := range no {
		if IsArtifactName(n) {
			t.Errorf("%s should not be an artifact", n)
		}
	}
}

func pe() []byte {
	b := make([]byte, 0x90)
	copy(b, "MZ")
	binary.LittleEndian.PutUint32(b[0x3c:], 0x80)
	copy(b[0x80:], "PE\x00\x00")
	return b
}

func TestIsExecutableBinary(t *testing.T) {
	fat := []byte{0xca, 0xfe, 0xba, 0xbe, 0, 0, 0, 2}
	java := []byte{0xca, 0xfe, 0xba, 0xbe, 0, 0, 0, 52}
	badPE := pe()
	copy(badPE[0x80:], "XX")
	farPE := make([]byte, 0x40)
	copy(farPE, "MZ")
	binary.LittleEndian.PutUint32(farPE[0x3c:], 0x1000)
	for _, c := range []struct {
		name string
		head []byte
		want bool
	}{
		{"elf", []byte("\x7fELF\x02\x01\x01"), true},
		{"macho64", []byte{0xcf, 0xfa, 0xed, 0xfe, 7, 0, 0, 1}, true},
		{"macho32", []byte{0xce, 0xfa, 0xed, 0xfe}, true},
		{"macho-be", []byte{0xfe, 0xed, 0xfa, 0xcf}, true},
		{"macho-be32", []byte{0xfe, 0xed, 0xfa, 0xce}, true},
		{"fat", fat, true},
		{"java", java, false},
		{"pe", pe(), true},
		{"mz-text", []byte("MZ is a text file that starts oddly"), false},
		{"bad-pe", badPE, false},
		{"far-pe", farPE, false},
		{"script", []byte("#!/bin/sh\necho hi\n"), false},
		{"text", []byte("hello"), false},
		{"empty", nil, false},
		{"short", []byte{0x7f}, false},
	} {
		if got := IsExecutableBinary(c.head); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
}

func TestDecide(t *testing.T) {
	elf := []byte("\x7fELF\x02")
	for _, c := range []struct {
		name string
		e    Entry
		want Action
	}{
		{"test binary by name", Entry{Rel: "pkg/pkg.test", Kind: KindFile, Head: elf}, Remove},
		{"coverage text by name", Entry{Rel: "cover.out", Kind: KindFile, Head: []byte("mode: atomic\n")}, Remove},
		{"binary without artifact name", Entry{Rel: "bin/server", Kind: KindFile, Head: elf}, Remove},
		{"script named like artifact", Entry{Rel: "run.test", Kind: KindFile, Head: []byte("#!/bin/sh\n")}, Keep},
		{"text file", Entry{Rel: "notes.md", Kind: KindFile, Head: []byte("# notes")}, Keep},
		{"go source", Entry{Rel: "x.go", Kind: KindFile, Head: []byte("package x")}, Keep},
		{"symlink with artifact name", Entry{Rel: "x.test", Kind: KindSymlink}, Keep},
		{"dir", Entry{Rel: "nested/", Kind: KindDir}, Keep},
		{"other", Entry{Rel: "fifo.out", Kind: KindOther}, Keep},
		{"unsafe", Entry{Rel: "../x.test", Kind: KindFile, Head: elf}, Keep},
		{"under .loops", Entry{Rel: ".loops/x.test", Kind: KindFile, Head: elf}, Keep},
	} {
		if got := Decide(c.e); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
}

func TestPruneDirs(t *testing.T) {
	removed := []string{"build/out/app", "build/out/x.test", "pkg/p.test", "vendor/a/b.test", "node_modules/m/x.exe", "top.test"}
	baseDirs := map[string]bool{"pkg": true, "vendor": true, "vendor/a": true}
	got := PruneDirs(removed, baseDirs)
	if want := []string{"build/out", "build"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestSkipDir(t *testing.T) {
	for _, c := range []struct {
		rel  string
		want bool
	}{{".git", true}, {"a/.git", true}, {".loops", true}, {"a/.loops", false}, {"vendor", true}, {"x/node_modules", true}, {"src", false}} {
		if got := SkipDir(c.rel); got != c.want {
			t.Errorf("%s: %v", c.rel, got)
		}
	}
}

func TestReport(t *testing.T) {
	r := Report{
		CacheFiles: 10, CacheBytes: 3 << 20,
		Removed:     []Item{{Path: "a.test", Bytes: 1 << 20}, {Path: "cover.out", Bytes: 10}},
		Kept:        []string{"notes.md", "tool"},
		DirsRemoved: []string{"build"},
	}
	if r.Files() != 12 || r.Bytes() != 4<<20+10 {
		t.Fatal(r.Files(), r.Bytes())
	}
	if n := r.Note(); n != "12 files, 4.0 MB freed; 2 kept for review" {
		t.Fatal(n)
	}
	txt := r.Text()
	for _, s := range []string{"cleanup: 12 files, 4.0 MB freed; 2 kept for review", "go cache: 10 files, 3.0 MB", "removed a.test (1.0 MB)", "removed cover.out", "removed dir build", "left for review: notes.md", "left for review: tool"} {
		if !strings.Contains(txt, s) {
			t.Errorf("missing %q in\n%s", s, txt)
		}
	}
	r.DryRun, r.NoBaseline = true, true
	r.Refused = []string{"go: symlink"}
	txt = r.Text()
	for _, s := range []string{"cleanup (dry run): would remove 12 files", "would remove a.test", "no baseline", "refused: go: symlink"} {
		if !strings.Contains(txt, s) {
			t.Errorf("missing %q in\n%s", s, txt)
		}
	}
	if (Report{}).Note() != "0 files, 0.0 MB freed; 0 kept for review" {
		t.Fatal((Report{}).Note())
	}
}
