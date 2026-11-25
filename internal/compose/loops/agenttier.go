package loops

import (
	"io/fs"
	"path"
	"strings"
)

// agentTiers returns the model-guard's role lookup: a role's default tier is
// the `model:` frontmatter line of agents/<role>.md in the plugin source.
// Unknown roles and unreadable files give "" (the guard then does not check).
func agentTiers(src fs.FS) func(role string) string {
	return func(role string) string {
		if role == "" || strings.ContainsAny(role, `/\`) || strings.HasPrefix(role, ".") {
			return ""
		}
		data, err := fs.ReadFile(src, path.Join("agents", role+".md"))
		if err != nil {
			return ""
		}
		return frontmatterModel(string(data))
	}
}

// frontmatterModel reads `model:` from a leading `---` block.
func frontmatterModel(doc string) string {
	doc = strings.ReplaceAll(doc, "\r\n", "\n")
	rest, ok := strings.CutPrefix(doc, "---\n")
	if !ok {
		return ""
	}
	for rest != "" {
		line, tail, _ := strings.Cut(rest, "\n")
		rest = tail
		if line == "---" {
			return ""
		}
		if v, ok := strings.CutPrefix(line, "model:"); ok {
			return strings.ToLower(strings.Trim(strings.TrimSpace(v), `"'`))
		}
	}
	return ""
}
