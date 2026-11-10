package guard

import (
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/pack"
)

const raceMsg = "race_guard: run go test with -race (unit tests must always use the race detector — WORKFLOW §6). Example: go test -race -count=1 ./…"

var (
	goBin      = regexp.MustCompile(`^go(1\.\d+(\.\d+)?)?$`)
	wrapperBin = map[string]bool{"gotestsum": true, "richgo": true}
	shells     = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true}
	// transparent prefixes that run their argument as a command.
	// value: options that consume the next word, and positional words to skip.
	transparent = map[string]wrapperSpec{
		"command": {}, "exec": {}, "nohup": {}, "time": {}, "builtin": {}, "setsid": {}, "unbuffer": {},
		"nice": {opts: "-n"}, "sudo": {opts: "-u -g"}, "ionice": {opts: "-c -n -p"}, "caffeinate": {opts: "-t -w"},
		"stdbuf": {opts: "-i -o -e"}, "timeout": {opts: "-s -k", pos: 1}, "chrt": {pos: 1},
	}
	// xargs options that consume the next word.
	xargsArg = map[string]bool{"-n": true, "-I": true, "-P": true, "-L": true, "-d": true, "-s": true, "-E": true, "-a": true, "-J": true}
)

type wrapperSpec struct {
	opts string
	pos  int
}

const maxRaceDepth = 3

// CheckRace returns a deny reason ("" = allow) when a shell line runs
// `go test` (or a gotestsum/richgo wrapper) without the race detector.
func CheckRace(command string) string {
	return CheckTests(command, nil)
}

// CheckTests returns a deny reason ("" = allow) when a shell line runs a
// test command without its race policy (TOOLING §4.2). Go is always checked
// (CheckRace). Every other pack in packs with race mode "flag" is checked
// from its data: a command matching its test_commands must carry one of its
// race.flags, or have its race.env set on the line. Modes "build" and
// "none" never deny: their policy is enforced by the gate.
func CheckTests(command string, packs []pack.Pack) string {
	s := raceScan{packs: flagPacks(packs)}
	return s.line(command, raceEnv{}, 0)
}

// flagPacks keeps the non-Go packs whose race policy the guard enforces.
func flagPacks(packs []pack.Pack) []pack.Pack {
	var out []pack.Pack
	for _, p := range packs {
		if p.ID == "go" || p.Race.Mode != pack.RaceFlag || len(p.TestCommands) == 0 || len(p.Race.Flags)+len(p.Race.Env) == 0 {
			continue
		}
		out = append(out, p)
	}
	return out
}

// raceEnv is the environment a command inherits on the line: the Go race
// setting from GOFLAGS, and every VAR=value assigned so far.
type raceEnv struct {
	goRace bool
	vars   map[string]string
}

func (e raceEnv) assign(word string) raceEnv {
	k, v, _ := strings.Cut(word, "=")
	if k == "GOFLAGS" {
		if on, found := goFlagsRace(v); found {
			e.goRace = on
		}
	}
	e.vars = maps.Clone(e.vars)
	if e.vars == nil {
		e.vars = map[string]string{}
	}
	e.vars[k] = v
	return e
}

func (e raceEnv) unset(k string) raceEnv {
	if k == "GOFLAGS" {
		e.goRace = false
	}
	if _, ok := e.vars[k]; ok {
		e.vars = maps.Clone(e.vars)
		delete(e.vars, k)
	}
	return e
}

// has reports whether every want entry is set: the variable's words
// include every word of the wanted value.
func (e raceEnv) has(want map[string]string) bool {
	if len(want) == 0 {
		return false
	}
	for k, v := range want {
		got, ok := e.vars[k]
		if !ok {
			return false
		}
		have := strings.Fields(got)
		for _, w := range strings.Fields(v) {
			if !slicesHas(have, w) {
				return false
			}
		}
	}
	return true
}

// raceScan walks shell lines for CheckTests.
type raceScan struct {
	packs []pack.Pack
}

// flagVal reports a race flag's last setting in args ("-race", "-race=…").
// Parsing stops at -args or --.
func raceFlag(args []string) (on, found bool) {
	for _, a := range args {
		if a == "-args" || a == "--args" || a == "--" {
			break
		}
		a = strings.TrimPrefix(a, "-")
		if !strings.HasPrefix(a, "race") {
			continue
		}
		switch a {
		case "race":
			on, found = true, true
		case "race=true", "race=1", "race=t", "race=T", "race=TRUE", "race=True":
			on, found = true, true
		case "race=false", "race=0", "race=f", "race=F", "race=FALSE", "race=False":
			on, found = false, true
		}
	}
	return
}

