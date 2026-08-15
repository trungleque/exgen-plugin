package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/trungleque/exgen-plugin/cli/internal/plugin"
)

func planWith(dir string, files map[string]string) *Plan {
	pl := &Plan{Target: "test", TargetTitle: "Test", Scope: "global"}
	for rel, content := range files {
		pl.Add(Action{
			Path: filepath.Join(dir, filepath.FromSlash(rel)),
			Data: []byte(content),
			Kind: plugin.KindSkill,
			Name: rel,
		})
	}
	return pl
}

func TestApplyCreatesThenReportsUnchanged(t *testing.T) {
	dir := t.TempDir()
	pl := planWith(dir, map[string]string{"skills/a/SKILL.md": "one"})

	results, err := Apply(pl, false)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != StatusCreated {
		t.Errorf("status = %q, want created", results[0].Status)
	}
	if data, err := os.ReadFile(pl.Actions[0].Path); err != nil || string(data) != "one" {
		t.Fatalf("file not written: %v %q", err, data)
	}

	results, err = Apply(pl, false)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != StatusUnchanged {
		t.Errorf("second run status = %q, want unchanged", results[0].Status)
	}
}

func TestApplyUpdatesChangedFile(t *testing.T) {
	dir := t.TempDir()
	first := planWith(dir, map[string]string{"s.md": "v1"})
	if _, err := Apply(first, false); err != nil {
		t.Fatal(err)
	}

	second := planWith(dir, map[string]string{"s.md": "v2"})
	results, err := Apply(second, false)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != StatusUpdated {
		t.Errorf("status = %q, want updated", results[0].Status)
	}
	data, _ := os.ReadFile(second.Actions[0].Path)
	if string(data) != "v2" {
		t.Errorf("content = %q, want v2", data)
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	dir := t.TempDir()
	pl := planWith(dir, map[string]string{"deep/nested/s.md": "x"})

	results, err := Apply(pl, true)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != StatusCreated {
		t.Errorf("status = %q, want the status it would have had", results[0].Status)
	}
	if _, err := os.Stat(pl.Actions[0].Path); !os.IsNotExist(err) {
		t.Error("dry run touched the disk")
	}
}

func TestRemoveDeletesInstalledFilesAndPrunesDirs(t *testing.T) {
	dir := t.TempDir()
	pl := planWith(dir, map[string]string{"skills/a/SKILL.md": "one", "skills/a/references/n.md": "two"})
	if _, err := Apply(pl, false); err != nil {
		t.Fatal(err)
	}

	results, err := Remove(pl.Removals(), false, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Status != StatusRemoved {
			t.Errorf("%s status = %q, want removed", r.Action.Path, r.Status)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "skills", "a")); !os.IsNotExist(err) {
		t.Error("empty skill directory should have been pruned")
	}
}

// An uninstall must not silently eat someone's edits.
func TestRemoveKeepsLocallyModifiedFiles(t *testing.T) {
	dir := t.TempDir()
	pl := planWith(dir, map[string]string{"s.md": "original"})
	if _, err := Apply(pl, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pl.Actions[0].Path, []byte("hand edited"), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := Remove(pl.Removals(), false, false)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != StatusModified {
		t.Errorf("status = %q, want modified", results[0].Status)
	}
	if _, err := os.Stat(pl.Actions[0].Path); err != nil {
		t.Error("a locally modified file was deleted without --force")
	}

	results, err = Remove(pl.Removals(), false, true)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != StatusRemoved {
		t.Errorf("--force status = %q, want removed", results[0].Status)
	}
	if _, err := os.Stat(pl.Actions[0].Path); !os.IsNotExist(err) {
		t.Error("--force should have removed the file")
	}
}

func TestRemoveReportsAbsentFiles(t *testing.T) {
	dir := t.TempDir()
	pl := planWith(dir, map[string]string{"never-installed.md": "x"})

	results, err := Remove(pl.Removals(), false, false)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != StatusAbsent {
		t.Errorf("status = %q, want absent", results[0].Status)
	}
}

func TestNoteDeduplicates(t *testing.T) {
	pl := &Plan{}
	pl.Note("install the package")
	pl.Note("install the package")
	if len(pl.Notes) != 1 {
		t.Errorf("notes = %v, want one entry", pl.Notes)
	}
}
