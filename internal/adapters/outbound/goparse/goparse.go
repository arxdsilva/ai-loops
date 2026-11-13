// Package goparse parses a Go module with go/parser into archrule.Module.
//
// It is syntax only: no type checking and no build-tag evaluation, so it
// runs in well under a second on a mid-size module. Known blind spots:
//
//   - every .go file counts, whatever its build constraints (a file guarded
//     by //go:build ignore is measured like any other);
//   - a dot import of time or math/rand hides its calls, and a local
//     variable named like an imported package is mistaken for it;
//   - methods a type gains through an alias or embedding are not counted,
//     and an embedded interface counts as one method;
//   - package-level vars assigned inside a package-level func literal
//     (var f = func() { x = 1 }) are not seen: only function declarations
//     are walked, as in the rimworld-go guard this ports.
//   - a global written through an alias is not seen: p := &g; *p = 1,
//     delete(g, k), json.Unmarshal(b, &g), flag.IntVar(&g, ...).
package goparse

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/arxdsilva/ai-loops/internal/domain/archrule"
)

// sourceParser implements archcheck.Parser.
type sourceParser struct{}

// New returns a sourceParser.
func New() *sourceParser { return &sourceParser{} }

// dirInfo is one walked directory.
type dirInfo struct {
	rel       string // module-relative, slash-separated, "." for the root
	abs       string
	entries   []string
	files     []string // non-test .go file names
	testFiles []string // _test.go file names
}

// Parse walks root, skipping testdata, vendor, hidden and _ directories
// and directories holding their own go.mod, and parses every package.
func (p *sourceParser) Parse(ctx context.Context, root string) (archrule.Module, error) {
	modPath, err := modulePath(filepath.Join(root, "go.mod"))
	if err != nil {
		return archrule.Module{}, err
	}
	dirs, err := walk(ctx, root)
	if err != nil {
		return archrule.Module{}, err
	}
	m := archrule.Module{Path: modPath}
	var todo []dirInfo
	for _, d := range dirs {
		m.Dirs = append(m.Dirs, archrule.Dir{Path: d.rel, Entries: d.entries})
		if len(d.files)+len(d.testFiles) > 0 {
			todo = append(todo, d)
		}
	}
	pkgs, err := parseAll(ctx, modPath, todo)
	if err != nil {
		return archrule.Module{}, err
	}
	m.Packages = pkgs
	return m, nil
}

// modulePath reads the module directive of a go.mod file.
func modulePath(gomod string) (string, error) {
	src, err := os.ReadFile(gomod)
	if err != nil {
		return "", fmt.Errorf("goparse: read go.mod: %w", err)
	}
	sc := bufio.NewScanner(bytes.NewReader(src))
	for sc.Scan() {
		line := sc.Text()
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			if uq, err := strconv.Unquote(fields[1]); err == nil {
				return uq, nil
			}
			return fields[1], nil
		}
	}
	return "", fmt.Errorf("goparse: %s has no module directive", gomod)
}

// skipDir reports the directories the go tool ignores or that are not part
// of this module.
func skipDir(abs, name string) bool {
	if name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		return true
	}
	_, err := os.Stat(filepath.Join(abs, "go.mod"))
	return err == nil
}

func walk(ctx context.Context, root string) ([]dirInfo, error) {
	var dirs []dirInfo
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path != root && skipDir(path, d.Name()) {
			return filepath.SkipDir
		}
		info, err := readDir(root, path)
		if err != nil {
			return err
		}
		dirs = append(dirs, info)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("goparse: walk %s: %w", root, err)
	}
	return dirs, nil
}

func readDir(root, abs string) (dirInfo, error) {
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return dirInfo{}, err
	}
	info := dirInfo{rel: filepath.ToSlash(rel), abs: abs}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return dirInfo{}, err
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() {
			info.entries = append(info.entries, name+"/")
			continue
		}
		info.entries = append(info.entries, name)
		if !strings.HasSuffix(name, ".go") || strings.HasPrefix(name, "_") || !e.Type().IsRegular() {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			info.testFiles = append(info.testFiles, name)
		} else {
			info.files = append(info.files, name)
		}
	}
	return info, nil
}

// parseAll parses the packages in parallel, at most GOMAXPROCS at a time.
func parseAll(ctx context.Context, modPath string, dirs []dirInfo) ([]archrule.Package, error) {
	out := make([]archrule.Package, len(dirs))
	errs := make([]error, len(dirs))
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	var wg sync.WaitGroup
	for i, d := range dirs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := ctx.Err(); err != nil {
				errs[i] = err
				return
			}
			out[i], errs[i] = parsePackage(modPath, d)
		}()
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Dir < out[j].Dir })
	return out, nil
}
