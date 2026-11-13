package hook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// edited is one edited file and the pack that owns it.
type edited struct {
	file string // absolute
	det  langpack.Detected
	root string // absolute pack root
	rel  string // slash path relative to root
}

// stepName is the step event and message name: go-post-edit for Go (older
// reports compare it), post-edit for every other pack.
func (e edited) stepName() string {
	if e.det.ID == "go" {
		return "go-post-edit"
	}
	return "post-edit"
}

// postEdit (alias go-post-edit) runs the owning pack's post_edit commands
// on each edited file: format_file on the file, then lint_dir once per
// distinct command, both in the pack root. A file no pack owns, a pack with
// no post_edit, or a tool not on PATH is skipped. A failure is sent back to
// the agent; formatting failures skip linting.
func (h *handler) postEdit(ctx context.Context, d dialect, p payload, out io.Writer) {
	var in toolInput
	_ = json.Unmarshal(p.ToolInput, &in)
	repo := h.repo(p)
	var paths []string
	for _, f := range append([]string{in.path()}, p.extraFiles...) {
		if f == "" {
			continue
		}
		if !filepath.IsAbs(f) {
			f = filepath.Join(repo, f)
		}
		if !slices.Contains(paths, f) && h.d.Files.Exists(f) {
			paths = append(paths, f)
		}
	}
	if len(paths) == 0 {
		return
	}
	ds, ok := h.detect(p)
	if !ok {
		ds = []langpack.Detected{legacyGo}
	}
	var files []edited
	for _, f := range paths {
		if det, root, rel, ok := owner(ds, repo, f); ok && det.Pack.PostEdit != nil {
			files = append(files, edited{file: f, det: det, root: root, rel: rel})
		}
	}
	if len(files) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	failed := map[string]bool{}
	var failMsg string
	fail := func(e edited, argv []string, output string) {
		if failMsg != "" {
			return
		}
		lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
		if len(lines) > 30 {
			lines = lines[:30]
		}
		failMsg = fmt.Sprintf("%s: %s failed for %s:\n%s", e.stepName(), toolLabel(argv), e.file, strings.Join(lines, "\n"))
	}
	for _, e := range files {
		argv := h.postEditArgv(e, e.det.Pack.PostEdit.FormatFile)
		if argv == nil {
			continue
		}
		if output, err := h.d.Cmd.Run(ctx, e.root, argv[0], argv[1:]...); err != nil {
			failed[e.file] = true
			fail(e, argv, output)
		}
	}
	if failMsg == "" {
		done := map[string]bool{}
		for _, e := range files {
			argv := h.postEditArgv(e, e.det.Pack.PostEdit.LintDir)
			key := e.root + "\x00" + strings.Join(argv, "\x00")
			if argv == nil || done[key] {
				continue
			}
			done[key] = true
			if output, err := h.d.Cmd.Run(ctx, e.root, argv[0], argv[1:]...); err != nil {
				for _, g := range files {
					if g.root == e.root && slices.Equal(h.postEditArgv(g, g.det.Pack.PostEdit.LintDir), argv) {
						failed[g.file] = true
					}
				}
				fail(e, argv, output)
			}
		}
	}
	for _, e := range files {
		_ = h.d.Rec.Record(ctx, run.Event{Kind: run.KindStep, Name: e.stepName(), Agent: p.AgentID, Role: roleName(p.AgentType), Target: e.file, OK: run.Bool(!failed[e.file])})
	}
	if failMsg != "" {
		d.postEditFailure(out, failMsg)
	}
}

// postEditArgv fills tmpl's placeholders for e; nil means skip: an empty
// template, a placeholder that cannot be filled, or a tool not on PATH.
// Go keeps the Go-only hook's behaviour: goimports replaces gofmt when it
// is on PATH, and the Go toolchain's own tools are not looked up.
func (h *handler) postEditArgv(e edited, tmpl []string) []string {
	if len(tmpl) == 0 {
		return nil
	}
	dir := filepath.ToSlash(filepath.Dir(filepath.FromSlash(e.rel)))
	manifest := h.manifestDir(e)
	vals := map[string][]string{"file": {e.file}, "dir": {dir}, "manifest_dir": {manifest}}
	for _, name := range pack.Placeholders(tmpl) {
		if _, ok := vals[name]; ok {
			continue
		}
		if v, ok := h.filePlaceholder(e, name, vals); ok {
			vals[name] = []string{v}
		}
	}
	argv, err := pack.Expand(tmpl, vals)
	if err != nil || len(argv) == 0 {
		return nil
	}
	if e.det.ID == "go" {
		if argv[0] == "gofmt" && h.d.Cmd.Has("goimports") {
			argv[0] = "goimports"
		}
		return argv
	}
	if !h.d.Cmd.Has(argv[0]) {
		return nil
	}
	return argv
}

// manifestDir is the nearest directory from the file's up to the pack root
// that holds the pack's manifest (the pack root when none does).
func (h *handler) manifestDir(e edited) string {
	if e.det.Pack.Manifest == "" {
		return e.root
	}
	for dir := filepath.Dir(e.file); ; dir = filepath.Dir(dir) {
		if h.d.Files.Exists(filepath.Join(dir, e.det.Pack.Manifest)) {
			return dir
		}
		if dir == e.root || dir == filepath.Dir(dir) || !strings.HasPrefix(dir, e.root) {
			return e.root
		}
	}
}

// filePlaceholder resolves a pack placeholder of the file+regex kind: the
// first submatch of Regex in File (relative paths are under the pack root).
func (h *handler) filePlaceholder(e edited, name string, vals map[string][]string) (string, bool) {
	ph, ok := e.det.Pack.Placeholders[name]
	if !ok || ph.File == "" || ph.Regex == "" {
		return "", false
	}
	f, err := pack.Expand([]string{ph.File}, vals)
	if err != nil || len(f) != 1 {
		return "", false
	}
	path := filepath.FromSlash(f[0])
	if !filepath.IsAbs(path) {
		path = filepath.Join(e.root, path)
	}
	read := h.d.ReadFile
	if read == nil {
		read = os.ReadFile
	}
	b, err := read(path)
	if err != nil {
		return "", false
	}
	re, err := regexp.Compile(ph.Regex)
	if err != nil {
		return "", false
	}
	m := re.FindStringSubmatch(string(b))
	if len(m) < 2 {
		return "", false
	}
	return m[1], true
}

// toolLabel names a command in messages: its leading plain words
// ("go vet", "cargo clippy", "gofmt").
func toolLabel(argv []string) string {
	n := 1
	for n < len(argv) && n < 3 && !strings.ContainsAny(argv[n], "-./{=") {
		n++
	}
	return strings.Join(argv[:n], " ")
}
