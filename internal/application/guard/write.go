package guard

import (
	"fmt"
	"path/filepath"
	"strings"
)

var readOnlyRoles = map[string]bool{
	"code-researcher":      true,
	"hardware-researcher":  true,
	"plan-reviewer":        true,
	"planner":              true,
	"practices-researcher": true,
	"product-reviewer":     true,
	"qa-verifier":          true,
	"reflector":            true,
	"security-reviewer":    true,
	"web-researcher":       true,
}

// CheckWrite returns a deny reason ("" = allow) when a read-only role
// attempts to write outside the active run directory (or outside
// <projectDir>/.loops/memory for reflector).
func CheckWrite(role, filePath, runDir, projectDir string) string {
	role = strings.TrimPrefix(strings.TrimSpace(role), "loops:")
	if role == "" || !readOnlyRoles[role] || filePath == "" || runDir == "" {
		return ""
	}
	if withinDir(runDir, filePath) {
		return ""
	}
	if role == "reflector" && projectDir != "" && withinDir(filepath.Join(projectDir, ".loops", "memory"), filePath) {
		return ""
	}
	return fmt.Sprintf("write_guard: role %s is read-only and may only write inside the run directory", role)
}

func withinDir(base, target string) bool {
	cleanBase := filepath.Clean(base)
	cleanTarget := filepath.Clean(target)
	rel, err := filepath.Rel(cleanBase, cleanTarget)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
