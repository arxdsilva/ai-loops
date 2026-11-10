package archrule

import (
	"path"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/glob"
)

type layer int

const (
	layerNone layer = iota
	layerDomain
	layerApplication
	layerPorts
	layerInbound
	layerOutbound
	layerPlatform
	layerCompose
	layerCmd
	layerPkg
	layerAdapters
)

var layerNames = [...]string{"other", "domain", "application", "ports", "inbound", "outbound", "platform", "compose", "cmd", "pkg", "adapters"}

func (l layer) String() string { return layerNames[l] }

// classifier maps module-relative directories to layers; the longest
// matching prefix wins. An entry holding "*" is a glob (internal/domain/glob)
// that a leading part of the directory must match ("**/domain" matches
// a/domain and a/domain/b); its length is that of the matched part.
type classifier struct {
	prefixes []string
	layers   []layer
	cmd      []string
}

func newClassifier(l Layers) classifier {
	def := DefaultLayers()
	pick := func(got, fallback []string) []string {
		if len(got) == 0 {
			return fallback
		}
		return got
	}
	groups := []struct {
		l     layer
		paths []string
	}{
		{layerDomain, pick(l.Domain, def.Domain)},
		{layerApplication, pick(l.Application, def.Application)},
		{layerPorts, pick(l.Ports, def.Ports)},
		{layerInbound, pick(l.Inbound, def.Inbound)},
		{layerOutbound, pick(l.Outbound, def.Outbound)},
		{layerPlatform, pick(l.Platform, def.Platform)},
		{layerCompose, pick(l.Compose, def.Compose)},
		{layerCmd, pick(l.Cmd, def.Cmd)},
		{layerPkg, pick(l.Pkg, def.Pkg)},
		{layerAdapters, pick(l.Adapters, def.Adapters)},
	}
	var c classifier
	for _, g := range groups {
		for _, p := range g.paths {
			p = cleanPrefix(p)
			c.prefixes = append(c.prefixes, p)
			c.layers = append(c.layers, g.l)
			if g.l == layerCmd {
				c.cmd = append(c.cmd, p)
			}
		}
	}
	return c
}

// cleanPrefix normalises a configured prefix: "./cmd/" becomes "cmd".
func cleanPrefix(p string) string {
	p = path.Clean(strings.ReplaceAll(p, "\\", "/"))
	return strings.TrimPrefix(p, "/")
}

func (c classifier) layerOf(dir string) layer {
	best, bestLen := layerNone, -1
	for i, p := range c.prefixes {
		if n := matchLen(dir, p); n > bestLen {
			best, bestLen = c.layers[i], n
		}
	}
	return best
}

// matchLen is the length of the part of dir that prefix p covers, -1 when
// dir is not under p.
func matchLen(dir, p string) int {
	if !strings.Contains(p, "*") {
		if isUnder(dir, p) {
			return len(p)
		}
		return -1
	}
	segs := strings.Split(dir, "/")
	for i := len(segs); i > 0; i-- {
		if head := strings.Join(segs[:i], "/"); glob.Match(p, head) {
			return len(head)
		}
	}
	return -1
}

// isCmdDir reports a direct child of a cmd prefix: cmd/<x>.
func (c classifier) isCmdDir(dir string) bool {
	for _, p := range c.cmd {
		if p != "." && path.Dir(dir) == p {
			return true
		}
		if strings.Contains(p, "*") && glob.Match(p, path.Dir(dir)) {
			return true
		}
	}
	return false
}
