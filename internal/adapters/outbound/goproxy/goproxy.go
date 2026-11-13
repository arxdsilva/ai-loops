// Package goproxy reads the latest released version of a module from the Go
// module proxy (GET <proxy>/<escaped module>/@latest).
package goproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
	"unicode"
)

const (
	defaultProxy = "https://proxy.golang.org"
	timeout      = 1500 * time.Millisecond
)

// proxyClient implements updatecheck.Source.
type proxyClient struct {
	module string
	getenv func(string) string
	http   *http.Client
}

// New returns a proxyClient for module.
func New(module string) *proxyClient {
	return &proxyClient{module: module, getenv: os.Getenv, http: &http.Client{}}
}

// Latest returns the latest version. It honours GOPROXY (first http(s)
// entry; "off"/"direct"-only → error), skips modules matched by
// GOPRIVATE/GONOPROXY (error), escapes upper-case path letters (!x), and
// times out after 1500ms.
func (c *proxyClient) Latest(ctx context.Context) (string, error) {
	if c.private() {
		return "", errors.New("goproxy: private module")
	}
	base, err := c.proxy()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/"+escape(c.module)+"/@latest", nil)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("goproxy: %s", resp.Status)
	}
	var info struct{ Version string }
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&info); err != nil {
		return "", fmt.Errorf("goproxy: bad response: %w", err)
	}
	if info.Version == "" {
		return "", errors.New("goproxy: empty version")
	}
	return info.Version, nil
}

func (c *proxyClient) proxy() (string, error) {
	v := strings.TrimSpace(c.getenv("GOPROXY"))
	if v == "" {
		return defaultProxy, nil
	}
	for _, e := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == '|' }) {
		e = strings.TrimSpace(e)
		if strings.HasPrefix(e, "http://") || strings.HasPrefix(e, "https://") {
			return strings.TrimRight(e, "/"), nil
		}
	}
	return "", errors.New("goproxy: GOPROXY has no http(s) proxy")
}

// private reports whether GOPRIVATE or GONOPROXY covers the module, with
// the go command's prefix semantics.
func (c *proxyClient) private() bool {
	for _, env := range []string{"GOPRIVATE", "GONOPROXY"} {
		for _, pat := range strings.Split(c.getenv(env), ",") {
			pat = strings.TrimSpace(pat)
			if pat == "" {
				continue
			}
			n := strings.Count(pat, "/") + 1
			parts := strings.Split(c.module, "/")
			if len(parts) < n {
				continue
			}
			if ok, _ := path.Match(pat, strings.Join(parts[:n], "/")); ok {
				return true
			}
		}
	}
	return false
}

// escape applies the module-proxy case encoding: upper-case letters become
// "!" plus the lower-case letter.
func escape(p string) string {
	var b strings.Builder
	for _, r := range p {
		if unicode.IsUpper(r) {
			b.WriteByte('!')
			r = unicode.ToLower(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}
