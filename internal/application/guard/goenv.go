package guard

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/workspace"
)

// GoUse is how a shell line uses the go tool, relative to a run's GOCACHE.
type GoUse struct {
	// Invokes: some command builds, tests, vets, runs, installs, generates,
	// lists or runs a tool with go (directly or via env, xargs, sh -c,
	// gotestsum/richgo).
	Invokes bool
	// Unscoped: at least one of those commands runs without GOCACHE set to
	// the run's cache.
	Unscoped bool
	// Simple: the line is one simple command (no operators, no leading
	// keyword), so VAR=value prefixes reach every command in it.
	Simple bool
	// Gate: the line is exactly one `go build|test|vet|list` command
	// (assignment prefixes allowed) and nothing else.
	Gate bool
}

// goCacheCmds are the go subcommands that use the build cache.
var goCacheCmds = map[string]bool{"build": true, "test": true, "vet": true, "run": true, "install": true, "generate": true, "tool": true, "list": true}

// gateCmds are the subcommands the gate itself runs.
var gateCmds = map[string]bool{"build": true, "test": true, "vet": true, "list": true}

// reserved words cannot follow a VAR=value prefix.
var reserved = map[string]bool{
	"if": true, "then": true, "else": true, "elif": true, "fi": true, "do": true, "done": true, "case": true, "esac": true,
	"while": true, "until": true, "for": true, "select": true, "function": true, "{": true, "}": true, "!": true,
	"[[": true, "]]": true, "time": true, "coproc": true,
}

// InspectGo reports how command uses the go tool against the run cache
// (the tokenizer and command walk mirror CheckRace). An unparseable line
// reports the zero GoUse.
func InspectGo(command, cache string) GoUse {
	command = strings.TrimRight(command, " \t\r\n")
	g := goScan{cache: filepath.Clean(cache)}
	tokens, err := tokenize(stripHeredocs(command))
	if err != nil || len(tokens) == 0 {
		return GoUse{}
	}
	g.line(command, false, 0)
	g.use.Simple, g.use.Gate = simple(tokens)
	g.use.Gate = g.use.Gate && g.use.Invokes
	return g.use
}

// simple reports whether tokens are one simple command, and whether that
// command is a gate go command.
func simple(tokens []token) (isSimple, gate bool) {
	words := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if t.op {
			return false, false
		}
		words = append(words, t.s)
	}
	i := 0
	for i < len(words) && envPrefix.MatchString(words[i]) {
		i++
	}
	if i == len(words) || reserved[words[i]] {
		return false, false
	}
	if !goBin.MatchString(path.Base(words[i])) {
		return true, false
	}
	args := skipGoFlags(words[i+1:])
	return true, len(args) > 0 && gateCmds[args[0]]
}

// skipGoFlags drops the go tool's own leading flags (go -C dir test).
func skipGoFlags(args []string) []string {
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		if args[0] == "-C" && len(args) > 1 {
			args = args[1:]
		}
		args = args[1:]
	}
	return args
}

// ScopeGo returns command with GOCACHE and GOTMPDIR set to the run's
// directories for every command in it: a VAR=value prefix for a simple
// command, an `export …;` statement in front of anything else.
func ScopeGo(command, cache, tmp string) string {
	assign := "GOCACHE=" + workspace.ShellQuote(cache) + " GOTMPDIR=" + workspace.ShellQuote(tmp)
	if InspectGo(command, cache).Simple {
		return assign + " " + command
	}
	return "export " + assign + "; " + command
}

type goScan struct {
	cache string
	use   GoUse
}

func (g *goScan) scoped(v string) bool {
	return v != "" && filepath.Clean(v) == g.cache
}

func (g *goScan) line(command string, scoped bool, depth int) {
	if depth < maxRaceDepth {
		for _, sub := range embedded(command) {
			g.line(sub, scoped, depth+1)
		}
	}
	tokens, err := tokenize(stripHeredocs(command))
	if err != nil {
		return
	}
	exported := scoped
	for _, p := range pipelines(tokens) {
		for _, stage := range p.stages {
			g.stage(stage, &exported, depth)
		}
	}
}

func (g *goScan) stage(stage []string, exported *bool, depth int) {
	local := *exported
	i := 0
	assigned := false
	skipAssign := func() {
		for i < len(stage) && envPrefix.MatchString(stage[i]) {
			if v, ok := strings.CutPrefix(stage[i], "GOCACHE="); ok {
				local = g.scoped(v)
			}
			assigned = true
			i++
		}
	}
	for i < len(stage) {
		w := stage[i]
		switch {
		case leaders[w]:
			i++
		case envPrefix.MatchString(w):
			skipAssign()
		case w == "export" || w == "declare" || w == "typeset" || w == "readonly":
			i++
			unexport := false
			for i < len(stage) && strings.HasPrefix(stage[i], "-") {
				unexport = unexport || stage[i] == "-n" // export -n: not in the environment
				i++
			}
			skipAssign()
			if !unexport {
				*exported = local
			}
			return
		case path.Base(w) == "eval":
			rest := strings.Join(stage[i+1:], " ")
			if strings.Contains(rest, "loops") && strings.Contains(rest, "run env") {
				*exported = true // eval "$(loops run env)"
			} else if depth < maxRaceDepth {
				g.line(rest, local, depth+1)
			}
			return
		case w == "env":
			i++
			for i < len(stage) && (strings.HasPrefix(stage[i], "-") || envPrefix.MatchString(stage[i])) {
				if envPrefix.MatchString(stage[i]) {
					skipAssign()
					continue
				}
				if stage[i] == "-u" || stage[i] == "-C" || stage[i] == "-S" {
					if stage[i] == "-u" && i+1 < len(stage) && stage[i+1] == "GOCACHE" {
						local = false
					}
					i++
				}
				i++
			}
		case path.Base(w) == "watch":
			i++
			for i < len(stage) && strings.HasPrefix(stage[i], "-") {
				if stage[i] == "-n" || stage[i] == "--interval" {
					i++
				}
				i++
			}
			if i < len(stage) && depth < maxRaceDepth {
				g.line(strings.Join(stage[i:], " "), local, depth+1)
			}
			return
		case isWrapper(w):
			spec := transparent[path.Base(w)]
			i++
			for i < len(stage) && strings.HasPrefix(stage[i], "-") {
				if slicesHas(strings.Fields(spec.opts), stage[i]) {
					i++
				}
				i++
			}
			i += spec.pos
		case path.Base(w) == "xargs":
			i++
			for i < len(stage) && strings.HasPrefix(stage[i], "-") {
				if xargsArg[stage[i]] {
					i++
				}
				i++
			}
		default:
			g.command(stage[i:], local, depth)
			return
		}
	}
	if assigned { // bare assignments persist for the rest of the line
		*exported = local
	}
}

func (g *goScan) command(words []string, scoped bool, depth int) {
	bin := path.Base(words[0])
	args := words[1:]
	hit := false
	switch {
	case goBin.MatchString(bin):
		args = skipGoFlags(args)
		hit = len(args) > 0 && goCacheCmds[args[0]]
	case bin == "richgo":
		hit = len(args) > 0 && args[0] == "test"
	case wrapperBin[bin]:
		hit = true
	case shells[bin] && depth < maxRaceDepth:
		for k, a := range args {
			if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.Contains(a, "c") && k+1 < len(args) {
				g.line(args[k+1], scoped, depth+1)
				return
			}
		}
	}
	if hit {
		g.use.Invokes = true
		g.use.Unscoped = g.use.Unscoped || !scoped
	}
}
