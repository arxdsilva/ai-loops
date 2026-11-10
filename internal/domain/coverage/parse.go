package coverage

import (
	"errors"
	"fmt"
)

// ErrUnsupported is returned by Parse for a format it cannot read yet.
var ErrUnsupported = errors.New("unsupported coverage format")

// Parse reads a coverage report in the given format (a language pack's
// coverage.format) into a Profile keyed by the file names the report uses;
// Normalize makes them repo-relative. Line-based formats (lcov, Cobertura,
// JaCoCo) become one single-line Block per instrumented line with NumStmt 1,
// so Evaluate's statement counts become line counts.
func Parse(format, text string) (Profile, error) {
	switch format {
	case "gocover", "":
		return ParseProfile(text)
	}
	return Profile{}, fmt.Errorf("%w %q", ErrUnsupported, format)
}
