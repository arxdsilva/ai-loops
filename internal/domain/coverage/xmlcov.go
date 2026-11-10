package coverage

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Cobertura: <coverage><sources><source>…<packages><package><classes>
// <class filename><lines><line number hits>. Method-level <lines> repeat the
// class lines and are not read.
type coberturaXML struct {
	Sources  []string `xml:"sources>source"`
	Packages []struct {
		Classes []struct {
			Filename string `xml:"filename,attr"`
			Lines    []struct {
				Number string `xml:"number,attr"`
				Hits   string `xml:"hits,attr"`
			} `xml:"lines>line"`
		} `xml:"classes>class"`
	} `xml:"packages>package"`
}

// JaCoCo (and Kover's XML): <report>[<group>…]<package name="a/b">
// <sourcefile name="X.java"><line nr mi ci mb cb/>.
type jacocoGroup struct {
	Groups   []jacocoGroup `xml:"group"`
	Packages []struct {
		Name        string `xml:"name,attr"`
		Sourcefiles []struct {
			Name  string `xml:"name,attr"`
			Lines []struct {
				Nr string `xml:"nr,attr"`
				MI string `xml:"mi,attr"`
				CI string `xml:"ci,attr"`
			} `xml:"line"`
		} `xml:"sourcefile"`
	} `xml:"package"`
}

// decodeXML decodes a report without fetching its DTD (encoding/xml never
// does) and accepts any declared charset: Latin-1 is converted, everything
// else is read as UTF-8.
func decodeXML(text, root string, v any) error {
	d := xml.NewDecoder(strings.NewReader(bom(text)))
	d.CharsetReader = func(charset string, in io.Reader) (io.Reader, error) {
		switch strings.ToLower(charset) {
		case "iso-8859-1", "latin1", "latin-1", "iso8859-1":
			b, err := io.ReadAll(in)
			if err != nil {
				return nil, err
			}
			rs := make([]rune, len(b))
			for i, c := range b {
				rs[i] = rune(c)
			}
			return strings.NewReader(string(rs)), nil
		}
		return in, nil
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("%s report: %w", root, err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if se.Name.Local != root {
			return fmt.Errorf("%s report: root element <%s>, want <%s>", root, se.Name.Local, root)
		}
		if err := d.DecodeElement(v, &se); err != nil {
			return fmt.Errorf("%s report: %w", root, err)
		}
		return nil
	}
}

func parseCobertura(text string) (Profile, error) {
	var c coberturaXML
	if err := decodeXML(text, "coverage", &c); err != nil {
		return Profile{}, err
	}
	acc := map[blockKey]Block{}
	for _, pkg := range c.Packages {
		for _, cl := range pkg.Classes {
			file := strings.TrimSpace(cl.Filename)
			if file == "" {
				continue
			}
			for _, l := range cl.Lines {
				ln, err := strconv.Atoi(strings.TrimSpace(l.Number))
				if err != nil || ln <= 0 {
					return Profile{}, fmt.Errorf("cobertura report: %s: bad line number %q", file, l.Number)
				}
				hits, err := parseCount(l.Hits)
				if err != nil {
					return Profile{}, fmt.Errorf("cobertura report: %s:%d: %w", file, ln, err)
				}
				add(acc, file, lineBlock(ln, hits))
			}
		}
	}
	var sources []string
	for _, s := range c.Sources {
		if s = strings.TrimSpace(s); s != "" {
			sources = appendUnique(sources, s)
		}
	}
	return Profile{Files: collect(acc), Sources: sources}, nil
}

func parseJacoco(text string) (Profile, error) {
	var r jacocoGroup
	if err := decodeXML(text, "report", &r); err != nil {
		return Profile{}, err
	}
	acc := map[blockKey]Block{}
	if err := r.addTo(acc); err != nil {
		return Profile{}, err
	}
	return Profile{Files: collect(acc)}, nil
}

// addTo adds the lines of g's packages and nested groups. A line counts
// as covered when it has covered instructions; lines with none are skipped.
func (g jacocoGroup) addTo(acc map[blockKey]Block) error {
	for _, sub := range g.Groups {
		if err := sub.addTo(acc); err != nil {
			return err
		}
	}
	for _, pkg := range g.Packages {
		dir := strings.Trim(strings.TrimSpace(pkg.Name), "/")
		for _, sf := range pkg.Sourcefiles {
			name := strings.TrimSpace(sf.Name)
			if name == "" {
				continue
			}
			file := name
			if dir != "" {
				file = dir + "/" + name
			}
			for _, l := range sf.Lines {
				ln, err := strconv.Atoi(strings.TrimSpace(l.Nr))
				if err != nil || ln <= 0 {
					return fmt.Errorf("jacoco report: %s: bad line number %q", file, l.Nr)
				}
				mi, err1 := parseCount(orZero(l.MI))
				ci, err2 := parseCount(orZero(l.CI))
				if err1 != nil || err2 != nil {
					return fmt.Errorf("jacoco report: %s:%d: bad instruction counts mi=%q ci=%q", file, ln, l.MI, l.CI)
				}
				if mi == 0 && ci == 0 {
					continue
				}
				add(acc, file, lineBlock(ln, ci))
			}
		}
	}
	return nil
}

func orZero(s string) string {
	if strings.TrimSpace(s) == "" {
		return "0"
	}
	return s
}

func appendUnique(xs []string, s string) []string {
	for _, x := range xs {
		if x == s {
			return xs
		}
	}
	return append(xs, s)
}

// bom strips a UTF-8 byte order mark, which some Windows tools write.
func bom(text string) string { return strings.TrimPrefix(text, "\ufeff") }
