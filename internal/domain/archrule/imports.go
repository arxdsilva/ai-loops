package archrule

import "strings"

// importFindings applies the dependency direction to every import spec of
// p, a package in layer lay of the module modPath.
func importFindings(modPath string, p Package, lay layer, c classifier) []Finding {
	var out []Finding
	for _, imp := range p.Imports {
		rel, internal := relImport(modPath, imp.Path)
		target := layerNone
		if internal {
			target = c.layerOf(rel)
		}
		add := func(sev Severity, why, fix, section string) {
			f := errorAt(imp.Pos, "imports", "%s (%s) imports %s: %s; %s (%s: %s)", p.Dir, lay, imp.Path, why, fix, hexSpec, section)
			f.Severity = sev
			out = append(out, f)
		}
		const inward = "define the interface where it is consumed and let an outer layer implement it"
		switch {
		case lay == layerDomain && internal && outwardOfDomain(target):
			add(SeverityError, "the domain must not depend on the "+target.String()+" layer", inward, "dependency direction")
		case lay == layerDomain && !internal && (stdUnder(imp.Path, "os") || stdUnder(imp.Path, "net/http")):
			add(SeverityError, "the domain does no I/O (files, processes, env, HTTP)", "take the data as parameters or through a port an adapter implements", "dependency direction")
		case (lay == layerApplication || lay == layerPorts) && isAdapter(target):
			add(SeverityError, "the "+lay.String()+" layer must not know the adapters ("+target.String()+")", inward, "dependency direction")
		case lay == layerOutbound && target == layerInbound:
			add(SeverityError, "outbound adapters must not know inbound adapters", "move the shared code inward (application or domain) or into platform", "dependency direction")
		}
		if (lay == layerDomain || lay == layerApplication || lay == layerPkg) && !internal && thirdParty(imp.Path) {
			add(SeverityError, "the "+lay.String()+" layer uses only the standard library", "wrap the dependency in an adapter behind a port", "dependency direction")
		}
		if internal && target == layerCompose && lay != layerCmd && lay != layerCompose {
			add(SeverityError, "only cmd may import the composition root", "have cmd/<x>/main.go call the compose package and pass dependencies in as interfaces", "composition root")
		}
		if internal && rel != "." && rel != p.Dir && isUnder(p.Dir, rel) {
			sev := SeverityWarn
			if lay == layerDomain || lay == layerApplication {
				sev = SeverityError
			}
			add(sev, "a sub-package imports its parent package "+rel, "the parent may use the sub-package, never the reverse: move the shared types into the sub-package or a sibling", "dependency direction")
		}
	}
	return out
}

// outwardOfDomain lists the layers the domain must not import; compose has
// its own rule.
func outwardOfDomain(l layer) bool {
	switch l {
	case layerApplication, layerPorts, layerInbound, layerOutbound, layerAdapters, layerPlatform, layerCmd:
		return true
	}
	return false
}

// isAdapter: inbound, outbound, or any other package under the adapters root.
func isAdapter(l layer) bool {
	return l == layerInbound || l == layerOutbound || l == layerAdapters
}

// relImport maps an import path to a module-relative directory.
func relImport(modPath, imp string) (string, bool) {
	if modPath == "" {
		return "", false
	}
	if imp == modPath {
		return ".", true
	}
	if rest, ok := strings.CutPrefix(imp, modPath+"/"); ok {
		return rest, true
	}
	return "", false
}

func stdUnder(imp, p string) bool { return imp == p || strings.HasPrefix(imp, p+"/") }

// thirdParty: a module path's first element has a dot; the standard
// library's does not.
func thirdParty(p string) bool {
	first, _, _ := strings.Cut(p, "/")
	return strings.Contains(first, ".")
}
