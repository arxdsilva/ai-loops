// Package coverage judges the test coverage of new code: it parses Go
// coverprofiles and `git diff -U0` output and evaluates the changed
// statements against a minimum (specs/WORKFLOW.md §6). Pure: reading files
// and running git are adapters.
package coverage

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Block is one coverprofile block: a source span with its statement count
// and execution count.
type Block struct {
	StartLine, StartCol int
	EndLine, EndCol     int
	NumStmt             int
	Count               int
}

// Profile is a parsed coverage report. Files is keyed by the report's file
// name (for gocover: import path + "/" + base name) until Normalize re-keys
// it pack-relative; blocks are sorted and deduplicated.
type Profile struct {
	Mode  string
	Files map[string][]Block
	// Format is the report format Parse read ("" = gocover, from
	// ParseProfile).
	Format string
	// Sources are Cobertura's <sources> roots, which its class filenames
	// are relative to; Normalize resolves against them.
	Sources []string
	// Normalized marks Files as pack-relative (Normalize's output), so
	// Evaluate no longer strips Options.Module.
	Normalized bool
}

type blockKey struct {
	file           string
	sl, sc, el, ec int
}

// ParseProfile parses a coverprofile. Several concatenated profiles (each
// with its own "mode:" line) are merged; a block reported more than once
// keeps its highest count.
func ParseProfile(text string) (Profile, error) {
	p := Profile{Files: map[string][]Block{}}
	acc := map[blockKey]Block{}
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" {
			continue
		}
		if m, ok := strings.CutPrefix(line, "mode:"); ok {
			if p.Mode == "" {
				p.Mode = strings.TrimSpace(m)
			}
			continue
		}
		file, b, err := parseBlock(line)
		if err != nil {
			return Profile{}, fmt.Errorf("coverprofile line %d: %w", i+1, err)
		}
		add(acc, file, b)
	}
	p.Files = collect(acc)
	return p, nil
}

// MergeProfiles merges profiles (e.g. one per package run or per report
// file); duplicate blocks keep the highest count. Mode and Format come from
// the first profile that has one, Sources are united, and the result is
// Normalized only when every input is.
func MergeProfiles(ps ...Profile) Profile {
	acc := map[blockKey]Block{}
	out := Profile{Normalized: len(ps) > 0}
	for _, p := range ps {
		if out.Mode == "" {
			out.Mode = p.Mode
		}
		if out.Format == "" {
			out.Format = p.Format
		}
		for _, s := range p.Sources {
			out.Sources = appendUnique(out.Sources, s)
		}
		out.Normalized = out.Normalized && p.Normalized
		for f, bs := range p.Files {
			for _, b := range bs {
				add(acc, f, b)
			}
		}
	}
	out.Files = collect(acc)
	return out
}

func add(acc map[blockKey]Block, file string, b Block) {
	k := blockKey{file, b.StartLine, b.StartCol, b.EndLine, b.EndCol}
	if old, ok := acc[k]; ok && old.Count >= b.Count {
		return
	}
	acc[k] = b
}

func collect(acc map[blockKey]Block) map[string][]Block {
	out := map[string][]Block{}
	for k, b := range acc {
		out[k.file] = append(out[k.file], b)
	}
	for _, bs := range out {
		sort.Slice(bs, func(i, j int) bool {
			if bs[i].StartLine != bs[j].StartLine {
				return bs[i].StartLine < bs[j].StartLine
			}
			return bs[i].StartCol < bs[j].StartCol
		})
	}
	return out
}

// parseBlock parses "file:sl.sc,el.ec numStmt count".
func parseBlock(line string) (string, Block, error) {
	colon := strings.LastIndex(line, ":")
	if colon < 0 {
		return "", Block{}, fmt.Errorf("no file name in %q", line)
	}
	file, rest := line[:colon], strings.Fields(line[colon+1:])
	if len(rest) != 3 {
		return "", Block{}, fmt.Errorf("want \"sl.sc,el.ec stmts count\", got %q", line)
	}
	start, end, ok := strings.Cut(rest[0], ",")
	if !ok {
		return "", Block{}, fmt.Errorf("bad span %q", rest[0])
	}
	var b Block
	var err error
	if b.StartLine, b.StartCol, err = pos(start); err != nil {
		return "", Block{}, err
	}
	if b.EndLine, b.EndCol, err = pos(end); err != nil {
		return "", Block{}, err
	}
	if b.NumStmt, err = strconv.Atoi(rest[1]); err != nil {
		return "", Block{}, fmt.Errorf("bad statement count %q", rest[1])
	}
	if b.Count, err = strconv.Atoi(rest[2]); err != nil {
		return "", Block{}, fmt.Errorf("bad count %q", rest[2])
	}
	return file, b, nil
}

func pos(s string) (int, int, error) {
	l, c, ok := strings.Cut(s, ".")
	line, err1 := strconv.Atoi(l)
	col, err2 := strconv.Atoi(c)
	if !ok || err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("bad position %q", s)
	}
	return line, col, nil
}
