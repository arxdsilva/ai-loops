package coverage

import (
	"errors"
	"reflect"
	"strings"
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

func TestParseLcov(t *testing.T) {
	text := "\ufeffVER:2.0\r\nTN:unit\r\nSF:a.ts\r\nFN:1,3,f\r\nFNL:0,1,3\r\nDA:1,1\r\nDA:2,0,abcd\r\nDA:3,1.5\r\nDA:4,-1\r\nBRDA:2,0,0,1\r\nLF:4\r\nLH:2\r\nend_of_record\r\n" +
		"TN:integration\nSF:a.ts\nDA:2,7\nDA:9,2e12\nend_of_record\nSF:b.ts\nDA:5,0\n"
	p, err := Parse("lcov", text)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"a.ts": "1:1 2:7 3:2 4:0 9:2147483647", "b.ts": "5:0"}
	if got := lines(p); !reflect.DeepEqual(got, want) {
		t.Fatalf("%q", got)
	}
	for _, bad := range []struct{ text, want string }{
		{"DA:1,1\n", "line 1: DA outside an SF record"},
		{"SF:a\nend_of_record\nDA:1,1\n", "line 3: DA outside"},
		{"SF:\n", "line 1: empty SF"},
		{"SF:a\nDA:1\n", "line 2: want DA:line,hits"},
		{"SF:a\nDA:x,1\n", "line 2: bad line number"},
		{"SF:a\nDA:0,1\n", "line 2: bad line number"},
		{"SF:a\nDA:1,x\n", "line 2: bad hit count"},
		{"SF:a\nDA:1,NaN\n", "line 2: bad hit count"},
	} {
		if _, err := Parse("lcov", bad.text); err == nil || !strings.Contains(err.Error(), bad.want) {
			t.Errorf("%q: err %v, want %q", bad.text, err, bad.want)
		}
	}
}

func TestParseCobertura(t *testing.T) {
	text := `<?xml version="1.0" encoding="ISO-8859-1"?>
<!DOCTYPE coverage SYSTEM "http://cobertura.sourceforge.net/xml/coverage-04.dtd">
<coverage><sources><source> C:\src\app\ </source><source></source><source>C:\src\app\</source></sources>
<packages><package name="p"><classes>
<class name="A" filename="lib\caf` + "\xe9" + `.py"><methods><method><lines><line number="99" hits="1"/></lines></method></methods>
<lines><line number="2" hits="0"/><line number="1" hits="3"/></lines></class>
<class name="A$Inner" filename="lib\caf` + "\xe9" + `.py"><lines><line number="2" hits="4"/></lines></class>
<class name="Empty" filename=""><lines><line number="1" hits="1"/></lines></class>
</classes></package></packages></coverage>`
	p, err := Parse("cobertura", text)
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{`lib\café.py`: "1:3 2:4"}; !reflect.DeepEqual(lines(p), want) {
		t.Fatalf("%q", lines(p))
	}
	if !reflect.DeepEqual(p.Sources, []string{`C:\src\app\`}) {
		t.Fatalf("sources %q", p.Sources)
	}
	n := Normalize(p, Normalizer{PackRoot: `c:\SRC\app`})
	if want := map[string]string{"lib/café.py": "1:3 2:4"}; !reflect.DeepEqual(lines(n), want) {
		t.Fatalf("normalized %q", lines(n))
	}
	for _, bad := range []struct{ text, want string }{
		{"", "coverage report: EOF"},
		{"<report/>", "root element <report>, want <coverage>"},
		{"<coverage><packages><package><classes><class filename='a'><lines><line number='x' hits='1'/></lines></class></classes></package></packages></coverage>", "a: bad line number"},
		{"<coverage><packages><package><classes><class filename='a'><lines><line number='1' hits='?'/></lines></class></classes></package></packages></coverage>", "a:1: bad hit count"},
		{"<coverage><packages></coverage>", "coverage report:"},
	} {
		if _, err := Parse("cobertura", bad.text); err == nil || !strings.Contains(err.Error(), bad.want) {
			t.Errorf("%q: err %v, want %q", bad.text, err, bad.want)
		}
	}
}

func TestParseJacoco(t *testing.T) {
	text := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><!DOCTYPE report PUBLIC "-//JACOCO//DTD Report 1.1//EN" "report.dtd">
<report name="agg"><group name="mod-a"><group name="inner"><package name="a/b"><sourcefile name="X.java">
<line nr="3" mi="1" ci="2"/><line nr="4" mi="0" ci="0"/><line nr="5" mi="4"/></sourcefile></package></group></group>
<package name=""><sourcefile name="Top.java"><line nr="1" mi="0" ci="1"/></sourcefile><sourcefile name=""><line nr="1" ci="1"/></sourcefile></package>
<package name="a/b"><sourcefile name="X.java"><line nr="5" mi="0" ci="9"/></sourcefile></package></report>`
	p, err := Parse("jacoco", text)
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"a/b/X.java": "3:2 5:9", "Top.java": "1:1"}; !reflect.DeepEqual(lines(p), want) {
		t.Fatalf("%q", lines(p))
	}
	for _, bad := range []struct{ text, want string }{
		{"<coverage/>", "root element <coverage>, want <report>"},
		{`<report><package name="p"><sourcefile name="S.kt"><line nr="" ci="1"/></sourcefile></package></report>`, "p/S.kt: bad line number"},
		{`<report><group><package name="p"><sourcefile name="S.kt"><line nr="1" ci="z"/></sourcefile></package></group></report>`, "p/S.kt:1: bad instruction counts"},
	} {
		if _, err := Parse("jacoco", bad.text); err == nil || !strings.Contains(err.Error(), bad.want) {
			t.Errorf("%q: err %v, want %q", bad.text, err, bad.want)
		}
	}
}

func TestParseErrorPropagates(t *testing.T) {
	if _, err := Parse("gocover", "mode: set\nbad\n"); err == nil {
		t.Fatal("want error")
	}
}
