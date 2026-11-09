// Package shimtest runs the real shell scripts (the generated package shim,
// the root bin/loops and install.sh) against a fake release server.
package shimtest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/application/port"
	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

const version = "9.9.9"

func repoRoot(t *testing.T) string {
	t.Helper()
	d, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func platformAsset(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("unix only")
	}
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skip("unsupported arch")
	}
	return "loops-" + runtime.GOOS + "-" + runtime.GOARCH
}

// release serves /v9.9.9/<asset> and checksums.txt. A nil sums means
// the real checksum; "bad" serves a wrong one.
type release struct {
	srv   *httptest.Server
	hits  []string // only read after the shim exited
	sum   string   // "", "bad", "none" (no checksums.txt) or "gone"
	empty bool     // 404 everything
}

// newRelease starts the fake server; sum "gone" makes every request 404.
func newRelease(t *testing.T, asset, sum string) *release {
	t.Helper()
	body := "#!/bin/sh\necho \"loops " + version + " $*\"\n"
	h := sha256.Sum256([]byte(body))
	r := &release{sum: sum, empty: sum == "gone"}
	r.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.hits = append(r.hits, req.URL.Path)
		switch {
		case r.empty:
			http.NotFound(w, req)
		case req.URL.Path == "/v"+version+"/"+asset:
			fmt.Fprint(w, body)
		case req.URL.Path == "/v"+version+"/checksums.txt" && r.sum != "none":
			s := hex.EncodeToString(h[:])
			if r.sum == "bad" {
				s = strings.Repeat("0", 64)
			}
			fmt.Fprintf(w, "%s  %s\n%s  other\n", s, asset, s)
		case req.URL.Path == "/latest":
			w.Header().Set("Location", "/releases/tag/v"+version)
			w.WriteHeader(http.StatusFound)
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(r.srv.Close)
	return r
}

func generatedShim(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	p, err := port.Load(os.DirFS(root))
	if err != nil {
		t.Fatal(err)
	}
	files, _, err := port.Build(p, os.DirFS(root), plugin.Codex, port.Options{Version: "v" + version})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Path == "bin/loops" {
			dir := filepath.Join(t.TempDir(), "pkg", "bin")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "loops")
			if err := os.WriteFile(path, f.Data, 0o755); err != nil {
				t.Fatal(err)
			}
			return path
		}
	}
	t.Fatal("no bin/loops generated")
	return ""
}

// run executes script with a PATH that has no go and no loops.
func run(t *testing.T, script string, env []string, args ...string) (string, string, error) {
	t.Helper()
	home := t.TempDir()
	cmd := exec.Command("/bin/sh", append([]string{script}, args...)...)
	cmd.Env = append([]string{"PATH=/usr/bin:/bin", "HOME=" + home, "XDG_CACHE_HOME=" + filepath.Join(home, "cache"), "TMPDIR=" + home}, env...)
	var so, se strings.Builder
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	return so.String(), se.String(), err
}

func noGo(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	if _, err := os.Stat("/usr/bin/go"); err == nil {
		t.Skip("go in /usr/bin")
	}
	if _, err := os.Stat("/bin/go"); err == nil {
		t.Skip("go in /bin")
	}
}

func TestGeneratedShimDownloadsVerifiesCaches(t *testing.T) {
	noGo(t)
	asset := platformAsset(t)
	r := newRelease(t, asset, "")
	shim := generatedShim(t)
	cacheDir := t.TempDir()
	env := []string{"LOOPS_RELEASE_BASE=" + r.srv.URL, "LOOPS_BIN_CACHE=" + cacheDir}

	out, errOut, err := run(t, shim, env, "hello")
	if err != nil || strings.TrimSpace(out) != "loops 9.9.9 hello" {
		t.Fatalf("first run: out=%q err=%v stderr=%s", out, err, errOut)
	}
	if fi, err := os.Stat(filepath.Join(cacheDir, "loops-"+version)); err != nil || fi.Mode()&0o111 == 0 {
		t.Fatalf("binary not cached executable: %v", err)
	}
	n := len(r.hits)
	out, _, err = run(t, shim, env, "again")
	if err != nil || strings.TrimSpace(out) != "loops 9.9.9 again" {
		t.Fatalf("second run: %q %v", out, err)
	}
	if len(r.hits) != n {
		t.Errorf("cached run hit the network: %v", r.hits[n:])
	}
	if ents, _ := filepath.Glob(filepath.Join(cacheDir, ".dl-*")); len(ents) != 0 {
		t.Errorf("temp dirs left: %v", ents)
	}
}

