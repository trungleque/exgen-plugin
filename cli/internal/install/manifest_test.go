package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManifestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".exgen", "manifest.json")

	m, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	pl := planWith(t.TempDir(), map[string]string{"a.md": "one", "b.md": "two"})
	m.Record("demo", "pi", pl.Actions)
	if err := m.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Plugin != "demo" {
		t.Errorf("plugin = %q", reloaded.Plugin)
	}
	entries := reloaded.Entries("pi")
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	if entries[0].Digest == "" {
		t.Error("digest not recorded")
	}
}

func TestLoadManifestTreatsMissingFileAsEmpty(t *testing.T) {
	m, err := LoadManifest(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("a missing manifest should not be an error: %v", err)
	}
	if len(m.Entries("pi")) != 0 {
		t.Error("expected no entries")
	}
}

// A corrupt manifest must not wedge the tool.
func TestLoadManifestSurvivesCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("corrupt manifest should be treated as absent: %v", err)
	}
	if len(m.Targets) != 0 {
		t.Error("expected an empty manifest")
	}
}

func TestSaveRemovesManifestWhenNothingIsTracked(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".exgen", "manifest.json")

	m, _ := LoadManifest(path)
	m.Record("demo", "pi", planWith(dir, map[string]string{"a.md": "x"}).Actions)
	if err := m.Save(); err != nil {
		t.Fatal(err)
	}

	m.Forget("pi")
	if err := m.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("an empty manifest should be deleted, not left behind")
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Error("the empty .exgen directory should be pruned too")
	}
}

// The reason the manifest exists: a newer exgen renders the same component
// differently, and that must not be mistaken for a user's local edit.
func TestManifestDistinguishesVersionSkewFromLocalEdits(t *testing.T) {
	dir := t.TempDir()

	installed := planWith(dir, map[string]string{"s.md": "rendered by v1"})
	if _, err := Apply(installed, false); err != nil {
		t.Fatal(err)
	}
	manifest, _ := LoadManifest(filepath.Join(dir, ".exgen", "manifest.json"))
	manifest.Record("demo", "pi", installed.Actions)

	// A later build renders the same component differently.
	rerendered := planWith(dir, map[string]string{"s.md": "rendered by v2"})

	// Plan-derived removal compares against v2 and wrongly cries "modified".
	results, err := Remove(rerendered.Removals(), true, false)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != StatusModified {
		t.Fatalf("precondition failed: expected the naive path to report modified, got %q", results[0].Status)
	}

	// The recorded digest matches what is really on disk, so it removes.
	results, err = Remove(manifest.Entries("pi"), false, false)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != StatusRemoved {
		t.Errorf("status = %q, want removed — the file is untouched since install", results[0].Status)
	}
	if _, err := os.Stat(installed.Actions[0].Path); !os.IsNotExist(err) {
		t.Error("file should have been removed")
	}
}

// A genuine local edit must still be protected.
func TestManifestStillProtectsRealEdits(t *testing.T) {
	dir := t.TempDir()

	pl := planWith(dir, map[string]string{"s.md": "original"})
	if _, err := Apply(pl, false); err != nil {
		t.Fatal(err)
	}
	manifest, _ := LoadManifest(filepath.Join(dir, ".exgen", "manifest.json"))
	manifest.Record("demo", "pi", pl.Actions)

	if err := os.WriteFile(pl.Actions[0].Path, []byte("hand edited"), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := Remove(manifest.Entries("pi"), false, false)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != StatusModified {
		t.Errorf("status = %q, want modified", results[0].Status)
	}
	if _, err := os.Stat(pl.Actions[0].Path); err != nil {
		t.Error("a hand-edited file was deleted")
	}
}
