package guard

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	passThrough = map[string]bool{"cat": true, "sed": true, "nl": true, "tr": true, "cut": true, "less": true, "more": true, "tee": true, "expand": true}
	sedRange    = regexp.MustCompile(`(\d+)\s*(?:,\s*(\+?\d+|\$))?\s*p`)
)

// checkBash sums the lines cat and sed -n views print from repo files.
func (g *Guard) checkBash(command, cwd, repo string, maxView int) string {
	tokens, err := tokenize(stripHeredocs(command))
	if err != nil {
		return ""
	}
	base := cwd
	lines, first := 0, ""
	for _, p := range pipelines(tokens) {
		words := commandWords(p.stages[0])
		if len(words) == 0 {
			continue
		}
		if words[0] == "cd" {
			var ok bool
			if base, ok = nextDir(words, base); !ok {
				break
			}
			continue
		}
		if (words[0] != "cat" && words[0] != "sed") || p.redirected || shrinksOutput(p.stages) {
			continue
		}
		printed, name, err := g.view(words, base, repo)
		if err != nil {
			return ""
		}
		lines += printed
		if first == "" {
			first = name
		}
	}
	if lines > maxView {
		return fmt.Sprintf("read_guard: this command prints about %d lines of repo files (%s). Use Read with offset and limit for the part you need, grep -n to find it, or the loops:code-researcher agent for a broad question. Several Reads can go in one message.", lines, first)
	}
	return ""
}

// resolve joins a path onto base like Python os.path.join + normpath.
func resolve(base, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(base, p))
}

func nextDir(words []string, base string) (string, bool) {
	if len(words) < 2 {
		return "", false
	}
	t := words[1]
	if strings.ContainsAny(t, "$`") || t == "-" || strings.HasPrefix(t, "~") {
		return "", false
	}
	return resolve(base, t), true
}

// repoFiles returns the existing repo files a path or glob names.
func (g *Guard) repoFiles(path, base, repo string) []string {
	if strings.ContainsAny(path, "$`") {
		return nil
	}
	full := resolve(base, path)
	matches := []string{full}
	if strings.ContainsAny(full, "*?[") {
		matches, _ = g.files.Glob(full)
	}
	root := filepath.Clean(repo) + string(filepath.Separator)
	var out []string
	for _, m := range matches {
		if strings.HasPrefix(m, root) && g.files.Exists(m) {
			out = append(out, m)
		}
	}
	return out
}

// view returns the lines printed and the first repo file of a cat or sed -n view.
func (g *Guard) view(words []string, base, repo string) (int, string, error) {
	var script string
	var paths []string
	isSed := words[0] == "sed"
	if !isSed {
		for _, a := range words[1:] {
			if !strings.HasPrefix(a, "-") {
				paths = append(paths, a)
			}
		}
	} else {
		var ok bool
		if script, paths, ok = parseSed(words[1:]); !ok {
			return 0, "", nil
		}
	}
	var files []string
	for _, p := range paths {
		files = append(files, g.repoFiles(p, base, repo)...)
	}
	if len(files) == 0 {
		return 0, "", nil
	}
	total := 0
	for _, f := range files {
		n, err := g.files.Lines(f)
		if err != nil {
			return 0, "", err
		}
		total += n
	}
	name, err := filepath.Rel(repo, files[0])
	if err != nil {
		return 0, "", err
	}
	if isSed {
		total = sedLines(script, total)
	}
	return total, name, nil
}

// parseSed returns the script and paths of a printing sed -n.
func parseSed(args []string) (string, []string, bool) {
	hasN := false
	for _, a := range args {
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") {
			f := a[1:]
			if strings.ContainsAny(f, "if") {
				return "", nil, false
			}
			if strings.Contains(f, "n") {
				hasN = true
			}
		}
	}
	if !hasN {
		return "", nil, false
	}
	var scripts, rest []string
	take := false
	for _, a := range args {
		switch {
		case take:
			scripts = append(scripts, a)
			take = false
		case strings.HasPrefix(a, "-"):
			take = strings.Contains(a[1:], "e")
		default:
			rest = append(rest, a)
		}
	}
	if len(scripts) == 0 && len(rest) > 0 {
		scripts, rest = []string{rest[0]}, rest[1:]
	}
	return strings.Join(scripts, " "), rest, true
}

// sedLines counts the lines numeric ranges print; pattern ranges count as 0.
func sedLines(script string, total int) int {
	lines := 0
	for _, m := range sedRange.FindAllStringSubmatch(script, -1) {
		first, _ := strconv.Atoi(m[1])
		switch end := m[2]; {
		case end == "":
			lines++
		case end == "$":
			lines += max(total-first+1, 0)
		case strings.HasPrefix(end, "+"):
			n, _ := strconv.Atoi(end[1:])
			lines += n + 1
		default:
			n, _ := strconv.Atoi(end)
			lines += max(n-first+1, 0)
		}
	}
	return min(lines, total)
}

// shrinksOutput reports whether a later pipeline stage filters the output.
func shrinksOutput(stages [][]string) bool {
	for _, st := range stages[1:] {
		words := commandWords(st)
		if len(words) == 0 || passThrough[words[0]] {
			continue
		}
		if words[0] == "grep" {
			for _, w := range words[1:] {
				if strings.HasPrefix(w, "-") && strings.Contains(w[1:], "v") {
					goto next
				}
			}
		}
		return true
	next:
	}
	return false
}
