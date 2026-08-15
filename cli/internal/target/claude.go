package target

import (
	"path/filepath"

	"github.com/trungleque/exgen-plugin/cli/internal/install"
	"github.com/trungleque/exgen-plugin/cli/internal/plugin"
)

func init() { register(claudeTarget{}) }

// claudeTarget is the identity translation: Claude Code's layout is the source
// format, so every component is copied byte for byte.
type claudeTarget struct{}

func (claudeTarget) ID() string    { return "claude" }
func (claudeTarget) Title() string { return "Claude Code" }
func (claudeTarget) Docs() string  { return "https://code.claude.com/docs/en/plugins" }

func (t claudeTarget) root(ctx Context) string { return ctx.Base(".claude", ".claude") }

func (t claudeTarget) Support(ctx Context) []Support {
	root := t.root(ctx)
	return []Support{
		{plugin.KindSkill, true, Abbrev(filepath.Join(root, "skills", "<name>", "SKILL.md"), ctx.Home), ""},
		{plugin.KindAgent, true, Abbrev(filepath.Join(root, "agents", "<name>.md"), ctx.Home), ""},
		{plugin.KindCommand, true, Abbrev(filepath.Join(root, "commands", "<name>.md"), ctx.Home), ""},
	}
}

func (t claudeTarget) Plan(p *plugin.Plugin, ctx Context) (*install.Plan, error) {
	root := t.root(ctx)
	pl := &install.Plan{
		Target:      t.ID(),
		TargetTitle: t.Title(),
		Scope:       string(ctx.Scope),
		Roots:       []string{Abbrev(root, ctx.Home)},
	}

	if ctx.Wants(plugin.KindSkill) {
		for _, s := range p.Skills {
			dir := filepath.Join(root, "skills", s.Name)
			pl.Add(install.Action{
				Path: filepath.Join(dir, "SKILL.md"),
				Data: plugin.Document(s.Frontmatter, s.Body),
				Kind: plugin.KindSkill,
				Name: s.Name,
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
		}
	}

	if ctx.Wants(plugin.KindAgent) {
		for _, a := range p.Agents {
			pl.Add(install.Action{
				Path: filepath.Join(root, "agents", a.Name+".md"),
				Data: plugin.Document(a.Frontmatter, a.Body),
				Kind: plugin.KindAgent,
				Name: a.Name,
			})
		}
	}

	if ctx.Wants(plugin.KindCommand) {
		for _, c := range p.Commands {
			pl.Add(install.Action{
				Path: filepath.Join(root, "commands", filepath.FromSlash(c.Name)+".md"),
				Data: plugin.Document(c.Frontmatter, c.Body),
				Kind: plugin.KindCommand,
				Name: c.Name,
			})
		}
	}

	return pl, nil
}
