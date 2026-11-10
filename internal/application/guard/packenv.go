package guard

import (
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/pack"
	"github.com/arxdsilva/ai-loops/internal/domain/workspace"
)

// PackScope is how one pack's tools are scoped to a run (TOOLING §5).
type PackScope struct {
	// Tools are the command words that belong to the pack.
	Tools []string
	// Env is the run-scoped environment (KEY=value).
	Env []string
	// Argv maps a tool (one or more words, space-separated) to the flags
	// inserted after it (cache_argv, placeholders already filled).
	Argv map[string][]string
}

// NewPackScope builds the scope of pack p for the run in runDir: the env is
// workspace.PackEnv of its cache_env, then its env (cache_env wins on a
// clash); {run}, {cache} and {tmp} in cache_argv are filled the same way.
// Flags whose placeholders cannot be filled are dropped.
func NewPackScope(p pack.Pack, runDir string) PackScope {
	s := PackScope{Tools: PackTools(p), Argv: map[string][]string{}}
	seen := map[string]bool{}
	for _, kv := range append(workspace.PackEnv(runDir, p.ID, p.CacheEnv), workspace.PackEnv(runDir, p.ID, p.Env)...) {
		k, _, _ := strings.Cut(kv, "=")
		if !seen[k] {
			seen[k] = true
			s.Env = append(s.Env, kv)
		}
	}
	base := workspace.PackDir(runDir, p.ID)
	vals := map[string][]string{"run": {runDir}, "cache": {filepath.Join(base, "cache")}, "tmp": {filepath.Join(base, "tmp")}}
	for k, argv := range p.CacheArgv {
		if out, err := pack.Expand(argv, vals); err == nil {
			s.Argv[k] = out
		}
	}
	return s
}

// PackTools lists the command words of p: every check's tool, the first
// argv word of every check, and the first word of every test command.
func PackTools(p pack.Pack) []string {
	var out []string
	add := func(w string) {
		if w != "" && !strings.ContainsAny(w, "{}") && !slicesHas(out, w) {
			out = append(out, w)
		}
	}
	for _, c := range p.Checks {
		add(c.Tool)
		if len(c.Argv) > 0 {
			add(c.Argv[0])
		}
		if len(c.FallbackArgv) > 0 {
			add(c.FallbackArgv[0])
		}
	}
	for _, tc := range p.TestCommands {
		if len(tc.ArgvPrefix) > 0 {
			add(tc.ArgvPrefix[0])
		}
	}
	for k := range p.CacheArgv {
		if f := strings.Fields(k); len(f) > 0 {
			add(f[0])
		}
	}
	return out
}

// Uses reports whether command runs one of the scope's tools as a command
// (directly, or as any stage of a pipeline or list).
func (s PackScope) Uses(command string) bool {
	tokens, err := tokenize(stripHeredocs(command))
	if err != nil {
		return false
	}
	for _, p := range pipelines(tokens) {
		for _, st := range p.stages {
			if w := commandWords(st); len(w) > 0 && s.tool(w[0]) {
				return true
			}
		}
	}
	return false
}

func (s PackScope) tool(w string) bool {
	for _, t := range s.Tools {
		if w == t || path.Base(w) == path.Base(t) {
			return true
		}
	}
	return false
}

