package target

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/trungleque/exgen-plugin/cli/internal/install"
	"github.com/trungleque/exgen-plugin/cli/internal/plugin"
)

func init() { register(antigravityTarget{}) }

// antigravityTarget installs into the Antigravity CLI.
//
// Antigravity has the closest thing to Claude Code's plugin format, so a
// global install writes a real plugin directory — manifest included — and a
// project install drops loose components into .agents/. Skills double as slash
// commands there, which is why commands are folded into skills.
type antigravityTarget struct{}

func (antigravityTarget) ID() string    { return "antigravity" }
func (antigravityTarget) Title() string { return "Antigravity CLI" }
func (antigravityTarget) Docs() string  { return "https://antigravity.google/docs/cli/plugins" }

// antigravitySkillFields is the documented skill frontmatter.
var antigravitySkillFields = []string{"name", "description"}

func (t antigravityTarget) pluginRoot(ctx Context, name string) string {
	return filepath.Join(ctx.Home, ".gemini", "antigravity-cli", "plugins", name)
}

func (t antigravityTarget) Support(ctx Context) []Support {
	if ctx.Scope == ScopeProject {
		return []Support{
			{plugin.KindSkill, true, filepath.Join(".agents", "skills", "<name>.md"), ""},
			{plugin.KindAgent, true, filepath.Join(".agents", "agents", "<name>.md"), "gains subagent: true"},
			{plugin.KindCommand, false, filepath.Join(".agents", "skills", "<name>.md"),
				"installed as a skill — Antigravity exposes skills as slash commands"},
		}
	}
	root := Abbrev(t.pluginRoot(ctx, "<plugin>"), ctx.Home)
	return []Support{
		{plugin.KindSkill, true, filepath.Join(root, "skills", "<name>", "SKILL.md"), ""},
		{plugin.KindAgent, true, filepath.Join(root, "agents", "<name>.md"), "gains subagent: true"},
		{plugin.KindCommand, false, filepath.Join(root, "skills", "<name>", "SKILL.md"),
			"installed as a skill — Antigravity exposes skills as slash commands"},
	}
}

func (t antigravityTarget) Plan(p *plugin.Plugin, ctx Context) (*install.Plan, error) {
	pl := &install.Plan{
		Target:      t.ID(),
		TargetTitle: t.Title(),
		Scope:       string(ctx.Scope),
	}

	global := ctx.Scope != ScopeProject
	var skillRoot, agentRoot string
	if global {
		root := t.pluginRoot(ctx, p.Name)
		skillRoot = filepath.Join(root, "skills")
		agentRoot = filepath.Join(root, "agents")
		pl.Roots = []string{Abbrev(root, ctx.Home)}

		manifest, err := antigravityManifest(p)
		if err != nil {
			return nil, err
		}
		pl.Add(install.Action{
			Path:      filepath.Join(root, "plugin.json"),
			Data:      manifest,
			Kind:      plugin.KindManifest,
			Name:      p.Name,
			Transform: "generated",
		})
	} else {
		skillRoot = filepath.Join(ctx.Dir, ".agents", "skills")
		agentRoot = filepath.Join(ctx.Dir, ".agents", "agents")
		pl.Roots = []string{Abbrev(filepath.Join(ctx.Dir, ".agents"), ctx.Home)}
		pl.Note(".agents/ is shared with Codex and Pi — installing those targets at project scope writes to the same skills directory")
	}

	if ctx.Wants(plugin.KindSkill) {
		for _, s := range p.Skills {
			t.planSkill(pl, skillRoot, s, global, p.Skills, plugin.KindSkill, "")
		}
	}
	if ctx.Wants(plugin.KindCommand) {
		for _, c := range p.Commands {
			t.planSkill(pl, skillRoot, commandAsSkill(c), global, p.Skills, plugin.KindCommand, "command as skill")
		}
	}
	if ctx.Wants(plugin.KindAgent) {
		for _, a := range p.Agents {
			t.planAgent(pl, agentRoot, a)
		}
	}
	return pl, nil
}

// planSkill writes a skill in the layout the active scope calls for: the
// directory form inside a plugin, and Antigravity's documented flat
// <name>.md at project scope.
//
// Flat layout moves the skill file up one level relative to its bundled
// assets, so any relative link in the body has to move with it — otherwise
// every reference a skill makes to its own references/ directory dangles.
func (t antigravityTarget) planSkill(pl *install.Plan, root string, s plugin.Skill, useDir bool, siblings []plugin.Skill, kind plugin.Kind, transform string) {
	notes := []string{}

	if !useDir {
		if rewritten, changed := flattenLinks(s.Body, s.Name, s.Files, siblings); changed {
			s.Body = rewritten
			notes = append(notes,
				fmt.Sprintf("relative links rewritten for the flat layout — bundled files live under %s/", s.Name))
		}
	}

	data, dropped := skillDoc(s, antigravitySkillFields)
	if note := noteDropped("frontmatter", dropped); note != "" {
		notes = append(notes, note)
	}
	if transform != "" {
		notes = append(notes, "Antigravity has no separate command concept; skills are invoked as /"+s.Name)
	}
	if s.Frontmatter.IsTrue("disable-model-invocation") {
		notes = append(notes, "Antigravity has no equivalent of disable-model-invocation — this skill can be selected by the model, not only by /"+s.Name)
	}

	path := filepath.Join(root, s.Name+".md")
	if useDir {
		path = filepath.Join(root, s.Name, "SKILL.md")
	}

	pl.Add(install.Action{
		Path:      path,
		Data:      data,
		Kind:      kind,
		Name:      s.Name,
		Transform: transform,
		Notes:     notes,
	})
	for _, f := range s.Files {
		pl.Add(install.Action{
			Path: filepath.Join(root, s.Name, filepath.FromSlash(f.RelPath)),
			Data: f.Data,
			Kind: kind,
			Name: s.Name,
			Role: f.RelPath,
		})
	}
}

func (t antigravityTarget) planAgent(pl *install.Plan, root string, a plugin.Agent) {
	fm := a.Frontmatter.Clone()
	var notes []string

	if _, ok := fm.Get("tools"); ok {
		fm.Delete("tools")
		notes = append(notes, "tool allowlist removed — Antigravity's tool names differ from Claude Code's, and an unrecognised allowlist would leave the agent with no tools")
	}
	if model := dropClaudeModel(&fm); model != "" {
		notes = append(notes, fmt.Sprintf("model %q is a Claude Code alias; removed so Antigravity's default applies", model))
	}

	dropped := fm.Retain("name", "description", "color")
	if note := noteDropped("frontmatter", dropped); note != "" {
		notes = append(notes, note)
	}
	fm.Set("name", a.Name)
	// Without this the file is a plain custom agent and the primary agent
	// cannot reach it through invoke_subagent.
	fm.SetBool("subagent", true)

	pl.Add(install.Action{
		Path:      filepath.Join(root, a.Name+".md"),
		Data:      plugin.Document(fm, a.Body),
		Kind:      plugin.KindAgent,
		Name:      a.Name,
		Transform: "subagent: true added",
		Notes:     notes,
	})
}

func antigravityManifest(p *plugin.Plugin) ([]byte, error) {
	// A struct rather than a map: map marshalling sorts keys alphabetically,
	// which would put $schema and description ahead of name.
	m := struct {
		Schema      string `json:"$schema"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
	}{
		Schema:      "https://antigravity.google/schemas/v1/plugin.json",
		Name:        p.Name,
		Description: p.Description,
	}
	data, err := json.MarshalIndent(m, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("render Antigravity plugin.json: %w", err)
	}
	return append(data, '\n'), nil
}
