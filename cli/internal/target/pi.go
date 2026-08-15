package target

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/trungleque/exgen-plugin/cli/internal/install"
	"github.com/trungleque/exgen-plugin/cli/internal/plugin"
)

func init() { register(piTarget{}) }

// piTarget installs into Pi (https://pi.dev).
//
// Skills carry over almost unchanged — Pi's SKILL.md schema is a superset of
// the fields exgen uses. The two translations are slash commands, which Pi
// calls prompt templates and which use $@ rather than $ARGUMENTS, and
// subagents, which Pi only gains through the pi-subagents package.
type piTarget struct{}

func (piTarget) ID() string    { return "pi" }
func (piTarget) Title() string { return "Pi" }
func (piTarget) Docs() string  { return "https://pi.dev/docs/latest/skills" }

const piSubagentsPackage = "@tintinweb/pi-subagents"

// piSkillFields is Pi's documented SKILL.md frontmatter schema. Keys outside
// it are dropped rather than passed through, so a Claude-only key such as
// argument-hint cannot trip Pi's validator.
var piSkillFields = []string{
	"name", "description", "license", "compatibility",
	"metadata", "allowed-tools", "disable-model-invocation",
}

func (t piTarget) root(ctx Context) string {
	return ctx.Base(filepath.Join(".pi", "agent"), ".pi")
}

func (t piTarget) Support(ctx Context) []Support {
	root := t.root(ctx)
	return []Support{
		{plugin.KindSkill, true, Abbrev(filepath.Join(root, "skills", "<name>", "SKILL.md"), ctx.Home), ""},
		{plugin.KindAgent, false, Abbrev(filepath.Join(root, "agents", "<name>.md"), ctx.Home),
			"needs the " + piSubagentsPackage + " package; Claude tool names remapped"},
		{plugin.KindCommand, true, Abbrev(filepath.Join(root, "prompts", "<name>.md"), ctx.Home),
			"installed as a prompt template; $ARGUMENTS rewritten to $@"},
	}
}

func (t piTarget) Plan(p *plugin.Plugin, ctx Context) (*install.Plan, error) {
	root := t.root(ctx)
	pl := &install.Plan{
		Target:      t.ID(),
		TargetTitle: t.Title(),
		Scope:       string(ctx.Scope),
		Roots:       []string{Abbrev(root, ctx.Home)},
	}
	if ctx.Scope == ScopeProject {
		pl.ProjectDir = ctx.Dir
	}

	if ctx.Wants(plugin.KindSkill) {
		for _, s := range p.Skills {
			t.planSkill(pl, root, s)
		}
	}
	if ctx.Wants(plugin.KindAgent) {
		for _, a := range p.Agents {
			t.planAgent(pl, root, a)
		}
	}
	if ctx.Wants(plugin.KindCommand) {
		for _, c := range p.Commands {
			t.planPrompt(pl, root, c.Name, c.Frontmatter, c.Body, plugin.KindCommand, "")
		}
	}
	return pl, nil
}

