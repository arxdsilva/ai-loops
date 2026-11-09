package port

import (
	"fmt"
	"strconv"
	"strings"
)

// frontmatter is a parsed `---` block: flat `key: value` pairs.
type frontmatter map[string]string

// parseFrontmatter splits a markdown document into its frontmatter and
// body. The block must open on the first line with `---` and close with a
// `---` line. Each non-blank line is `key: value`, where value is plain
// text, a double-quoted string (JSON/Go escapes), or a single-quoted
// string (” escapes a quote). Keys are [A-Za-z0-9_-]+ and unique. The
// body is everything after the closing line, leading blank lines removed.
func parseFrontmatter(name string, data []byte) (frontmatter, string, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return nil, "", fmt.Errorf("%s: missing frontmatter (first line must be ---)", name)
	}
	rest := text[len("---\n"):]
	fm := frontmatter{}
	for lineNo := 2; ; lineNo++ {
		if rest == "" {
			return nil, "", fmt.Errorf("%s: unterminated frontmatter (no closing ---)", name)
		}
		line, tail, _ := strings.Cut(rest, "\n")
		rest = tail
		if line == "---" {
			break
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, raw, ok := strings.Cut(line, ":")
		if !ok || !validKey(key) {
			return nil, "", fmt.Errorf("%s:%d: malformed frontmatter line %q (want key: value)", name, lineNo, line)
		}
		if _, dup := fm[key]; dup {
			return nil, "", fmt.Errorf("%s:%d: duplicate frontmatter key %q", name, lineNo, key)
		}
		val, err := scalar(strings.TrimSpace(raw))
		if err != nil {
			return nil, "", fmt.Errorf("%s:%d: %s: %w", name, lineNo, key, err)
		}
		fm[key] = val
	}
	return fm, strings.TrimLeft(rest, "\n"), nil
}

func validKey(k string) bool {
	if k == "" {
		return false
	}
	for _, r := range k {
		if !(r == '-' || r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

// scalar decodes one frontmatter value.
func scalar(v string) (string, error) {
	switch {
	case strings.HasPrefix(v, `"`):
		if len(v) < 2 || !strings.HasSuffix(v, `"`) {
			return "", fmt.Errorf("unterminated double-quoted string %s", v)
		}
		s, err := strconv.Unquote(v)
		if err != nil {
			return "", fmt.Errorf("bad double-quoted string %s: %w", v, err)
		}
		return s, nil
	case strings.HasPrefix(v, "'"):
		if len(v) < 2 || !strings.HasSuffix(v, "'") {
			return "", fmt.Errorf("unterminated single-quoted string %s", v)
		}
		inner := v[1 : len(v)-1]
		if strings.Contains(strings.ReplaceAll(inner, "''", ""), "'") {
			return "", fmt.Errorf("bad single-quoted string %s", v)
		}
		return strings.ReplaceAll(inner, "''", "'"), nil
	case strings.HasPrefix(v, "|") || strings.HasPrefix(v, ">"):
		return "", fmt.Errorf("block scalars are not supported: %s", v)
	}
	return v, nil
}

// list splits a comma-separated value (`Edit, NotebookEdit`, optionally
// in [brackets]) into trimmed, non-empty items.
func list(v string) ([]string, error) {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, "[") {
		if !strings.HasSuffix(v, "]") {
			return nil, fmt.Errorf("unterminated list %s", v)
		}
		v = v[1 : len(v)-1]
	}
	if strings.TrimSpace(v) == "" {
		return nil, nil
	}
	var out []string
	for _, item := range strings.Split(v, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			return nil, fmt.Errorf("empty item in list %q", v)
		}
		out = append(out, item)
	}
	return out, nil
}
