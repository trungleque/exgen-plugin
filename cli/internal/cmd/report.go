package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/trungleque/exgen-plugin/cli/internal/install"
	"github.com/trungleque/exgen-plugin/cli/internal/plugin"
	"github.com/trungleque/exgen-plugin/cli/internal/target"
)

const (
	markOK      = "+"
	markSame    = "="
	markChanged = "~"
	markWarn    = "!"
	markGone    = "-"
	markSkip    = "x"
)

// tally counts statuses across a run so the closing line can be honest about
// what actually changed.
type tally struct {
	created, updated, unchanged, removed, absent, modified, degraded int
}

func (t *tally) add(status install.Status) {
	switch status {
	case install.StatusCreated:
		t.created++
	case install.StatusUpdated:
		t.updated++
	case install.StatusUnchanged:
		t.unchanged++
	case install.StatusRemoved:
		t.removed++
	case install.StatusAbsent:
		t.absent++
	case install.StatusModified:
		t.modified++
	}
}

func statusMark(s install.Status) string {
	switch s {
	case install.StatusCreated:
		return markOK
	case install.StatusUpdated:
		return markChanged
	case install.StatusUnchanged:
		return markSame
	case install.StatusRemoved:
		return markGone
	case install.StatusModified:
		return markWarn
	default:
		return markSkip
	}
}

// reportPlan prints one target's results, grouped by component kind, with each
// translation note attached to the component it applies to.
func reportPlan(w io.Writer, pl *install.Plan, results []install.Result, home string, t *tally) {
	fmt.Fprintf(w, "\n%s  (%s)\n", pl.TargetTitle, pl.Scope)
	for _, root := range pl.Roots {
		fmt.Fprintf(w, "  %s\n", root)
	}

	byKind := map[plugin.Kind][]install.Result{}
	for _, r := range results {
		byKind[r.Action.Kind] = append(byKind[r.Action.Kind], r)
	}

	for _, kind := range reportKinds {
		group := byKind[kind]
		if len(group) == 0 {
			continue
		}
		fmt.Fprintf(w, "\n  %s\n", kindLabel(kind))
		for _, r := range group {
			t.add(r.Status)
			a := r.Action

			// Bundled assets are rolled into their component's line rather
			// than listed one by one.
			if a.Role != "" && a.Role != "prompt template" {
				continue
			}

			label := a.Name
			if a.Role != "" {
				label = fmt.Sprintf("%s (%s)", a.Name, a.Role)
			}
			suffix := ""
			if extra := countAssets(group, a.Name); extra > 0 && a.Role == "" {
				suffix = fmt.Sprintf("  +%d file(s)", extra)
			}
			if a.Transform != "" {
				suffix += "  [" + a.Transform + "]"
			}

			fmt.Fprintf(w, "    %s %-28s %s%s\n", statusMark(r.Status), label, target.Abbrev(a.Path, home), suffix)

			for _, note := range a.Notes {
				t.degraded++
				fmt.Fprintf(w, "      %s %s\n", markWarn, note)
			}
		}
	}

	if len(pl.Skips) > 0 {
		fmt.Fprintf(w, "\n  not installed\n")
		for _, s := range pl.Skips {
			fmt.Fprintf(w, "    %s %-28s %s\n", markSkip, s.Name, s.Reason)
		}
	}

	if len(pl.Notes) > 0 {
		fmt.Fprintf(w, "\n  notes\n")
		for _, n := range pl.Notes {
			fmt.Fprintf(w, "    - %s\n", n)
		}
	}
}

// reportKinds is the display order, with target-generated files first so the
// section reads as "here is the container, here is what went in it".
var reportKinds = []plugin.Kind{
	plugin.KindManifest, plugin.KindSkill, plugin.KindAgent, plugin.KindCommand,
}

func kindLabel(k plugin.Kind) string {
	if k == plugin.KindManifest {
		return "manifest"
	}
	return string(k) + "s"
}

// countAssets counts the bundled files that travelled with a component.
func countAssets(group []install.Result, name string) int {
	n := 0
	for _, r := range group {
		if r.Action.Name == name && r.Action.Role != "" && r.Action.Role != "prompt template" {
			n++
		}
	}
	return n
}

func (t tally) summary(dryRun bool) string {
	var parts []string
	add := func(n int, label string) {
		if n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, label))
		}
	}
	add(t.created, "created")
	add(t.updated, "updated")
	add(t.unchanged, "unchanged")
	add(t.removed, "removed")
	add(t.absent, "already absent")
	add(t.modified, "locally modified, kept")
	add(t.degraded, "translation note(s)")
	if len(parts) == 0 {
		return "nothing to do"
	}
	line := strings.Join(parts, ", ")
	if dryRun {
		return line + "  (dry run — nothing written)"
	}
	return line
}

// describeSupport renders the per-target capability table used by `list` and
// `targets`.
func describeSupport(w io.Writer, tg target.Target, ctx target.Context, counts map[plugin.Kind]int) {
	fmt.Fprintf(w, "\n%s  (%s)\n  %s\n", tg.Title(), tg.ID(), tg.Docs())
	for _, s := range tg.Support(ctx) {
		native := "native"
		if !s.Native {
			native = "translated"
		}
		count := ""
		if counts != nil {
			count = fmt.Sprintf(" x%d", counts[s.Kind])
		}
		fmt.Fprintf(w, "    %-9s %-11s%-4s %s\n", s.Kind+"s", native, count, s.Location)
		if s.Note != "" {
			fmt.Fprintf(w, "      %s %s\n", markWarn, s.Note)
		}
	}
}
