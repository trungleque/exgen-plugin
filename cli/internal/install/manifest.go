package install

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/trungleque/exgen-plugin/cli/internal/plugin"
)

// ManifestVersion is the on-disk schema version.
const ManifestVersion = 1

// Entry records one installed file.
//
// The digest is what makes uninstall trustworthy: comparing a file against the
// bytes *this* build would write mistakes a version upgrade for a local edit,
// because a newer exgen legitimately renders the same component differently.
// Comparing against the digest recorded at install time does not.
type Entry struct {
	Path   string      `json:"path"`
	Digest string      `json:"digest"`
	Kind   plugin.Kind `json:"kind"`
	Name   string      `json:"name"`
	Role   string      `json:"role,omitempty"`
}

// Manifest tracks what a plugin installed, per target.
type Manifest struct {
	Version int                `json:"version"`
	Plugin  string             `json:"plugin"`
	Targets map[string][]Entry `json:"targets"`

	path string
}

// Digest returns the hex sha256 of data.
func Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// LoadManifest reads a manifest, returning an empty one when absent so callers
// need not distinguish "first install" from "no manifest yet".
func LoadManifest(path string) (*Manifest, error) {
	m := &Manifest{Version: ManifestVersion, Targets: map[string][]Entry{}, path: path}

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, m); err != nil {
		// A corrupt manifest must not block an install; treat it as absent.
		return &Manifest{Version: ManifestVersion, Targets: map[string][]Entry{}, path: path}, nil
	}
	if m.Targets == nil {
		m.Targets = map[string][]Entry{}
	}
	m.path = path
	return m, nil
}

// Record replaces the entries for one target with what a plan just wrote.
func (m *Manifest) Record(pluginName, targetID string, actions []Action) {
	m.Plugin = pluginName
	entries := make([]Entry, 0, len(actions))
	for _, a := range actions {
		entries = append(entries, Entry{
			Path:   a.Path,
			Digest: Digest(a.Data),
			Kind:   a.Kind,
			Name:   a.Name,
			Role:   a.Role,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	m.Targets[targetID] = entries
}

// Forget drops a target's entries after an uninstall.
func (m *Manifest) Forget(targetID string) { delete(m.Targets, targetID) }

// Entries returns what was recorded for a target.
func (m *Manifest) Entries(targetID string) []Entry { return m.Targets[targetID] }

// Save writes the manifest, or removes it once nothing is left to track.
func (m *Manifest) Save() error {
	if m.path == "" {
		return nil
	}
	if len(m.Targets) == 0 {
		err := os.Remove(m.path)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		// Leave no empty .exgen/ behind.
		os.Remove(filepath.Dir(m.path))
		return nil
	}

	m.Version = ManifestVersion
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(m.path, append(data, '\n'), 0o644)
}
