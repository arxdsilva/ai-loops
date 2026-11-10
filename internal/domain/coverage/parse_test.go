package coverage

import (
	"errors"
	"testing"
)

func TestParseFormats(t *testing.T) {
	p, err := Parse("gocover", "mode: atomic\nm/a.go:1.1,2.2 1 1\n")
	if err != nil || len(p.Files["m/a.go"]) != 1 {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := Parse("", "mode: set\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse("clover", ""); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("want ErrUnsupported, got %v", err)
	}
}

func TestCandidatesPackGlobs(t *testing.T) {
	ch := Changes{
		"src/app/a.py":         {Ranges: WholeFile()},
		"tests/test_a.py":      {Ranges: WholeFile()},
		"src/app/gen/x_pb2.py": {Ranges: WholeFile()},
		"cmd/tool.py":          {Ranges: WholeFile()},
		"main.go":              {Ranges: WholeFile()},
	}
	o := Options{SourceGlobs: []string{"**/*.py"}, TestGlobs: []string{"tests/**", "**/test_*.py"}, BaseExclude: []string{"**/*_pb2.py"}}
	got := Candidates(ch, o)
	if len(got) != 2 || got[0] != "cmd/tool.py" || got[1] != "src/app/a.py" {
		t.Fatalf("%v", got)
	}
	if got := Candidates(ch, Options{}); len(got) != 1 || got[0] != "main.go" {
		t.Fatalf("Go defaults: %v", got)
	}
}
