package guard

import (
	"regexp"
	"strings"
)

// goTestWord finds `go test` (also go1.22 test, /abs/go, go -C dir test) in
// shell text; group 1 ends right after the word "test".
var goTestWord = regexp.MustCompile("(?:^|[\\s;&|(){}'\"`])([\\w./~$-]*/)?(go(?:1\\.\\d+(?:\\.\\d+)?)?(?:\\s+-C\\s+\\S+)?\\s+test)(?:\\s|$|['\";)&|`])")

// segmentEnd finds where the arguments of a command end in raw shell text.
var segmentEnd = regexp.MustCompile("&&|\\|\\||[;|\n)`]")

var raceWord = regexp.MustCompile(`(?:^|\s)-race(?:\s|=|$)`)

// AddRace inserts -race after every `go test` in command that lacks it, so
// a command CheckRace denies can be rewritten instead (toolchain-env does
// so where the platform rewrites Go commands). ok is true only when the
// result passes CheckRace; a command it cannot fix (a quoted or computed
// go invocation, -race=false, a gotestsum wrapper) is returned unchanged
// with ok false and stays denied.
func AddRace(command string) (fixed string, ok bool) {
	if CheckRace(command) == "" {
		return command, false // nothing to add
	}
	var b strings.Builder
	last := 0
	for pos := 0; pos < len(command); {
		m := goTestWord.FindStringSubmatchIndex(command[pos:])
		if m == nil {
			break
		}
		end := pos + m[5] // end of group 2: just after "test"
		pos = end
		rest := command[end:]
		if loc := segmentEnd.FindStringIndex(rest); loc != nil {
			rest = rest[:loc[0]]
		}
		if raceWord.MatchString(" " + rest) {
			continue
		}
		b.WriteString(command[last:end])
		b.WriteString(" -race")
		last = end
	}
	b.WriteString(command[last:])
	fixed = b.String()
	if fixed == command || CheckRace(fixed) != "" {
		return command, false
	}
	return fixed, true
}
