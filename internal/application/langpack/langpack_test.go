package langpack

import (
	"errors"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/domain/pack"
)

const tmpl = `{"schema":1,"id":"%s","name":"X","versions":{"as_of":"2026-11"},"detect":[%s],%s
 "source_globs":["**/*.x"],"test_globs":["**/*_test.x"],"generated":{},
 "checks":[{"name":"test","kind":"test","argv":["x","test"],"required":true}],
 "test_commands":[{"argv_prefix":["x","test"]}],
 "race":{"mode":"none","checks":["test"],"substitute":"random order"},
 "coverage":{"format":"lcov","path":"{coverage}","produced_by":["test"]},
 "post_edit":null,"cache_env":{},"artifacts":[],"arch":{"parser":"none"}%s}`

func packJSON(id, detect, extra, tail string) []byte {
	s := strings.Replace(strings.Replace(strings.Replace(tmpl, "%s", id, 1), "%s", detect, 1), "%s", extra, 1)
	return []byte(strings.Replace(s, "%s", tail, 1))
}

type src struct {
	files map[string][]byte
	err   error
}

func (s src) PackFiles() (map[string][]byte, error) { return s.files, s.err }

type tree struct {
	files map[string]bool
	err   error
}

type view struct {
	t    tree
	base string
}

func (v view) full(rel string) string {
	if v.base == "." {
		return rel
	}
	return v.base + "/" + rel
}
func (v view) Exists(rel string) bool {
	f := v.full(rel)
	for k := range v.t.files {
		if k == f || strings.HasPrefix(k, f+"/") {
			return true
		}
	}
	return false
}
func (v view) Read(rel string) (string, bool) { return "", v.Exists(rel) }
func (v view) List(string) []string           { return nil }

func (t tree) View(_, dir string) pack.View { return view{t: t, base: dir} }
func (t tree) Dirs(string, int, func(string) bool) ([]string, error) {
	if t.err != nil {
		return nil, t.err
	}
	seen := map[string]bool{".": true}
	out := []string{"."}
	for f := range t.files {
		parts := strings.Split(f, "/")
		for i := 1; i < len(parts); i++ {
			d := strings.Join(parts[:i], "/")
			if !seen[d] {
				seen[d] = true
				out = append(out, d)
			}
		}
	}
	return out, nil
}

func service(t tree) *Service {
	return New(src{files: map[string][]byte{
		"specs/languages/go/pack.json":     packJSON("go", `{"file":"go.mod"}`, "", ""),
		"specs/languages/java/pack.json":   packJSON("java", `{"file":"pom.xml"},{"file":"build.gradle.kts"}`, `"detect_not":[{"file":"src/main/kotlin"}],`, `,"variants":[{"name":"gradle","detect":[{"file":"build.gradle.kts"}]}]`),
		"specs/languages/kotlin/pack.json": packJSON("kotlin", `{"file":"src/main/kotlin"}`, "", ""),
	}}, t)
}

func TestDetectPolyglot(t *testing.T) {
	s := service(tree{files: map[string]bool{
		"go.mod": true, "internal/a.go": true,
		"services/billing/pom.xml": true, "services/billing/mod-a/pom.xml": true,
		"services/app/build.gradle.kts": true, "services/app/src/main/kotlin/A.kt": true,
		"tools/gen/build.gradle.kts": true,
	}})
	ds, err := s.Detect("/repo", nil)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range ds {
		got = append(got, d.ID+"@"+d.Root+":"+d.Variant)
	}
	want := "go@.: kotlin@services/app: java@services/billing: java@tools/gen:gradle"
	if strings.Join(got, " ") != want {
		t.Fatalf("got  %v\nwant %v", strings.Join(got, " "), want)
	}
}

func TestOverridesAndGet(t *testing.T) {
	s := service(tree{files: map[string]bool{"pom.xml": true}})
	ds, err := s.Detect("/repo", []Override{{ID: "go", Root: "./"}, {ID: "java", Root: "../x", Variant: "gradle"}, {ID: "kotlin", Root: "svc"}})
	if err != nil || len(ds) != 3 || ds[0].Root != "." || ds[1].Root != "." || ds[1].Variant != "gradle" || ds[2].Root != "svc" {
		t.Fatalf("%+v %v", ds, err)
	}
	if _, err := s.Detect("/repo", []Override{{ID: "cobol"}}); err == nil {
		t.Fatal("unknown pack accepted")
	}
	if _, err := s.Detect("/repo", []Override{{ID: "java", Variant: "ant"}}); err == nil {
		t.Fatal("unknown variant accepted")
	}
	if p, err := s.Get("kotlin"); err != nil || p.ID != "kotlin" {
		t.Fatal(err)
	}
	if _, err := s.Get("cobol"); err == nil || !strings.Contains(err.Error(), "go, java, kotlin") {
		t.Fatalf("%v", err)
	}
}

func TestAllErrors(t *testing.T) {
	bad := New(src{files: map[string][]byte{
		"specs/languages/go/pack.json": packJSON("go", `{"file":"go.mod"}`, "", ""),
		"specs/languages/py/pack.json": packJSON("python", `{"file":"pyproject.toml"}`, "", ""),
		"specs/languages/x/pack.json":  []byte(`{`),
		"other/go/pack.json":           packJSON("go", `{"file":"go.mod"}`, "", ""),
	}}, tree{})
	_, err := bad.All()
	for _, want := range []string{"does not match its directory", "already defined", "specs/languages/x/pack.json"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in %v", want, err)
		}
	}
	if _, err := New(src{err: errors.New("boom")}, tree{}).All(); err == nil {
		t.Fatal("source error swallowed")
	}
	if _, err := New(src{err: errors.New("boom")}, tree{}).Get("go"); err == nil {
		t.Fatal("source error swallowed by Get")
	}
	if _, err := New(src{err: errors.New("boom")}, tree{}).Detect("/", nil); err == nil {
		t.Fatal("source error swallowed by Detect")
	}
	if _, err := service(tree{err: errors.New("walk")}).Detect("/", nil); err == nil {
		t.Fatal("tree error swallowed")
	}
	if !SkipDir(".git") || !SkipDir("node_modules") || SkipDir("services") {
		t.Fatal("SkipDir")
	}
}