func TestGeneratedShimRefusesBadChecksum(t *testing.T) {
	noGo(t)
	asset := platformAsset(t)
	for _, sum := range []string{"bad", "none"} {
		r := newRelease(t, asset, sum)
		cacheDir := t.TempDir()
		out, errOut, err := run(t, generatedShim(t), []string{"LOOPS_RELEASE_BASE=" + r.srv.URL, "LOOPS_BIN_CACHE=" + cacheDir})
		if err == nil || out != "" {
			t.Fatalf("%s: ran an unverified binary: out=%q err=%v", sum, out, err)
		}
		if !strings.Contains(errOut, "refus") {
			t.Errorf("%s: stderr = %q", sum, errOut)
		}
		if _, err := os.Stat(filepath.Join(cacheDir, "loops-"+version)); err == nil {
			t.Errorf("%s: unverified binary was cached", sum)
		}
	}
}

func TestGeneratedShimSkipVerify(t *testing.T) {
	noGo(t)
	asset := platformAsset(t)
	r := newRelease(t, asset, "bad")
	out, _, err := run(t, generatedShim(t), []string{"LOOPS_RELEASE_BASE=" + r.srv.URL, "LOOPS_BIN_CACHE=" + t.TempDir(), "LOOPS_SKIP_VERIFY=1"})
	if err != nil || strings.TrimSpace(out) != "loops 9.9.9" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestGeneratedShimNoReleaseNoGo(t *testing.T) {
	noGo(t)
	asset := platformAsset(t)
	r := newRelease(t, asset, "gone")
	_, errOut, err := run(t, generatedShim(t), []string{"LOOPS_RELEASE_BASE=" + r.srv.URL, "LOOPS_BIN_CACHE=" + t.TempDir()})
	if err == nil {
		t.Fatal("expected failure")
	}
	for _, want := range []string{"no prebuilt binary for " + runtime.GOOS + "/" + runtime.GOARCH + " v9.9.9 and no Go toolchain", "https://github.com/arxdsilva/ai-loops#install"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("stderr lacks %q: %s", want, errOut)
		}
	}
}

func TestGeneratedShimPathScriptIgnoredBinaryUsed(t *testing.T) {
	noGo(t)
	asset := platformAsset(t)
	r := newRelease(t, asset, "")
	// A script named loops on PATH reporting the right version must be ignored.
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "loops"), []byte("#!/bin/sh\necho 'loops 9.9.9'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	cmd := exec.Command("/bin/sh", generatedShim(t), "x")
	cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "HOME=" + home, "LOOPS_RELEASE_BASE=" + r.srv.URL, "LOOPS_BIN_CACHE=" + t.TempDir()}
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) != "loops 9.9.9 x" {
		t.Fatalf("out=%q err=%v (script on PATH should be skipped for the download)", out, err)
	}
}

func TestGeneratedShimFollowsSymlink(t *testing.T) {
	noGo(t)
	asset := platformAsset(t)
	r := newRelease(t, asset, "")
	link := filepath.Join(t.TempDir(), "loops")
	if err := os.Symlink(generatedShim(t), link); err != nil {
		t.Fatal(err)
	}
	out, errOut, err := run(t, link, []string{"LOOPS_RELEASE_BASE=" + r.srv.URL, "LOOPS_BIN_CACHE=" + t.TempDir()})
	if err != nil || strings.TrimSpace(out) != "loops 9.9.9" {
		t.Fatalf("out=%q err=%v %s", out, err, errOut)
	}
}

// TestRootShimSharesLib keeps bin/loops and the generated shim in sync and
// checks the root shim resolves like the generated one outside a git checkout.
func TestRootShimSharesLib(t *testing.T) {
	root := repoRoot(t)
	rootShim, err := os.ReadFile(filepath.Join(root, "bin", "loops"))
	if err != nil {
		t.Fatal(err)
	}
	gen, err := os.ReadFile(generatedShim(t))
	if err != nil {
		t.Fatal(err)
	}
	block := func(b []byte) string {
		s := string(b)
		i, j := strings.Index(s, "# --- shared begin"), strings.Index(s, "# --- shared end")
		if i < 0 || j < i {
			t.Fatal("shared block markers missing")
		}
		return s[i:j]
	}
	if block(rootShim) != block(gen) {
		t.Error("bin/loops and the generated shim have drifted; shared blocks differ")
	}
}