func (t piTarget) planSkill(pl *install.Plan, root string, s plugin.Skill) {
	dir := filepath.Join(root, "skills", s.Name)
	data, dropped := skillDoc(s, piSkillFields)

	var notes []string
	if note := noteDropped("frontmatter", dropped); note != "" {
		notes = append(notes, note)
	}

	pl.Add(install.Action{
		Path:  filepath.Join(dir, "SKILL.md"),
		Data:  data,
		Kind:  plugin.KindSkill,
		Name:  s.Name,
		Notes: notes,
	})
	for _, f := range s.Files {
		pl.Add(install.Action{
			Path: filepath.Join(dir, filepath.FromSlash(f.RelPath)),
			Data: f.Data,
			Kind: plugin.KindSkill,
			Name: s.Name,
			Role: f.RelPath,
		})
	}

	// A skill Pi hides from the model has no invocation path left, since Pi
	// skills are not slash commands. Give it one as a prompt template.
	if s.Frontmatter.IsTrue("disable-model-invocation") {
		// A project install is checked in and cloned elsewhere, so point at a
		// repo-relative path there; a global install is stable enough to name
		// absolutely.
		ref := filepath.Join(dir, "SKILL.md")
		if rel, err := filepath.Rel(pl.ProjectDir, ref); err == nil && pl.ProjectDir != "" {
			ref = rel
		}
		body := fmt.Sprintf(
			"Read and follow the `%s` skill at `%s`.\n\nApply it to the following input:\n\n$@\n",
			s.Name, ref,
		)
		fm := plugin.Frontmatter{}
		fm.Set("description", firstSentence(s.Description))
		if hint, ok := s.Frontmatter.Get("argument-hint"); ok && hint != "" {
			fm.Set("argument-hint", hint)
		}
		pl.Add(install.Action{
			Path:      filepath.Join(root, "prompts", s.Name+".md"),
			Data:      plugin.Document(fm, body),
			Kind:      plugin.KindSkill,
			Name:      s.Name,
			Role:      "prompt template",
			Transform: "explicit-invocation shim",
			Notes: []string{
				"disable-model-invocation hides the skill from Pi's model, so /" + s.Name +
					" is added as the human invocation path",
			},
		})
	}
}

func (t piTarget) planAgent(pl *install.Plan, root string, a plugin.Agent) {
	fm := a.Frontmatter.Clone()
	var notes []string

	if raw, ok := fm.Get("tools"); ok {
		kept, dropped := mapTools(raw, piTools)
		if len(kept) == 0 {
			fm.Delete("tools")
			notes = append(notes, "no Claude tool mapped to a Pi tool; allowlist removed so the agent keeps defaults")
		} else {
			fm.Set("tools", strings.Join(kept, ", "))
			notes = append(notes, fmt.Sprintf("tools remapped to %s", strings.Join(kept, ", ")))
		}
		if note := noteDropped("tools", dropped); note != "" {
			notes = append(notes, note)
		}
	}
	if model := dropClaudeModel(&fm); model != "" {
		notes = append(notes, fmt.Sprintf("model %q is a Claude Code alias and does not resolve in Pi; removed so Pi's default applies", model))
	}

	// The intersection of the two pi-subagents forks' schemas.
	dropped := fm.Retain("name", "description", "tools", "color")
	if note := noteDropped("frontmatter", dropped); note != "" {
		notes = append(notes, note)
	}
	fm.Set("name", a.Name)

	pl.Add(install.Action{
		Path:      filepath.Join(root, "agents", a.Name+".md"),
		Data:      plugin.Document(fm, a.Body),
		Kind:      plugin.KindAgent,
		Name:      a.Name,
		Transform: "subagent via package",
		Notes:     notes,
	})
	pl.Note("Pi has no built-in subagents — install the package: pi install npm:" + piSubagentsPackage)
}

func (t piTarget) planPrompt(pl *install.Plan, root, name string, src plugin.Frontmatter, body string, kind plugin.Kind, role string) {
	fm := src.Clone()
	fm.Retain("description", "argument-hint")

	body, rewritten := toPiArguments(body)
	var notes []string
	transform := ""
	if rewritten {
		transform = "$ARGUMENTS → $@"
		notes = append(notes, "$ARGUMENTS rewritten to Pi's $@")
	}

	flat := strings.ReplaceAll(name, "/", "-")
	if flat != name {
		notes = append(notes, fmt.Sprintf("Pi does not namespace prompts; %s flattened to %s", name, flat))
	}

	pl.Add(install.Action{
		Path:      filepath.Join(root, "prompts", flat+".md"),
		Data:      plugin.Document(fm, body),
		Kind:      kind,
		Name:      name,
		Role:      role,
		Transform: transform,
		Notes:     notes,
	})
}

// firstSentence trims a long skill description down to something that reads
// well in a slash-command picker.
func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if idx := strings.Index(s, ". "); idx > 0 {
		return s[:idx+1]
	}
	return s
}
