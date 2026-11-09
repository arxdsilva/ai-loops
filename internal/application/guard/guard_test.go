package guard_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insurgencygames/ai-loops/internal/adapters/outbound/fsys"
	"github.com/insurgencygames/ai-loops/internal/application/guard"
)

func newRepo(t *testing.T) string {
	t.Helper()
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, n := range map[string]int{"pkg/small.go": 10, "pkg/big.go": 600, "pkg/other.go": 10, "pkg/medium.go": 100, "pic.png": 600} {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(strings.Repeat("x\n", n)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

func input(t *testing.T, v map[string]any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestRead(t *testing.T) {
	repo := newRepo(t)
	g := guard.New(fsys.New(), guard.DefaultConfig())
	read := func(name string, extra map[string]any) string {
		path := name
		if !strings.HasPrefix(name, "/") {
			path = filepath.Join(repo, name)
		}
		in := map[string]any{"file_path": path}
		for k, v := range extra {
			in[k] = v
		}
		return g.CheckRead("Read", input(t, in), repo, repo)
	}
	tests := []struct {
		name  string
		file  string
		extra map[string]any
		want  string // "" = allow, else substring
	}{
		{"test_whole_big_file_is_denied", "pkg/big.go", nil, "600 lines"},
		{"test_whole_big_file_is_denied message", "pkg/big.go", nil, "read_guard: pkg/big.go has 600 lines. Use Read with offset and limit, after grep -n finds the range, or the loops:code-researcher agent for a broad question."},
		{"test_ranged_read_of_big_file_passes offset+limit", "pkg/big.go", map[string]any{"offset": 1, "limit": 50}, ""},
		{"test_ranged_read_of_big_file_passes limit", "pkg/big.go", map[string]any{"limit": 50}, ""},
		{"test_small_file_passes", "pkg/small.go", nil, ""},
		{"test_binary_and_outside_paths_pass png", "pic.png", nil, ""},
		{"test_binary_and_outside_paths_pass outside", "/tmp/elsewhere.txt", nil, ""},
		{"test_binary_and_outside_paths_pass missing", "pkg/missing.go", nil, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := read(tc.file, tc.extra)
			if tc.want == "" && got != "" || tc.want != "" && !strings.Contains(got, tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBash(t *testing.T) {
	repo := newRepo(t)
	pkg := filepath.Join(repo, "pkg")
	g := guard.New(fsys.New(), guard.DefaultConfig())
	bash := func(cmd, cwd string) string {
		if cwd == "" {
			cwd = repo
		}
		return g.CheckRead("Bash", input(t, map[string]any{"command": cmd}), cwd, repo)
	}
	type row struct {
		name string
		cmds []string
		cwd  string
		deny bool
	}
	rows := []row{
		{"test_long_views_are_denied", []string{
			"cat pkg/big.go", "cat -n pkg/big.go", "cat pkg/*.go", "sed -n '1,200p' pkg/big.go",
			"sed -n -e '1,200p' pkg/big.go", "sed -n '100,$p' pkg/big.go", "sed -n '1,+200p' pkg/big.go",
			"echo ---; cat pkg/big.go", "ls && sed -n 1,200p pkg/big.go", "(cat pkg/big.go)"}, "", true},
		{"test_views_add_up_across_a_command", []string{
			"sed -n 1,100p pkg/big.go; sed -n 200,300p pkg/big.go",
			"sed -n '1,100p;200,300p' pkg/big.go",
			"cat pkg/medium.go; cat pkg/medium.go"}, "", true},
		{"test_short_views_pass", []string{
			"cat pkg/small.go", "cat pkg/small.go pkg/other.go", "cat pkg/medium.go",
			"sed -n '10,20p' pkg/big.go", "sed -n 1,150p pkg/big.go", "sed -n 5p pkg/big.go",
			"sed -n '/func/,/^}/p' pkg/big.go"}, "", false},
		{"test_cd_is_followed abs", []string{"cd " + repo + "/pkg && cat big.go"}, "", true},
		{"test_cd_is_followed chain", []string{"cd " + repo + "; cd pkg; cat big.go"}, "", true},
		{"test_cd_is_followed cwd", []string{"cat ../pkg/big.go"}, pkg, true},
		{"test_pipes_that_do_not_shrink_are_denied", []string{
			"cat -n pkg/big.go | sed -n 1,400p", "cat pkg/*.go | grep -v '^//'"}, "", true},
		{"test_filters_pass", []string{
			"cat pkg/big.go | grep foo", "cat pkg/big.go | head -20", "sed -n 1,500p pkg/big.go | grep -n foo",
			"cat pkg/big.go | wc -l", "grep -n foo pkg/big.go", "grep -rn foo pkg | head"}, "", false},
		{"test_writes_pass", []string{
			"cat pkg/big.go > /tmp/copy.go", "sed -i '' 's/a/b/' pkg/big.go", "sed -i.bak -n 1,300p pkg/big.go",
			"cat > pkg/new.go <<'EOF'\ncat pkg/big.go\nEOF", "cat <<EOF\nsed -n 1,300p pkg/big.go\nEOF"}, "", false},
		{"test_unresolved_and_outside_paths_pass", []string{
			"cat /tmp/elsewhere.txt", "cat pkg/missing.go", `cat "$F"`, "cd $DIR && cat pkg/big.go",
			`for f in a b; do cat "$f"; done`, "cd /tmp && cat pkg/big.go"}, "", false},
		{"test_quotes_and_comments_are_not_commands", []string{
			"echo 'cat pkg/big.go'", "git log --grep='cat pkg/big.go; x'", "# cat pkg/big.go\nls",
			"python3 -c 'print(1)\ncat pkg/big.go'"}, "", false},
		{"test_substitution_stays_allowed", []string{`X="$(cat pkg/big.go | wc -l)"; echo "$X"`}, "", false},
		{"test_bad_quoting_fails_open", []string{"cat 'pkg/big.go"}, "", false},
		// extras beyond the Python suite
		{"leading keywords and env prefixes", []string{"FOO=1 cat pkg/big.go", "if true; then cat pkg/big.go; fi"}, "", true},
		{"heredoc then view", []string{"cat <<EOF\nhi\nEOF\ncat pkg/big.go"}, "", true},
		{"sed 100,+49 is 50 lines", []string{"sed -n '100,+49p' pkg/big.go"}, "", false},
		{"sed single-quote escapes", []string{`cat pkg/\big.go`, `cat "pkg/big.go"`}, "", true},
		{"sed -f script is not a view", []string{"sed -n -f s.sed pkg/big.go"}, "", false},
		{"unknown cd stops the scan", []string{"cd - && cat pkg/big.go", "cd ~ && cat pkg/big.go", "cd"}, "", false},
	}
	if err := os.MkdirAll(filepath.Join(repo, "dot"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "dot", ".env.big"), []byte(strings.Repeat("x\n", 600)), 0o644); err != nil {
		t.Fatal(err)
	}
	rows = append(rows, row{"glob star skips dotfiles", []string{"cat dot/*", "cat dot/*.big"}, "", false})
	rows = append(rows, row{"glob explicit dot matches", []string{"cat dot/.env*"}, "", true})
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			for _, c := range r.cmds {
				if got := bash(c, r.cwd); (got != "") != r.deny {
					t.Errorf("%q: deny=%v got %q", c, r.deny, got)
				}
			}
		})
	}

	t.Run("message", func(t *testing.T) {
		want := "read_guard: this command prints about 600 lines of repo files (pkg/big.go). Use Read with offset and limit for the part you need, grep -n to find it, or the loops:code-researcher agent for a broad question. Several Reads can go in one message."
		if got := bash("cat pkg/big.go", ""); got != want {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("test_other_tools_pass", func(t *testing.T) {
		if got := g.CheckRead("Edit", input(t, map[string]any{"file_path": "x"}), repo, repo); got != "" {
			t.Fatal(got)
		}
	})
}

func TestConfig(t *testing.T) {
	repo := newRepo(t)
	big := input(t, map[string]any{"file_path": filepath.Join(repo, "pkg/big.go")})
	cat := input(t, map[string]any{"command": "cat pkg/big.go"})
	f := fsys.New()
	if guard.New(f, guard.Config{Off: true}).CheckRead("Read", big, repo, repo) != "" {
		t.Error("Off should allow Read")
	}
	if guard.New(f, guard.Config{Off: true}).CheckRead("Bash", cat, repo, repo) != "" {
		t.Error("Off should allow Bash")
	}
	if guard.New(f, guard.Config{}).CheckRead("Read", big, repo, repo) == "" {
		t.Error("zero MinLines should default to 500")
	}
	if guard.New(f, guard.Config{}).CheckRead("Bash", cat, repo, repo) == "" {
		t.Error("zero MaxViewLines should default to 150")
	}
	if guard.New(f, guard.Config{MinLines: 700}).CheckRead("Read", big, repo, repo) != "" {
		t.Error("MinLines 700 should allow 600 lines")
	}
	if guard.New(f, guard.Config{MaxViewLines: 700}).CheckRead("Bash", cat, repo, repo) != "" {
		t.Error("MaxViewLines 700 should allow 600 lines")
	}
	if guard.New(f, guard.Config{}).CheckRead("Bash", json.RawMessage("not json"), repo, repo) != "" {
		t.Error("bad JSON fails open")
	}
}

type errFiles struct{}

func (errFiles) Lines(string) (int, error)     { return 0, os.ErrPermission }
func (errFiles) Exists(string) bool            { return true }
func (errFiles) Glob(string) ([]string, error) { return nil, nil }

func TestLinesErrorFailsOpen(t *testing.T) {
	g := guard.New(errFiles{}, guard.DefaultConfig())
	for _, tc := range []struct{ tool, key, val string }{{"Read", "file_path", "/r/a.go"}, {"Bash", "command", "cat a.go"}} {
		if got := g.CheckRead(tc.tool, input(t, map[string]any{tc.key: tc.val}), "/r", "/r"); got != "" {
			t.Errorf("%s: %q", tc.tool, got)
		}
	}
}

func TestModel(t *testing.T) {
	const want = "model_guard: pass model for this agent: haiku for mechanical work, sonnet for implementation against a contract, opus for hard reasoning (ROLES.md, Model tiers)."
	tests := []struct {
		name string
		in   string
		deny bool
	}{
		{"test_untyped_without_model_is_denied prompt", `{"prompt":"x"}`, true},
		{"test_untyped_without_model_is_denied general-purpose", `{"subagent_type":"general-purpose"}`, true},
		{"empty type, empty model", `{"subagent_type":"","model":""}`, true},
		{"null input", `null`, true},
		{"test_named_model_passes untyped", `{"model":"sonnet"}`, false},
		{"test_named_model_passes general-purpose", `{"subagent_type":"general-purpose","model":"opus"}`, false},
		{"test_typed_agents_and_forks_pass Explore", `{"subagent_type":"Explore"}`, false},
		{"test_typed_agents_and_forks_pass Plan", `{"subagent_type":"Plan"}`, false},
		{"test_typed_agents_and_forks_pass fork", `{"subagent_type":"fork"}`, false},
		{"test_typed_agents_and_forks_pass web-fetch", `{"subagent_type":"web-fetch"}`, false},
		{"loops typed", `{"subagent_type":"loops:code-researcher"}`, false},
		{"bad json fails open", `{`, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := guard.CheckModel(json.RawMessage(tc.in))
			if tc.deny && got != want || !tc.deny && got != "" {
				t.Fatalf("got %q", got)
			}
		})
	}
}
