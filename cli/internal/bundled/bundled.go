// Package bundled carries a build-time copy of the exgen plugin so that
// `exgen install <target>` works from a bare binary, with no checkout.
//
// The copy exists because go:embed cannot reach outside its own module: the
// module root is cli/, and the plugin lives at ../plugins/exgen. `make sync`
// refreshes it, and TestBundledMatchesSource fails the build if the two drift.
package bundled

import (
	"embed"
	"io/fs"

	"github.com/trungleque/exgen-plugin/cli/internal/plugin"
)

//go:generate make -C ../.. sync

// all: is required — the plugin's manifest lives in .claude-plugin/, and a
// plain embed pattern silently skips dot-prefixed directories.
//
//go:embed all:exgen
var embedded embed.FS

// FS returns the embedded plugin directory, rooted at the plugin itself.
func FS() (fs.FS, error) { return fs.Sub(embedded, "exgen") }

// Version reports the embedded plugin's version, or "" if it cannot be read.
//
// This is the fallback for `go install`, which cannot pass the -ldflags the
// Makefile uses; the binary would otherwise report itself as "dev" even though
// the toolkit inside it is a specific release.
func Version() string {
	p, err := Plugin()
	if err != nil {
		return ""
	}
	return p.Version
}

// Plugin parses the embedded plugin.
func Plugin() (*plugin.Plugin, error) {
	sub, err := FS()
	if err != nil {
		return nil, err
	}
	p, err := plugin.Load(sub)
	if err != nil {
		return nil, err
	}
	p.Source = "embedded"
	return p, nil
}
