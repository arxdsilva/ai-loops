package archrule

import (
	"path"
	"strings"
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
// matching prefix wins.
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
		if isUnder(dir, p) && len(p) > bestLen {
			best, bestLen = c.layers[i], len(p)
		}
	}
	return best
}

// isCmdDir reports a direct child of a cmd prefix: cmd/<x>.
func (c classifier) isCmdDir(dir string) bool {
	for _, p := range c.cmd {
		if p != "." && path.Dir(dir) == p {
			return true
		}
	}
	return false
}
