package pack

import (
	"fmt"
	"path"
	"slices"
	"strings"
)

// Expand fills the {name} placeholders of argv from vals.
//   - An element that is exactly "{name}" expands to every value of name, in
//     place (a list placeholder such as {pkgs} or {coverage_args} may expand
//     to nothing).
//   - A placeholder inside a longer element ("-coverprofile={coverage}")
//     needs exactly one value.
//
// An unknown placeholder is an error, so a pack typo never reaches a shell.
func Expand(argv []string, vals map[string][]string) ([]string, error) {
	out := make([]string, 0, len(argv))
	for _, a := range argv {
		if m := placeholderRe.FindStringSubmatch(a); m != nil && m[0] == a {
			v, ok := vals[m[1]]
			if !ok {
				return nil, fmt.Errorf("unknown placeholder %s", a)
			}
			out = append(out, v...)
			continue
		}
		var err error
		s := placeholderRe.ReplaceAllStringFunc(a, func(ph string) string {
			name := ph[1 : len(ph)-1]
			v, ok := vals[name]
			switch {
			case !ok:
				err = fmt.Errorf("unknown placeholder %s in %q", ph, a)
			case len(v) != 1:
				err = fmt.Errorf("placeholder %s in %q needs one value, has %d", ph, a, len(v))
			default:
				return v[0]
			}
			return ph
		})
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// Placeholders returns the distinct placeholder names used in argv, in order.
func Placeholders(argv []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, a := range argv {
		for _, m := range placeholderRe.FindAllStringSubmatch(a, -1) {
			if !seen[m[1]] {
				seen[m[1]] = true
				out = append(out, m[1])
			}
		}
	}
	return out
}

// SourceExts lists the file extensions of every pack's sources (lower case,
// with the dot). The stop gate uses it to notice source edits without
// loading packs; a test keeps it in step with the embedded packs' globs.
var SourceExts = []string{
	".go",
	".py", ".pyi",
	".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs",
	".java", ".kt", ".kts", ".scala", ".sc", ".clj", ".cljs", ".cljc",
	".cs", ".csproj", ".php", ".rb", ".rake", ".rs", ".swift",
	".ex", ".exs", ".erl", ".hrl",
	".c", ".cc", ".cpp", ".cxx", ".h", ".hh", ".hpp", ".hxx", ".ipp", ".ixx", ".cppm",
}

// LooksLikeSource reports whether the file name has a source extension of
// some pack.
func LooksLikeSource(name string) bool {
	return slices.Contains(SourceExts, strings.ToLower(path.Ext(name)))
}
