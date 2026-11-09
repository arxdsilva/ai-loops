package coverage

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseProfile(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		want    map[string][]Block
		mode    string
		wantErr string
	}{
		{
			name: "single",
			text: "mode: atomic\nexample.com/m/a/a.go:4.2,4.11 1 1\nexample.com/m/a/a.go:7.2,7.10 2 0\n",
			mode: "atomic",
			want: map[string][]Block{"example.com/m/a/a.go": {
				{StartLine: 4, StartCol: 2, EndLine: 4, EndCol: 11, NumStmt: 1, Count: 1},
				{StartLine: 7, StartCol: 2, EndLine: 7, EndCol: 10, NumStmt: 2, Count: 0},
			}},
		},
		{
			name: "CRLF and blank lines",
			text: "mode: set\r\n\r\nm/a.go:1.1,2.2 3 1\r\n",
			mode: "set",
			want: map[string][]Block{"m/a.go": {{1, 1, 2, 2, 3, 1}}},
		},
		{
			name: "merged profiles take the max count and sort blocks",
			text: "mode: atomic\nm/a.go:9.1,9.5 1 0\nm/a.go:3.1,4.2 2 0\nmode: atomic\nm/a.go:3.1,4.2 2 5\nm/b.go:1.1,1.9 1 0\nm/a.go:3.1,4.2 2 1\n",
			mode: "atomic",
			want: map[string][]Block{
				"m/a.go": {{3, 1, 4, 2, 2, 5}, {9, 1, 9, 5, 1, 0}},
				"m/b.go": {{1, 1, 1, 9, 1, 0}},
			},
		},
		{
			name: "windows-ish path with a colon",
			text: "mode: count\nC:/m/a.go:1.1,1.2 1 1\n",
			mode: "count",
			want: map[string][]Block{"C:/m/a.go": {{1, 1, 1, 2, 1, 1}}},
		},
		{name: "empty", text: "", want: map[string][]Block{}},
		{name: "no colon", text: "mode: set\nfoo 1 1\n", wantErr: "line 2"},
		{name: "bad fields", text: "mode: set\na.go:1.1,2.2 1\n", wantErr: "line 2"},
		{name: "bad position", text: "mode: set\na.go:1.1-2.2 1 1\n", wantErr: "line 2"},
		{name: "bad number", text: "mode: set\na.go:1.x,2.2 1 1\n", wantErr: "line 2"},
		{name: "bad stmt count", text: "mode: set\na.go:1.1,2.2 x 1\n", wantErr: "line 2"},
		{name: "bad count", text: "mode: set\na.go:1.1,2.2 1 -\n", wantErr: "line 2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := ParseProfile(tt.text)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if p.Mode != tt.mode || !reflect.DeepEqual(p.Files, tt.want) {
				t.Fatalf("got %q %+v", p.Mode, p.Files)
			}
		})
	}
}

func TestMergeProfiles(t *testing.T) {
	a, _ := ParseProfile("mode: atomic\nm/a.go:1.1,2.1 1 0\nm/a.go:5.1,6.1 1 2\n")
	b, _ := ParseProfile("mode: atomic\nm/a.go:1.1,2.1 1 3\nm/c.go:1.1,1.4 1 0\n")
	got := MergeProfiles(a, b)
	want := map[string][]Block{
		"m/a.go": {{1, 1, 2, 1, 1, 3}, {5, 1, 6, 1, 1, 2}},
		"m/c.go": {{1, 1, 1, 4, 1, 0}},
	}
	if got.Mode != "atomic" || !reflect.DeepEqual(got.Files, want) {
		t.Fatalf("%+v", got)
	}
	if e := MergeProfiles(); e.Files == nil || len(e.Files) != 0 {
		t.Fatalf("%+v", e)
	}
}
