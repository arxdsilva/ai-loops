package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
)

// LangPacks loads and detects language packs (specs/languages/<id>/pack.json).
type LangPacks interface {
	All() ([]pack.Pack, error)
	Get(id string) (pack.Pack, error)
	Detect(root string, overrides []langpack.Override) ([]langpack.Detected, error)
}

const langUsage = "usage: loops lang list [--json] | show <id> [--variant v] | validate [pack.json…] | detect [--json] [--write] [--root dir]"

// cmdLang implements `loops lang …`.
func cmdLang(a *App, _ context.Context, args []string) int {
	if a.Langs == nil {
		return a.fail("lang", errors.New("language packs unavailable"))
	}
	if len(args) == 0 {
		fmt.Fprintln(a.Err, langUsage)
		return 2
	}
	l := langCmd{a: a}
	switch args[0] {
	case "list":
		return l.list(args[1:])
	case "show":
		return l.show(args[1:])
	case "validate":
		return l.validate(args[1:])
	case "detect":
		return l.detect(args[1:])
	}
	fmt.Fprintln(a.Err, langUsage)
	return 2
}

type langCmd struct{ a *App }

func (l langCmd) list(args []string) int {
	fs := l.a.newFlagSet("lang list")
	asJSON := fs.Bool("json", false, "print JSON")
	if _, err := parseFlagsAnywhere(fs, args); err != nil {
		return 2
	}
	all, err := l.a.Langs.All()
	if err != nil {
		return l.a.fail("lang", err)
	}
	if *asJSON {
		return l.json(all)
	}
	fmt.Fprintf(l.a.Out, "%-11s %-24s %-6s %-10s %s\n", "ID", "NAME", "RACE", "COVERAGE", "DETECT")
	for _, p := range all {
		fmt.Fprintf(l.a.Out, "%-11s %-24s %-6s %-10s %s\n", p.ID, p.Name, p.Race.Mode, p.Coverage.Format, markers(p.Detect))
	}
	return 0
}

func (l langCmd) show(args []string) int {
	fs := l.a.newFlagSet("lang show")
	variant := fs.String("variant", "", "apply this build-tool variant")
	rest, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(rest) != 1 {
		fmt.Fprintln(l.a.Err, langUsage)
		return 2
	}
	p, err := l.a.Langs.Get(rest[0])
	if err == nil && *variant != "" {
		p, err = p.WithVariant(*variant)
	}
	if err != nil {
		return l.a.fail("lang", err)
	}
	return l.json(p)
}

// validate parses the given pack files, or every embedded pack.
func (l langCmd) validate(files []string) int {
	if len(files) == 0 {
		all, err := l.a.Langs.All()
		if err != nil {
			fmt.Fprintln(l.a.Err, err)
			return 1
		}
		fmt.Fprintf(l.a.Out, "ok: %d packs\n", len(all))
		return 0
	}
	bad := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err == nil {
			_, err = pack.Parse(b)
		}
		if err != nil {
			bad++
			fmt.Fprintf(l.a.Err, "%s: %v\n", f, err)
			continue
		}
		fmt.Fprintf(l.a.Out, "ok: %s\n", f)
	}
	if bad > 0 {
		return 1
	}
	return 0
}

func (l langCmd) detect(args []string) int {
	fs := l.a.newFlagSet("lang detect")
	asJSON := fs.Bool("json", false, "print JSON")
	write := fs.Bool("write", false, "also write research/language.json into the current run dir")
	root := fs.String("root", "", "project root (default: the loops project root)")
	if _, err := parseFlagsAnywhere(fs, args); err != nil {
		return 2
	}
	dir := l.a.Root
	if *root != "" {
		dir = *root
	}
	ds, err := l.a.Langs.Detect(dir, l.a.Languages)
	if err != nil {
		return l.a.fail("lang", err)
	}
	if ds == nil {
		ds = []langpack.Detected{}
	}
	js, err := json.MarshalIndent(ds, "", "  ")
	if err != nil {
		return l.a.fail("lang", err)
	}
	js = append(js, '\n')
	if *write {
		run, err := l.a.Tracker.Dir(context.Background(), "")
		if err == nil {
			err = os.MkdirAll(filepath.Join(run, "research"), 0o755)
		}
		if err == nil {
			err = writeFileAtomic(filepath.Join(run, "research", "language.json"), js)
		}
		if err != nil {
			return l.a.fail("lang", err)
		}
	}
	if *asJSON {
		l.a.Out.Write(js)
		return 0
	}
	if len(ds) == 0 {
		fmt.Fprintln(l.a.Out, "no language pack detected (pin one with \"languages\" in .loops/config.json)")
		return 0
	}
	for _, d := range ds {
		v := ""
		if d.Variant != "" {
			v = " (" + d.Variant + ")"
		}
		fmt.Fprintf(l.a.Out, "%s%s at %s: race %s, coverage %s\n", d.ID, v, d.Root, d.Pack.Race.Mode, d.Pack.Coverage.Format)
	}
	return 0
}

func (l langCmd) json(v any) int {
	js, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return l.a.fail("lang", err)
	}
	l.a.Out.Write(append(js, '\n'))
	return 0
}

func markers(ms []pack.Marker) string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		s := m.File + m.Glob
		if m.Contains != "" {
			s += "∋" + m.Contains
		}
		out = append(out, s)
	}
	return strings.Join(out, " | ")
}
