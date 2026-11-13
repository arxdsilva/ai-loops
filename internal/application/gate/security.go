package gate

import (
	"slices"

	"github.com/arxdsilva/ai-loops/internal/application/langpack"
)

// SecurityPass is the QA security pass: each pack's Security checks,
// enabled, missing tools skipped, nothing recorded; notes name packs
// without tools (WORKFLOW §2.4). It returns o with PackChecks, Enable,
// Missing and NoRecord set, and one note per pack that has no tools.
// o's slices and maps are copied, never mutated.
func SecurityPass(o Options) (Options, []string) {
	pc := make(map[string][]string, len(o.Packs))
	enable := slices.Clone(o.Enable)
	var notes []string
	noted := map[string]bool{}
	for _, d := range o.Packs {
		if !noted[d.ID] {
			noted[d.ID] = true
			notes = appendSecurityNote(notes, d)
		}
		names := pc[d.ID]
		for _, c := range d.Pack.SecurityChecks() {
			if !slices.Contains(names, c.Name) {
				names = append(names, c.Name)
			}
			if !contains(enable, c.Name) {
				enable = append(enable, c.Name)
			}
		}
		pc[d.ID] = names // present even when empty: the pack runs nothing
	}
	o.Packs = slices.Clone(o.Packs)
	o.PackChecks = pc
	o.Enable = enable
	o.Missing = MissingSkip
	o.NoRecord = true
	return o, notes
}

func appendSecurityNote(notes []string, d langpack.Detected) []string {
	switch s := d.Pack.Security; {
	case s == nil:
		return append(notes, "gate: "+d.ID+": security not declared")
	case s.None != "":
		return append(notes, "gate: "+d.ID+": no security tools ("+s.None+")")
	}
	return notes
}
