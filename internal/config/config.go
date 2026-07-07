/*
Copyright (c) ILM.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

// Package config models the on-disk ilmctl context file. In Phase 1 the file is a
// RESERVED format for the future ILM Core layer — it is read-only and is NOT a
// cluster selector (clusters come from the kubeconfig). No writes happen in P1.
package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
	"sigs.k8s.io/yaml"
)

const (
	apiVersion = "ilm.omnitrust.com/v1alpha1"
	kind       = "Config"
)

// Context is the ilmctl context-file model (kubeconfig-parallel; reserved for Core).
type Context struct {
	APIVersion     string         `json:"apiVersion"`
	Kind           string         `json:"kind"`
	CurrentContext string         `json:"current-context,omitempty"`
	Contexts       []NamedContext `json:"contexts,omitempty"`
}

// NamedContext binds a name to an InstanceContext.
type NamedContext struct {
	Name    string          `json:"name"`
	Context InstanceContext `json:"context"`
}

// InstanceContext selects an ILM instance and its auth parameters (Core layer; unused in P1).
type InstanceContext struct {
	Server   string `json:"server,omitempty"`
	Issuer   string `json:"issuer,omitempty"`
	ClientID string `json:"clientID,omitempty"`
	EdgeMTLS string `json:"edgeMTLS,omitempty"`
}

// Loader resolves and reads the context file. ExplicitPath wins when set.
type Loader struct {
	ExplicitPath string
}

// DefaultPath returns $XDG_CONFIG_HOME/ilm/config, falling back to ~/.config/ilm/config.
func DefaultPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "ilm", "config")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".config", "ilm", "config")
	}
	return filepath.Join(home, ".config", "ilm", "config")
}

// resolvePath implements precedence: --ilmconfig > $ILMCONFIG (first entry) > default.
func (l *Loader) resolvePath() string {
	if l.ExplicitPath != "" {
		return l.ExplicitPath
	}
	if env := os.Getenv("ILMCONFIG"); env != "" {
		// $ILMCONFIG is an OS-path-list, like $KUBECONFIG; P1 honours the first entry only.
		if first := strings.SplitN(env, string(os.PathListSeparator), 2)[0]; first != "" {
			return first
		}
	}
	return DefaultPath()
}

// Load reads the resolved context file. An absent file yields an empty (but valid)
// Context and no error — P1 never requires the file to exist.
// Load is read-only: it never writes to disk.
func (l *Loader) Load() (*Context, error) {
	c := &Context{APIVersion: apiVersion, Kind: kind}
	path := l.resolvePath()
	raw, err := os.ReadFile(path) //nolint:gosec // path is user-supplied config, read-only
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(raw, c); err != nil {
		return nil, err
	}
	if c.APIVersion == "" {
		c.APIVersion = apiVersion
	}
	if c.Kind == "" {
		c.Kind = kind
	}
	return c, nil
}

// AddFlags registers the --ilmconfig flag against the loader.
func AddFlags(fs *pflag.FlagSet, l *Loader) {
	fs.StringVar(&l.ExplicitPath, "ilmconfig", l.ExplicitPath,
		"Path to the ilmctl context file (reserved for the ILM Core layer; overrides $ILMCONFIG).")
}
