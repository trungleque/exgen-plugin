// Package install describes and applies the file writes a target needs,
// keeping the "what would happen" and "make it happen" halves separate so
// --dry-run and uninstall can reuse exactly the same computation.
package install

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/trungleque/exgen-plugin/cli/internal/plugin"
)

// Action is one file the target wants on disk.
type Action struct {
	Path      string      // absolute destination
	Data      []byte      // exact bytes to write
	Kind      plugin.Kind // component category this file belongs to
	Name      string      // component name, for reporting
	Role      string      // "" for the component's own file, else e.g. "reference", "prompt template"
	Notes     []string    // per-component degradation notes
	Transform string      // short label when the file is not a verbatim copy
}

// Skip records a component a target cannot represent at all.
type Skip struct {
	Kind   plugin.Kind
	Name   string
	Reason string
}

// Plan is everything a single target install would do.
type Plan struct {
	Target      string
	TargetTitle string
	Scope       string
	Roots       []string // destination roots, for the header line
	Actions     []Action
	Skips       []Skip
	Notes       []string // target-level advice, e.g. a package to install

	// ProjectDir is set only for a repository-scoped install, and marks the
	// directory that generated cross-references should be relative to so the
	// result survives being committed and cloned elsewhere.
	ProjectDir string
}

// Add appends an action.
func (p *Plan) Add(a Action) { p.Actions = append(p.Actions, a) }

// Skip records an unrepresentable component.
func (p *Plan) Skip(kind plugin.Kind, name, reason string) {
	p.Skips = append(p.Skips, Skip{Kind: kind, Name: name, Reason: reason})
}

// Note records target-level advice, ignoring duplicates.
func (p *Plan) Note(note string) {
	for _, existing := range p.Notes {
		if existing == note {
			return
		}
	}
	p.Notes = append(p.Notes, note)
}

// Status is the outcome of applying one action.
type Status string

const (
	StatusCreated   Status = "created"
	StatusUpdated   Status = "updated"
	StatusUnchanged Status = "unchanged"
	StatusRemoved   Status = "removed"
	StatusAbsent    Status = "absent"
	StatusModified  Status = "modified" // uninstall found local edits and left them alone
)

// Result pairs an action with what happened to it.
type Result struct {
	Action Action
	Status Status
}

// Apply writes every action. With dryRun set, nothing touches the disk but the
// statuses still reflect what would change.
func Apply(p *Plan, dryRun bool) ([]Result, error) {
	results := make([]Result, 0, len(p.Actions))
	for _, a := range p.Actions {
		status, err := applyOne(a, dryRun)
		if err != nil {
			return results, err
		}
		results = append(results, Result{Action: a, Status: status})
	}
	return results, nil
}

func applyOne(a Action, dryRun bool) (Status, error) {
	existing, err := os.ReadFile(a.Path)
	switch {
	case err == nil && bytes.Equal(existing, a.Data):
		return StatusUnchanged, nil
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return "", err
	}

	status := StatusUpdated
	if errors.Is(err, fs.ErrNotExist) {
		status = StatusCreated
	}
	if dryRun {
		return status, nil
	}
	if err := os.MkdirAll(filepath.Dir(a.Path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(a.Path, a.Data, 0o644); err != nil {
		return "", err
	}
	return status, nil
}

// Removals derives removal candidates from a plan, for the case where no
// install manifest exists — an install from before manifests, or one whose
// manifest was deleted. Digests come from what this build would write, which
// is only correct when the installed version matches; that is exactly the
// weakness the manifest exists to remove.
func (p *Plan) Removals() []Entry {
	out := make([]Entry, 0, len(p.Actions))
	for _, a := range p.Actions {
		out = append(out, Entry{
			Path:   a.Path,
			Digest: Digest(a.Data),
			Kind:   a.Kind,
			Name:   a.Name,
			Role:   a.Role,
		})
	}
	return out
}

// Remove deletes recorded files. A file whose current contents do not match
// the digest recorded when it was installed is assumed to be hand-edited and
// is left alone unless force is set — an uninstall must never eat someone's
// changes.
func Remove(entries []Entry, dryRun, force bool) ([]Result, error) {
	results := make([]Result, 0, len(entries))
	dirs := map[string]bool{}

	for _, e := range entries {
		action := Action{Path: e.Path, Kind: e.Kind, Name: e.Name, Role: e.Role}

		existing, err := os.ReadFile(e.Path)
		if errors.Is(err, fs.ErrNotExist) {
			results = append(results, Result{Action: action, Status: StatusAbsent})
			continue
		}
		if err != nil {
			return results, err
		}
		if e.Digest != "" && Digest(existing) != e.Digest && !force {
			results = append(results, Result{Action: action, Status: StatusModified})
			continue
		}
		if !dryRun {
			if err := os.Remove(e.Path); err != nil {
				return results, err
			}
		}
		dirs[filepath.Dir(e.Path)] = true
		results = append(results, Result{Action: action, Status: StatusRemoved})
	}

	if !dryRun {
		pruneEmpty(dirs)
	}
	return results, nil
}

// pruneEmpty removes directories left empty by an uninstall, deepest first so
// that skills/<name>/references/ collapses along with skills/<name>/.
func pruneEmpty(dirs map[string]bool) {
	ordered := make([]string, 0, len(dirs))
	for d := range dirs {
		ordered = append(ordered, d)
	}
	sort.Slice(ordered, func(i, j int) bool { return len(ordered[i]) > len(ordered[j]) })
	for _, d := range ordered {
		for range 4 { // walk up a few levels at most; os.Remove fails on non-empty
			if err := os.Remove(d); err != nil {
				break
			}
			d = filepath.Dir(d)
		}
	}
}
