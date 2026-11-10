package coverage

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ErrUnsupported is returned by Parse for a format it cannot read yet.
var ErrUnsupported = errors.New("unsupported coverage format")

// Coverage formats Parse reads (a language pack's coverage.format).
const (
	FormatGocover   = "gocover"
	FormatLcov      = "lcov"
	FormatCobertura = "cobertura"
	FormatJacoco    = "jacoco"
)

// Parse reads a coverage report in the given format (a language pack's
// coverage.format) into a Profile keyed by the file names the report uses;
// Normalize makes them pack-relative. Line-based formats (lcov, Cobertura,
// JaCoCo) become one single-line Block per instrumented line with NumStmt 1,
// so Evaluate's statement counts become line counts. Several reports of one
// format are combined with MergeProfiles.
func Parse(format, text string) (Profile, error) {
	var (
		p   Profile
		err error
	)
	switch format {
	case FormatGocover, "":
		p, err = ParseProfile(text)
		format = FormatGocover
	case FormatLcov:
		p, err = parseLcov(text)
	case FormatCobertura:
		p, err = parseCobertura(text)
	case FormatJacoco:
		p, err = parseJacoco(text)
	default:
		return Profile{}, fmt.Errorf("%w %q", ErrUnsupported, format)
	}
	if err != nil {
		return Profile{}, err
	}
	p.Format = format
	return p, nil
}

// lineBlock is the Block of one instrumented line.
func lineBlock(line, hits int) Block {
	return Block{StartLine: line, EndLine: line, NumStmt: 1, Count: hits}
}

// parseLcov reads lcov tracefiles: SF/DA/end_of_record records; every other
// record (TN, VER, FN*, BR*, LF, LH) is ignored. A file listed in several
// records is merged.
func parseLcov(text string) (Profile, error) {
	acc := map[blockKey]Block{}
	file := ""
	for i, line := range strings.Split(bom(text), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		switch {
		case line == "end_of_record":
			file = ""
		case strings.HasPrefix(line, "SF:"):
			file = strings.TrimSpace(line[len("SF:"):])
			if file == "" {
				return Profile{}, fmt.Errorf("lcov line %d: empty SF", i+1)
			}
		case strings.HasPrefix(line, "DA:"):
			if file == "" {
				return Profile{}, fmt.Errorf("lcov line %d: DA outside an SF record", i+1)
			}
			f := strings.Split(line[len("DA:"):], ",")
			if len(f) < 2 {
				return Profile{}, fmt.Errorf("lcov line %d: want DA:line,hits, got %q", i+1, line)
			}
			ln, err := strconv.Atoi(strings.TrimSpace(f[0]))
			if err != nil || ln <= 0 {
				return Profile{}, fmt.Errorf("lcov line %d: bad line number %q", i+1, f[0])
			}
			hits, err := parseCount(f[1])
			if err != nil {
				return Profile{}, fmt.Errorf("lcov line %d: %w", i+1, err)
			}
			add(acc, file, lineBlock(ln, hits))
		}
	}
	return Profile{Files: collect(acc)}, nil
}

// parseCount reads an execution count: an integer, or a number some tools
// print in float or exponent form. Negative counts read as 0; huge ones
// saturate.
func parseCount(s string) (int, error) {
	s = strings.TrimSpace(s)
	if n, err := strconv.Atoi(s); err == nil {
		return max(n, 0), nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) {
		return 0, fmt.Errorf("bad hit count %q", s)
	}
	switch {
	case f <= 0:
		return 0, nil
	case f >= math.MaxInt32:
		return math.MaxInt32, nil
	}
	return int(math.Ceil(f)), nil
}