func TestRootShimDownloadsWithoutGit(t *testing.T) {
	noGo(t)
	asset := platformAsset(t)
	r := newRelease(t, asset, "")
	// Fake plugin checkout without .git: bin/loops + plugin.json.
	pkg := t.TempDir()
	if err := os.MkdirAll(filepath.Join(pkg, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(pkg, ".claude-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join(repoRoot(t), "bin", "loops"))
	if err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(pkg, "bin", "loops")
	if err := os.WriteFile(shim, src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, ".claude-plugin", "plugin.json"), []byte("{\n  \"name\": \"loops\",\n  \"version\": \""+version+"\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	out, errOut, err := run(t, shim, []string{"LOOPS_RELEASE_BASE=" + r.srv.URL, "CLAUDE_PLUGIN_DATA=" + cacheDir}, "q")
	if err != nil || strings.TrimSpace(out) != "loops 9.9.9 q" {
		t.Fatalf("out=%q err=%v %s", out, err, errOut)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "loops-"+version)); err != nil {
		t.Errorf("not cached in CLAUDE_PLUGIN_DATA: %v", err)
	}
	// Bad checksum is refused here too.
	rb := newRelease(t, asset, "bad")
	_, errOut, err = run(t, shim, []string{"LOOPS_RELEASE_BASE=" + rb.srv.URL, "CLAUDE_PLUGIN_DATA=" + t.TempDir()})
	if err == nil || !strings.Contains(errOut, "checksum mismatch") {
		t.Errorf("bad checksum: err=%v stderr=%s", err, errOut)
	}
}

func installSh(t *testing.T) string { return filepath.Join(repoRoot(t), "install.sh") }

func TestInstallScript(t *testing.T) {
	asset := platformAsset(t)
	r := newRelease(t, asset, "")
	bindir := filepath.Join(t.TempDir(), "bin")
	out, errOut, err := run(t, installSh(t), []string{"LOOPS_RELEASE_BASE=" + r.srv.URL}, "--version", "v"+version, "--bin-dir", bindir)
	if err != nil {
		t.Fatalf("err=%v out=%s stderr=%s", err, out, errOut)
	}
	if !strings.Contains(out, "loops 9.9.9") || !strings.Contains(errOut, "not on your PATH") {
		t.Errorf("out=%q stderr=%q", out, errOut)
	}
	if fi, err := os.Stat(filepath.Join(bindir, "loops")); err != nil || fi.Mode()&0o111 == 0 {
		t.Fatalf("binary not installed: %v", err)
	}
}

func TestInstallScriptLatestAndTarget(t *testing.T) {
	asset := platformAsset(t)
	r := newRelease(t, asset, "")
	bindir := t.TempDir()
	// The fake binary echoes its args, so the target run is visible in stdout.
	out, errOut, err := run(t, installSh(t), []string{"LOOPS_RELEASE_BASE=" + r.srv.URL, "LOOPS_LATEST_URL=" + r.srv.URL + "/latest", "PATH=" + bindir + ":/usr/bin:/bin"},
		"--bin-dir", bindir, "--target", "opencode", "--scope", "user", "--force")
	if err != nil {
		t.Fatalf("err=%v out=%s stderr=%s", err, out, errOut)
	}
	if !strings.Contains(out, "loops 9.9.9 install --target opencode --scope user --force") {
		t.Errorf("target install not run: %q", out)
	}
	if strings.Contains(errOut, "not on your PATH") {
		t.Errorf("spurious PATH warning: %s", errOut)
	}
}

func TestInstallScriptRefuses(t *testing.T) {
	asset := platformAsset(t)
	bad := newRelease(t, asset, "bad")
	bindir := filepath.Join(t.TempDir(), "bin")
	_, errOut, err := run(t, installSh(t), []string{"LOOPS_RELEASE_BASE=" + bad.srv.URL}, "--version", version, "--bin-dir", bindir)
	if err == nil || !strings.Contains(errOut, "checksum mismatch") {
		t.Errorf("bad checksum: %v %s", err, errOut)
	}
	if _, err := os.Stat(filepath.Join(bindir, "loops")); err == nil {
		t.Error("installed a bad binary")
	}
	gone := newRelease(t, asset, "gone")
	_, errOut, err = run(t, installSh(t), []string{"LOOPS_RELEASE_BASE=" + gone.srv.URL}, "--version", version, "--bin-dir", bindir)
	if err == nil || !strings.Contains(errOut, "no release binary") {
		t.Errorf("404: %v %s", err, errOut)
	}
	_, errOut, err = run(t, installSh(t), nil, "--bogus")
	if err == nil || !strings.Contains(errOut, "unknown option") {
		t.Errorf("bogus flag: %v %s", err, errOut)
	}
}
