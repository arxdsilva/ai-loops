package workspace

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/pack"
)

// GoPack is the id of the Go pack, whose run env (GOCACHE/GOTMPDIR) loops
// has always provided (`loops go`, `loops run env`, the gate).
const GoPack = "go"

var (
	envKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	packIDRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

// ValidPackID reports whether id can name a pack's scratch directory under
// the run dir (the pack schema's id syntax: one lower-case path element).
func ValidPackID(id string) bool { return packIDRe.MatchString(id) }

// PackCache is a pack's run cache dir (<run>/<id>/cache).
func PackCache(runDir, id string) string { return filepath.Join(PackDir(runDir, id), "cache") }

// PackTmp is a pack's run tmp dir (<run>/<id>/tmp).
func PackTmp(runDir, id string) string { return filepath.Join(PackDir(runDir, id), "tmp") }

// expander resolves pack env and argv templates: {run}, {cache}, {tmp}.
func expander(runDir, id string) *strings.Replacer {
	return strings.NewReplacer("{run}", runDir, "{cache}", PackCache(runDir, id), "{tmp}", PackTmp(runDir, id))
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// RunEnv is the environment that scopes pack p's tools to the run: its
// cache_env (PackEnv) followed by its env, templates expanded, each part
// sorted by key. A key cache_env sets wins over env; keys that are not
// shell-safe names are dropped.
func RunEnv(runDir string, p pack.Pack) []string {
	var out []string
	set := map[string]bool{}
	for _, kv := range PackEnv(runDir, p.ID, p.CacheEnv) {
		k, _, _ := strings.Cut(kv, "=")
		if envKeyRe.MatchString(k) {
			set[k] = true
			out = append(out, kv)
		}
	}
	r := expander(runDir, p.ID)
	for _, k := range sortedKeys(p.Env) {
		if !set[k] && envKeyRe.MatchString(k) {
			out = append(out, k+"="+r.Replace(p.Env[k]))
		}
	}
	return out
}

// ExtraEnv is RunEnv without the keys in skip (for the Go pack: the
// GOCACHE/GOTMPDIR that Env already provides).
func ExtraEnv(env []string, skip ...string) []string {
	var out []string
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		drop := false
		for _, s := range skip {
			drop = drop || k == s
		}
		if !drop {
			out = append(out, kv)
		}
	}
	return out
}

// toolName is argv0 as a bare, lower-case tool name ("./gradlew" -> "gradlew",
// `C:\go\bin\go.exe` -> "go").
func toolName(argv0 string) string {
	n := strings.ToLower(path.Base(strings.ReplaceAll(argv0, `\`, "/")))
	for _, ext := range []string{".exe", ".cmd", ".bat"} {
		n = strings.TrimSuffix(n, ext)
	}
	return n
}

// cacheArgvKey returns p's cache_argv key for argv: the longest key whose
// words prefix argv (sorted first on a tie), and how many words it has. The
// first word matches by tool name ("./gradlew" matches "gradlew.bat"), the
// others exactly ("swift build" matches swift build …, not swift test …).
func cacheArgvKey(p pack.Pack, argv []string) (string, int) {
	keys := make([]string, 0, len(p.CacheArgv))
	for k := range p.CacheArgv {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	best, n := "", 0
	for _, k := range keys {
		words := strings.Fields(k)
		if len(words) <= n || len(words) > len(argv) {
			continue
		}
		if words[0] != argv[0] && toolName(words[0]) != toolName(argv[0]) {
			continue
		}
		if slices.Equal(words[1:], argv[1:len(words)]) {
			best, n = k, len(words)
		}
	}
	return best, n
}

// PackArgv returns argv with p's cache_argv inserted right after the longest
// key that prefixes it ("swift build" -> swift build --scratch-path X …),
// templates expanded. argv is returned unchanged (a copy) when no key
// matches.
func PackArgv(runDir string, p pack.Pack, argv []string) []string {
	out := append([]string(nil), argv...)
	if len(argv) == 0 {
		return out
	}
	k, n := cacheArgvKey(p, argv)
	if n == 0 {
		return out
	}
	r := expander(runDir, p.ID)
	res := append([]string(nil), argv[:n]...)
	for _, a := range p.CacheArgv[k] {
		res = append(res, r.Replace(a))
	}
	return append(res, argv[n:]...)
}

// UsesTool reports whether pack p drives the tool argv0 names: the pack id,
// a cache_argv key, a check's tool or argv[0], a test command or the
// post-edit formatter.
func UsesTool(p pack.Pack, argv0 string) bool {
	t := toolName(argv0)
	if t == "" || t == "." || t == "/" {
		return false
	}
	if p.ID == t {
		return true
	}
	for k := range p.CacheArgv {
		if f := strings.Fields(k); len(f) > 0 && toolName(f[0]) == t {
			return true
		}
	}
	for _, c := range p.Checks {
		if toolName(c.Tool) == t || (len(c.Argv) > 0 && toolName(c.Argv[0]) == t) {
			return true
		}
	}
	for _, tc := range p.TestCommands {
		if len(tc.ArgvPrefix) > 0 && toolName(tc.ArgvPrefix[0]) == t {
			return true
		}
	}
	return p.PostEdit != nil && len(p.PostEdit.FormatFile) > 0 && toolName(p.PostEdit.FormatFile[0]) == t
}

// PickPack chooses the pack `loops exec` scopes a command to: the pack
// named id when id is set, else the first pack that uses the tool argv0
// names, else the only pack. ok is false when none fits.
func PickPack(packs []pack.Pack, id, argv0 string) (pack.Pack, bool) {
	if id != "" {
		for _, p := range packs {
			if p.ID == id {
				return p, true
			}
		}
		return pack.Pack{}, false
	}
	for _, p := range packs {
		if UsesTool(p, argv0) {
			return p, true
		}
	}
	if len(uniqueIDs(packs)) == 1 {
		return packs[0], true
	}
	return pack.Pack{}, false
}

// uniqueIDs returns the pack ids in first-seen order (a pack detected at
// several roots shares one run dir).
func uniqueIDs(packs []pack.Pack) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range packs {
		if !seen[p.ID] {
			seen[p.ID] = true
			out = append(out, p.ID)
		}
	}
	return out
}

// Unique returns packs with one entry per id (the first), dropping ids that
// cannot name a run directory.
func Unique(packs []pack.Pack) []pack.Pack {
	var out []pack.Pack
	seen := map[string]bool{}
	for _, p := range packs {
		if !seen[p.ID] && ValidPackID(p.ID) {
			seen[p.ID] = true
			out = append(out, p)
		}
	}
	return out
}

// ExportsEnv renders statements that set each K=V entry of env in shell
// ("sh"/"bash"/"zsh"/"" = POSIX, "fish", "powershell"/"pwsh"), in order. An
// empty env renders "" (after the shell is validated); a key that is not a
// shell-safe name is an error.
func ExportsEnv(shell string, env []string) (string, error) {
	var q func(string) string
	var set func(k, v string) string
	sep, prefix := "; ", ""
	switch shell {
	case "", "sh", "bash", "zsh", "dash", "ksh":
		q = ShellQuote
		set = func(k, v string) string { return k + "=" + v }
		sep, prefix = " ", "export "
	case "fish":
		q = func(s string) string { return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'" }
		set = func(k, v string) string { return "set -gx " + k + " " + v }
	case "powershell", "pwsh":
		q = func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
		set = func(k, v string) string { return "$env:" + k + "=" + v }
	default:
		return "", fmt.Errorf("unknown shell %q (want sh, fish or powershell)", shell)
	}
	parts := make([]string, 0, len(env))
	for _, kv := range env {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || !envKeyRe.MatchString(k) {
			return "", fmt.Errorf("bad environment entry %q", kv)
		}
		parts = append(parts, set(k, q(v)))
	}
	if len(parts) == 0 {
		return "", nil
	}
	return prefix + strings.Join(parts, sep), nil
}
