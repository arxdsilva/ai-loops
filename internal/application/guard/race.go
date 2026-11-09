package guard

import (
	"path"
	"regexp"
	"strings"
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
	return checkRaceLine(command, false, 0)
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

func checkRaceLine(command string, inheritedRace bool, depth int) string {
	if depth < maxRaceDepth {
		for _, sub := range embedded(command) {
			if r := checkRaceLine(sub, inheritedRace, depth+1); r != "" {
				return r
			}
		}
	}
	tokens, err := tokenize(stripHeredocs(command))
	if err != nil {
		return ""
	}
	exported := inheritedRace
	for _, p := range pipelines(tokens) {
		for _, stage := range p.stages {
			if r := checkRaceStage(stage, &exported, depth); r != "" {
				return r
			}
		}
	}
	return ""
}

func checkRaceStage(stage []string, exported *bool, depth int) string {
	local := *exported
	i := 0
	assigned := false
	skipAssign := func() {
		for i < len(stage) && envPrefix.MatchString(stage[i]) {
			if v, ok := strings.CutPrefix(stage[i], "GOFLAGS="); ok {
				if on, found := goFlagsRace(v); found {
					local = on
				}
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
			for i < len(stage) && strings.HasPrefix(stage[i], "-") {
				i++
			}
			skipAssign()
			*exported = local
			return ""
		case w == "env":
			i++
			for i < len(stage) && (strings.HasPrefix(stage[i], "-") || envPrefix.MatchString(stage[i])) {
				if envPrefix.MatchString(stage[i]) {
					skipAssign()
					continue
				}
				if stage[i] == "-u" || stage[i] == "-C" || stage[i] == "-S" {
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
				return checkRaceLine(strings.Join(stage[i:], " "), local, depth+1)
			}
			return ""
		case path.Base(w) == "eval":
			if depth < maxRaceDepth {
				return checkRaceLine(strings.Join(stage[i+1:], " "), local, depth+1)
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
			return checkRaceCommand(stage[i:], local, depth)
		}
	}
	if assigned { // bare assignments persist for the rest of the line
		*exported = local
	}
	return ""
}

func checkRaceCommand(words []string, envRace bool, depth int) string {
	bin := path.Base(words[0])
	args := words[1:]
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
				return checkRaceLine(args[k+1], envRace, depth+1)
			}
		}
	}
	return ""
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