// goFlagsRace reports the race setting inside a GOFLAGS value.
func goFlagsRace(v string) (on, found bool) {
	return raceFlag(strings.Fields(v))
}

func isWrapper(w string) bool {
	_, ok := transparent[path.Base(w)]
	return ok
}

func slicesHas(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// embedded returns shell text hidden in backticks, $(...) and heredocs that
// feed a shell, so the caller can check it as its own line.
func embedded(command string) []string {
	var out []string
	for i := 0; i < len(command); i++ {
		switch {
		case command[i] == '`':
			if j := strings.IndexByte(command[i+1:], '`'); j >= 0 {
				out = append(out, command[i+1:i+1+j])
				i += j + 1
			}
		case strings.HasPrefix(command[i:], "$("):
			depth := 0
			for j := i + 1; j < len(command); j++ {
				if command[j] == '(' {
					depth++
				} else if command[j] == ')' {
					if depth--; depth == 0 {
						out = append(out, command[i+2:j])
						i = j
						break
					}
				}
			}
		}
	}
	pos := 0
	for pos < len(command) {
		loc := heredocHead.FindStringSubmatchIndex(command[pos:])
		if loc == nil {
			break
		}
		start, headEnd := pos+loc[0], pos+loc[1]
		delim := command[pos+loc[4] : pos+loc[5]]
		if command[pos+loc[6]:pos+loc[7]] != "" && command[pos+loc[2]:pos+loc[3]] == "" {
			headEnd--
		}
		end := heredocEnd(command, headEnd, delim)
		if end < 0 {
			pos = start + 2
			continue
		}
		lineStart := strings.LastIndexByte(command[:start], '\n') + 1
		if toks, err := tokenize(command[lineStart:start]); err == nil {
			for _, t := range toks {
				if !t.op && shells[path.Base(t.s)] {
					nl := strings.IndexByte(command[headEnd:], '\n')
					out = append(out, command[headEnd+nl+1:end])
					break
				}
			}
		}
		pos = end
	}
	return out
}

func (s *raceScan) line(command string, inherited raceEnv, depth int) string {
	if depth < maxRaceDepth {
		for _, sub := range embedded(command) {
			if r := s.line(sub, inherited, depth+1); r != "" {
				return r
			}
		}
	}
	tokens, err := tokenize(stripHeredocs(command))
	if err != nil {
		return ""
	}
	exported := inherited
	for _, p := range pipelines(tokens) {
		for _, stage := range p.stages {
			if r := s.stage(stage, &exported, depth); r != "" {
				return r
			}
		}
	}
	return ""
}

func (s *raceScan) stage(stage []string, exported *raceEnv, depth int) string {
	local := *exported
	i := 0
	assigned := false
	skipAssign := func() {
		for i < len(stage) && envPrefix.MatchString(stage[i]) {
			local = local.assign(stage[i])
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
			for i < len(stage) && strings.HasPrefix(stage[i], "-") {
				i++
			}
			skipAssign()
			*exported = local
			return ""
		case w == "unset":
			for _, k := range stage[i+1:] {
				if !strings.HasPrefix(k, "-") {
					local = local.unset(k)
				}
			}
			*exported = local
			return ""
		case path.Base(w) == "env":
			i++
		envOpts:
			for i < len(stage) && (strings.HasPrefix(stage[i], "-") || envPrefix.MatchString(stage[i])) {
				a := stage[i]
				switch {
				case envPrefix.MatchString(a):
					skipAssign()
					continue
				case a == "-i" || a == "-" || a == "--ignore-environment":
					local = raceEnv{} // the command sees only what env sets next
				case a == "-u" || a == "--unset":
					if i+1 < len(stage) {
						local = local.unset(stage[i+1])
					}
					i++
				case strings.HasPrefix(a, "--unset="):
					local = local.unset(strings.TrimPrefix(a, "--unset="))
				case strings.HasPrefix(a, "-u"):
					local = local.unset(a[2:])
				case a == "-C" || a == "-S" || a == "--chdir" || a == "--split-string":
					i++
				case a == "--":
					i++
					break envOpts
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
				return s.line(strings.Join(stage[i:], " "), local, depth+1)
			}
			return ""
		case path.Base(w) == "eval":
			if depth < maxRaceDepth {
				return s.line(strings.Join(stage[i+1:], " "), local, depth+1)
			}
			return ""
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
			return s.command(stage[i:], local, depth)
		}
	}
	if assigned { // bare assignments persist for the rest of the line
		*exported = local
	}
	return ""
}

func (s *raceScan) command(words []string, env raceEnv, depth int) string {
	bin := path.Base(words[0])
	args := words[1:]
	envRace := env.goRace
	switch {
	case goBin.MatchString(bin):
		for len(args) > 0 && strings.HasPrefix(args[0], "-") { // go -C dir test
			if args[0] == "-C" && len(args) > 1 {
				args = args[1:]
			}
			args = args[1:]
		}
		if len(args) == 0 || args[0] != "test" {
			return ""
		}
		return needRace(args[1:], envRace, true)
	case bin == "richgo":
		if len(args) == 0 || args[0] != "test" {
			return ""
		}
		return needRace(args[1:], envRace, true)
	case wrapperBin[bin]:
		for _, a := range args {
			if a == "--help" || a == "-h" || a == "--version" || a == "-version" {
				return ""
			}
		}
		for k, a := range args {
			if a == "--" {
				return needRace(args[k+1:], envRace, false)
			}
		}
		return needRace(nil, envRace, false)
	case shells[bin] && depth < maxRaceDepth:
		for k, a := range args {
			if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.Contains(a, "c") && k+1 < len(args) {
				return s.line(args[k+1], env, depth+1)
			}
		}
	default:
		return s.packCommand(words, env)
	}
	return ""
}

// packCommand checks words against the flag-mode packs' test commands.
func (s *raceScan) packCommand(words []string, env raceEnv) string {
	for _, p := range s.packs {
		n := testPrefix(p, words)
		if n == 0 {
			continue
		}
		if hasFlag(words[n:], p.Race.Flags) || env.has(p.Race.Env) {
			return ""
		}
		return packRaceMsg(p, words[:n])
	}
	return ""
}

// testPrefix returns the length of the pack test-command prefix words starts
// with (0 = none). The first word matches by base name too (/usr/bin/swift).
func testPrefix(p pack.Pack, words []string) int {
	for _, tc := range p.TestCommands {
		pre := tc.ArgvPrefix
		if len(pre) == 0 || len(words) < len(pre) {
			continue
		}
		if words[0] != pre[0] && path.Base(words[0]) != path.Base(pre[0]) {
			continue
		}
		if slices.Equal(words[1:len(pre)], pre[1:]) {
			return len(pre)
		}
	}
	return 0
}

// hasFlag reports whether args carry one of flags; "--opt=value" also
// matches the two words "--opt value".
func hasFlag(args, flags []string) bool {
	for i, a := range args {
		for _, f := range flags {
			if a == f {
				return true
			}
			if k, v, ok := strings.Cut(f, "="); ok && a == k && i+1 < len(args) && args[i+1] == v {
				return true
			}
		}
	}
	return false
}

func packRaceMsg(p pack.Pack, prefix []string) string {
	name := p.Name
	if name == "" {
		name = p.ID
	}
	msg := "test_guard: run `" + strings.Join(prefix, " ") + "` with " + strings.Join(p.Race.Flags, " or ")
	if len(p.Race.Env) > 0 {
		var kv []string
		for _, k := range sortedKeys(p.Race.Env) {
			kv = append(kv, k+"="+p.Race.Env[k])
		}
		if len(p.Race.Flags) == 0 {
			msg = "test_guard: run `" + strings.Join(prefix, " ") + "` with " + strings.Join(kv, " ")
		} else {
			msg += " (or set " + strings.Join(kv, " ") + ")"
		}
	}
	return msg + ": the " + name + " pack's race policy (WORKFLOW §6)."
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func needRace(args []string, envRace, list bool) string {
	if list {
		for _, a := range args {
			if a == "-args" {
				break
			}
			if a == "-list" || a == "--list" || strings.HasPrefix(a, "-list=") || strings.HasPrefix(a, "--list=") {
				return ""
			}
		}
	}
	if on, found := raceFlag(args); found {
		if on {
			return ""
		}
		return raceMsg
	}
	if envRace {
		return ""
	}
	return raceMsg
}
