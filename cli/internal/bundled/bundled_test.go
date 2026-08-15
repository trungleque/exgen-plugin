package bundled

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// sourcePlugin is the plugin this package mirrors. The mirror exists only
// because go:embed cannot reach outside the cli/ module.
const sourcePlugin = "../../../plugins/exgen"

// TestBundledMatchesSource is the guard that makes the mirror safe: editing a
// skill without running `make sync` fails here rather than shipping a binary
// that installs stale instructions.
func TestBundledMatchesSource(t *testing.T) {
	if _, err := os.Stat(sourcePlugin); err != nil {
		t.Skip("source plugin not present — running outside the repository")
	}

	embeddedFS, err := FS()
	if err != nil {
		t.Fatal(err)
	}

	sourceFiles := map[string][]byte{}
	err = filepath.WalkDir(sourcePlugin, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if d.Name() == ".DS_Store" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(sourcePlugin, path)
		if err != nil {
			return err
		}
		sourceFiles[filepath.ToSlash(rel)] = data
		return nil
	})
	if err != nil {
		t.Fatalf("walk source: %v", err)
	}
	if len(sourceFiles) == 0 {
		t.Fatal("source plugin has no files")
	}

	seen := map[string]bool{}
	err = fs.WalkDir(embeddedFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if d.Name() == ".DS_Store" {
			return nil
		}
		seen[path] = true
		want, ok := sourceFiles[path]
		if !ok {
			t.Errorf("embedded copy has %s, which is not in %s — run `make sync`", path, sourcePlugin)
			return nil
		}
		got, err := fs.ReadFile(embeddedFS, path)
		if err != nil {
			return err
		}
		if !bytes.Equal(got, want) {
			t.Errorf("embedded copy of %s is stale — run `make sync`", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded: %v", err)
	}

	for path := range sourceFiles {
		if !seen[path] {
			t.Errorf("%s is missing from the embedded copy — run `make sync`", path)
		}
	}
}

// The manifest lives in a dot-directory, which a plain go:embed pattern skips.
func TestEmbeddedManifestIsPresent(t *testing.T) {
	embeddedFS, err := FS()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.ReadFile(embeddedFS, ".claude-plugin/plugin.json"); err != nil {
		t.Fatalf("embed pattern dropped the dot-directory manifest: %v", err)
	}
}

func TestEmbeddedPluginParses(t *testing.T) {
	p, err := Plugin()
	if err != nil {
		t.Fatal(err)
	}
	if p.Name == "" || p.Version == "" {
		t.Errorf("manifest fields missing: %+v", p)
	}
	if len(p.Skills) == 0 || len(p.Agents) == 0 {
		t.Errorf("expected skills and agents, got %d and %d", len(p.Skills), len(p.Agents))
	}
	for _, s := range p.Skills {
		if s.Description == "" {
			t.Errorf("skill %s has no description — it would never trigger", s.Name)
		}
	}
}
