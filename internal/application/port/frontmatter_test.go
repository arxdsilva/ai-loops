package port

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	doc := "---\r\nname: work\r\n\r\ndescription: \"a \\\"b\\\" \\u00e9\\n\"\nhint: 'it''s'\nempty:\nplain: a: b\n---\n\n\nBody\n---\nmore\n"
	fm, body, err := parseFrontmatter("x.md", []byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	want := frontmatter{"name": "work", "description": "a \"b\" é\n", "hint": "it's", "empty": "", "plain": "a: b"}
	if !reflect.DeepEqual(fm, want) {
		t.Fatalf("fm = %#v", fm)
	}
	if body != "Body\n---\nmore\n" {
		t.Fatalf("body = %q", body)
	}
}

func TestParseFrontmatterErrors(t *testing.T) {
	cases := map[string]string{
		"no open":           "name: x\n",
		"unterminated":      "---\nname: x\n",
		"no colon":          "---\nname x\n---\n",
		"bad key":           "---\nna me: x\n---\n",
		"empty key":         "---\n: x\n---\n",
		"duplicate":         "---\nname: x\nname: y\n---\n",
		"open dq":           "---\nname: \"x\n---\n",
		"bad escape":        "---\nname: \"\\q\"\n---\n",
		"lone dq":           "---\nname: \"\n---\n",
		"open sq":           "---\nname: 'x\n---\n",
		"lone sq":           "---\nname: '\n---\n",
		"bad sq":            "---\nname: 'a'b'\n---\n",
		"block scalar":      "---\nname: |\n---\n",
		"folded scalar":     "---\nname: >\n---\n",
		"nested indent key": "---\n  - item\n---\n",
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, err := parseFrontmatter("f.md", []byte(doc))
			if err == nil || !strings.HasPrefix(err.Error(), "f.md") {
				t.Fatalf("err = %v, want an f.md error", err)
			}
		})
	}
}

func TestList(t *testing.T) {
	for in, want := range map[string][]string{
		"":                    nil,
		"  ":                  nil,
		"Edit":                {"Edit"},
		"Edit, NotebookEdit":  {"Edit", "NotebookEdit"},
		"[Read,Grep , Glob ]": {"Read", "Grep", "Glob"},
		"[]":                  nil,
	} {
		got, err := list(in)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("list(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"a,,b", "a,", "[a, b"} {
		if _, err := list(in); err == nil {
			t.Errorf("list(%q) succeeded", in)
		}
	}
}
