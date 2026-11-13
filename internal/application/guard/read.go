package guard

import (
	"fmt"
	"path/filepath"
	"strings"
)

var binaryExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	".pdf": true, ".ipynb": true, ".wav": true, ".ogg": true,
}

func (g *guard) checkReadTool(in map[string]any, repo string, minLines int) string {
	if truthy(in["offset"]) || truthy(in["limit"]) {
		return ""
	}
	path, _ := in["file_path"].(string)
	if !strings.HasPrefix(path, filepath.Clean(repo)+string(filepath.Separator)) {
		return ""
	}
	if binaryExt[strings.ToLower(filepath.Ext(path))] || !g.files.Exists(path) {
		return ""
	}
	lines, err := g.files.Lines(path)
	if err != nil || lines <= minLines {
		return ""
	}
	rel, err := filepath.Rel(repo, path)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("read_guard: %s has %d lines. Use Read with offset and limit, after grep -n finds the range, or the loops:code-researcher agent for a broad question.", rel, lines)
}
