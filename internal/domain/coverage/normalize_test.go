package coverage

import (
	"reflect"
	"testing"
)

func lineProfile(format string, sources []string, files ...string) Profile {
	acc := map[blockKey]Block{}
	for i, f := range files {
		add(acc, f, lineBlock(i+1, i))
	}
	return Profile{Format: format, Sources: sources, Files: collect(acc)}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		p    Profile
		n    Normalizer
		want map[string]string
	}{
		{"gocover strips the module and drops other modules",
			lineProfile("", nil, "example.com/m/a/a.go", "other.com/z/z.go", "/abs/m/b.go", "example.com/m/../x.go"),
			Normalizer{PackRoot: "/abs/m", Module: "example.com/m"},
			map[string]string{"a/a.go": "1:0", "b.go": "3:2"}},
		{"gocover without a module keeps relative names",
			lineProfile("gocover", nil, "a/a.go", "./b.go"),
			Normalizer{PackRoot: "/abs/m"},
			map[string]string{"a/a.go": "1:0", "b.go": "2:1"}},
		{"lcov: Windows drive, backslashes, file:// and case-insensitive root",
			lineProfile("lcov", nil, `C:\Work\App\src\a.ts`, "file:///c:/work/app/src/b.ts", "file:///work/x.ts", `D:\Work\App\c.ts`, "../up.ts", "./", "src/./d.ts"),
			Normalizer{PackRoot: `c:\work\app\`},
			map[string]string{"src/a.ts": "1:0", "src/b.ts": "2:1", "src/d.ts": "7:6"}},
		{"no pack root drops absolute names",
			lineProfile("lcov", nil, "/abs/a.ts", "rel/b.ts"),
			Normalizer{},
			map[string]string{"rel/b.ts": "2:1"}},
		{"the root itself is not a file; root / strips the slash",
			lineProfile("lcov", nil, "/", "/x/y.c"),
			Normalizer{PackRoot: "/"},
			map[string]string{"x/y.c": "2:1"}},
		{"files landing on one path merge, highest count wins",
			Profile{Format: "lcov", Files: map[string][]Block{
				"/r/a.py": {lineBlock(1, 0), lineBlock(2, 5)},
				"./a.py":  {lineBlock(1, 3), lineBlock(2, 1)},
			}},
			Normalizer{PackRoot: "/r"},
			map[string]string{"a.py": "1:3 2:5"}},
		{"cobertura: absolute sources first, then relative ones, then the bare name",
			lineProfile("cobertura", []string{"src", "/r/lib/", "/elsewhere"}, "m/a.py", "n/b.py", "c.py"),
			Normalizer{PackRoot: "/r", Exists: exists("src/m/a.py", "c.py")},
			map[string]string{"src/m/a.py": "1:0", "lib/n/b.py": "2:1", "c.py": "3:2"}},
		{"cobertura without sources and outside candidates",
			lineProfile("cobertura", []string{"/elsewhere"}, "a.py", "../b.py"),
			Normalizer{PackRoot: "/r"},
			map[string]string{"a.py": "1:0"}},
		{"jacoco without source roots keeps the package path",
			lineProfile("jacoco", nil, "com/x/A.java", "B.java"),
			Normalizer{PackRoot: "/r"},
			map[string]string{"com/x/A.java": "1:0", "B.java": "2:1"}},
		{"jacoco source roots with backslashes; bare key when it exists",
			lineProfile("jacoco", nil, "com/x/A.java", "com/y/B.kt"),
			Normalizer{PackRoot: "/r", SourceRoots: []string{`mod\src\main\java`}, Exists: exists("com/y/B.kt")},
			map[string]string{"mod/src/main/java/com/x/A.java": "1:0", "com/y/B.kt": "2:1"}},
		{"unknown formats are taken as pack-relative",
			lineProfile("clover", nil, "/r/a.php"),
			Normalizer{PackRoot: "/r"},
			map[string]string{"a.php": "1:0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Normalize(tt.p, tt.n)
			if !got.Normalized || got.Format != tt.p.Format {
				t.Fatalf("%+v", got)
			}
			if g := lines(got); !reflect.DeepEqual(g, tt.want) {
				t.Fatalf("got %q\nwant %q", g, tt.want)
			}
			if again := Normalize(got, Normalizer{PackRoot: "/other", Module: "zzz"}); !reflect.DeepEqual(again, got) {
				t.Fatal("normalizing twice must be a no-op")
			}
		})
	}
}

func TestMergeProfilesMetadata(t *testing.T) {
	a := Profile{Format: "cobertura", Sources: []string{"/a", "/b"}, Normalized: true}
	b := Profile{Format: "lcov", Sources: []string{"/b", "/c"}}
	m := MergeProfiles(Profile{}, a, b)
	if m.Format != "cobertura" || !reflect.DeepEqual(m.Sources, []string{"/a", "/b", "/c"}) || m.Normalized {
		t.Fatalf("%+v", m)
	}
	if MergeProfiles().Normalized {
		t.Fatal("empty merge is not normalized")
	}
}