// Scope rewrites command so the pack's tools run with the scope's env and
// cache flags: a VAR=value prefix and the cache_argv flags after the tool
// for a simple command, an `export …;` statement in front of anything else
// (cache flags are then left out). A command led by a reserved word or
// keyword (`!`, `time`, `{`, `if`…) is not simple: `VAR=x ! cmd` would run
// a command named "!". ok is false when the line does not use
// the pack's tools, already sets one of the env variables, goes through
// `loops`, or cannot be parsed: the caller leaves it alone.
func (s PackScope) Scope(command string) (string, bool) {
	command = strings.TrimRight(command, " \t\r\n")
	tokens, err := tokenize(stripHeredocs(command))
	if err != nil || len(tokens) == 0 || !s.Uses(command) {
		return "", false
	}
	keys := map[string]bool{}
	for _, kv := range s.Env {
		k, _, _ := strings.Cut(kv, "=")
		keys[k] = true
	}
	simple := !reserved[firstWord(tokens)]
	for i, t := range tokens {
		if t.op {
			simple = false
			continue
		}
		if k, _, ok := strings.Cut(t.s, "="); ok && envPrefix.MatchString(t.s) && keys[k] {
			return "", false // already scoped by hand
		}
		if path.Base(t.s) == "loops" && i+1 < len(tokens) && !tokens[i+1].op {
			return "", false // loops exec / eval "$(loops run env)"
		}
		if strings.Contains(t.s, "loops run env") || strings.Contains(t.s, "loops exec") {
			return "", false
		}
	}
	assign := make([]string, 0, len(s.Env))
	for _, kv := range s.Env {
		k, v, _ := strings.Cut(kv, "=")
		assign = append(assign, k+"="+workspace.ShellQuote(v))
	}
	if !simple {
		if len(assign) == 0 {
			return "", false
		}
		return "export " + strings.Join(assign, " ") + "; " + command, true
	}
	out := s.insertArgv(command)
	if len(assign) == 0 {
		if out == command {
			return "", false
		}
		return out, true
	}
	return strings.Join(assign, " ") + " " + out, true
}

// firstWord returns the command word of a token list: the first word after
// any VAR=value prefixes ("" when there is none).
func firstWord(tokens []token) string {
	for _, t := range tokens {
		if t.op {
			return ""
		}
		if !envPrefix.MatchString(t.s) {
			return t.s
		}
	}
	return ""
}

// insertArgv adds the cache flags after the tool words of a simple command:
// the flags of the longest cache_argv key that prefixes it ("swift build"
// before "swift"), unless the command already carries their first flag.
func (s PackScope) insertArgv(command string) string {
	words, ends := rawWords(command)
	i := 0
	for i < len(words) && envPrefix.MatchString(words[i]) {
		i++
	}
	if i == len(words) {
		return command
	}
	keys := make([]string, 0, len(s.Argv))
	for k := range s.Argv {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b string) int { // longest tool prefix first
		if n := len(strings.Fields(b)) - len(strings.Fields(a)); n != 0 {
			return n
		}
		return strings.Compare(a, b)
	})
	for _, key := range keys {
		flags := s.Argv[key]
		kw := strings.Fields(key)
		if len(flags) == 0 || len(kw) == 0 || len(words)-i < len(kw) {
			continue
		}
		if words[i] != kw[0] && path.Base(words[i]) != path.Base(kw[0]) {
			continue
		}
		match := true
		for j := 1; j < len(kw); j++ {
			match = match && words[i+j] == kw[j]
		}
		if !match {
			continue
		}
		if slicesHas(words, flags[0]) {
			return command // already scoped by hand; a shorter key must not apply
		}
		q := make([]string, len(flags))
		for j, f := range flags {
			q[j] = workspace.ShellQuote(f)
		}
		at := ends[i+len(kw)-1]
		return command[:at] + " " + strings.Join(q, " ") + command[at:]
	}
	return command
}

// rawWords splits a simple command into its unquoted words and the byte
// offset where each word ends in command (quotes as in tokenize).
func rawWords(command string) (words []string, ends []int) {
	var word []byte
	in := false
	var quote byte
	for i := 0; i < len(command); i++ {
		ch := command[i]
		switch {
		case quote != 0:
			if ch == quote {
				quote = 0
			} else {
				word = append(word, ch)
			}
		case ch == '\'' || ch == '"':
			quote, in = ch, true
		case ch == '\\' && i+1 < len(command):
			i++
			word, in = append(word, command[i]), true
		case ch == ' ' || ch == '\t':
			if in {
				words, ends = append(words, string(word)), append(ends, i)
				word, in = nil, false
			}
		default:
			word, in = append(word, ch), true
		}
	}
	if in {
		words, ends = append(words, string(word)), append(ends, len(command))
	}
	return words, ends
}
