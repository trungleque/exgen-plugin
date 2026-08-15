// Package plugin reads a Claude Code plugin directory into a target-neutral
// model. Claude Code's layout is the source format for every install target,
// so this package deliberately knows nothing about Pi, Antigravity or Codex.
package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
)

// Kind names a component category.
type Kind string

const (
	KindSkill   Kind = "skill"
	KindAgent   Kind = "agent"
	KindCommand Kind = "command"
	// KindManifest is not a source component: it labels files a target
	// generates for itself, such as an Antigravity plugin.json.
	KindManifest Kind = "manifest"
)

// AllKinds lists source component categories in install order.
var AllKinds = []Kind{KindSkill, KindAgent, KindCommand}

// Plugin is a parsed Claude Code plugin.
type Plugin struct {
	Name        string
	Description string
	Version     string
	License     string
	Author      string
	Source      string // human-readable origin, e.g. a path or "embedded"

	Skills   []Skill
	Agents   []Agent
	Commands []Command
}

// Skill is a skills/<name>/SKILL.md plus anything bundled alongside it.
type Skill struct {
	Name        string
	Description string
	Frontmatter Frontmatter
	Body        string
	Files       []File // paths relative to the skill directory, SKILL.md excluded
}

// Agent is an agents/<name>.md subagent definition.
type Agent struct {
	Name        string
	Description string
	Frontmatter Frontmatter
	Body        string
}

// Command is a commands/<name>.md slash command. Name keeps any subdirectory
// prefix, which Claude Code treats as a namespace.
type Command struct {
	Name        string
	Description string
	Frontmatter Frontmatter
	Body        string
}

// File is a bundled asset carried along with a skill.
type File struct {
	RelPath string
	Data    []byte
}

// Manifest mirrors .claude-plugin/plugin.json.
type Manifest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	License     string `json:"license"`
	Author      struct {
		Name string `json:"name"`
	} `json:"author"`
}

// IsEmpty reports whether the plugin carries no installable component.
func (p *Plugin) IsEmpty() bool {
	return len(p.Skills)+len(p.Agents)+len(p.Commands) == 0
}

// Count returns the number of components of a kind.
func (p *Plugin) Count(k Kind) int {
	switch k {
	case KindSkill:
		return len(p.Skills)
	case KindAgent:
		return len(p.Agents)
	case KindCommand:
		return len(p.Commands)
	}
	return 0
}

// LoadDir reads a plugin from a filesystem path.
func LoadDir(dir string) (*Plugin, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("plugin source %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("plugin source %s is not a directory", dir)
	}
	p, err := Load(os.DirFS(dir))
	if err != nil {
		return nil, err
	}
	p.Source = dir
	return p, nil
}

// Load reads a plugin rooted at the given filesystem.
func Load(fsys fs.FS) (*Plugin, error) {
	p := &Plugin{Source: "embedded"}

	if data, err := fs.ReadFile(fsys, ".claude-plugin/plugin.json"); err == nil {
		var m Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("parse .claude-plugin/plugin.json: %w", err)
		}
		p.Name, p.Description = m.Name, m.Description
		p.Version, p.License, p.Author = m.Version, m.License, m.Author.Name
	}
	if p.Name == "" {
		p.Name = "plugin"
	}

	if err := loadSkills(fsys, p); err != nil {
		return nil, err
	}
	if err := loadAgents(fsys, p); err != nil {
		return nil, err
	}
	if err := loadCommands(fsys, p); err != nil {
		return nil, err
	}
	if p.IsEmpty() {
		return nil, fmt.Errorf("no skills, agents or commands found — is this a Claude Code plugin directory?")
	}
	return p, nil
}

func loadSkills(fsys fs.FS, p *Plugin) error {
	entries, err := fs.ReadDir(fsys, "skills")
	if err != nil {
		return nil // no skills/ is fine
	}
	for _, e := range entries {
		if e.IsDir() {
			skill, err := loadSkillDir(fsys, path.Join("skills", e.Name()), e.Name())
			if err != nil {
				return err
			}
			if skill != nil {
				p.Skills = append(p.Skills, *skill)
			}
			continue
		}
		// A bare skills/<name>.md is tolerated so that flat layouts round-trip.
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		fm, body, err := readComponent(fsys, path.Join("skills", e.Name()))
		if err != nil {
			return err
		}
		p.Skills = append(p.Skills, newSkill(name, fm, body, nil))
	}
	sort.Slice(p.Skills, func(i, j int) bool { return p.Skills[i].Name < p.Skills[j].Name })
	return nil
}

func loadSkillDir(fsys fs.FS, dir, name string) (*Skill, error) {
	fm, body, err := readComponent(fsys, path.Join(dir, "SKILL.md"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil // directory without SKILL.md is not a skill
		}
		return nil, err
	}

	var files []File
	walkErr := fs.WalkDir(fsys, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || p == path.Join(dir, "SKILL.md") {
			return nil
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(p, dir+"/")
		files = append(files, File{RelPath: rel, Data: data})
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("read skill %s: %w", name, walkErr)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].RelPath < files[j].RelPath })

	s := newSkill(name, fm, body, files)
	return &s, nil
}

func newSkill(name string, fm Frontmatter, body string, files []File) Skill {
	if declared, ok := fm.Get("name"); ok && declared != "" {
		name = declared
	}
	desc, _ := fm.Get("description")
	return Skill{Name: name, Description: desc, Frontmatter: fm, Body: body, Files: files}
}

func loadAgents(fsys fs.FS, p *Plugin) error {
	entries, err := fs.ReadDir(fsys, "agents")
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		fm, body, err := readComponent(fsys, path.Join("agents", e.Name()))
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		if declared, ok := fm.Get("name"); ok && declared != "" {
			name = declared
		}
		desc, _ := fm.Get("description")
		p.Agents = append(p.Agents, Agent{Name: name, Description: desc, Frontmatter: fm, Body: body})
	}
	sort.Slice(p.Agents, func(i, j int) bool { return p.Agents[i].Name < p.Agents[j].Name })
	return nil
}

func loadCommands(fsys fs.FS, p *Plugin) error {
	if _, err := fs.Stat(fsys, "commands"); err != nil {
		return nil
	}
	err := fs.WalkDir(fsys, "commands", func(fp string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(fp, ".md") {
			return nil
		}
		fm, body, readErr := readComponent(fsys, fp)
		if readErr != nil {
			return readErr
		}
		name := strings.TrimSuffix(strings.TrimPrefix(fp, "commands/"), ".md")
		desc, _ := fm.Get("description")
		p.Commands = append(p.Commands, Command{Name: name, Description: desc, Frontmatter: fm, Body: body})
		return nil
	})
	if err != nil {
		return fmt.Errorf("read commands: %w", err)
	}
	sort.Slice(p.Commands, func(i, j int) bool { return p.Commands[i].Name < p.Commands[j].Name })
	return nil
}

func readComponent(fsys fs.FS, p string) (Frontmatter, string, error) {
	data, err := fs.ReadFile(fsys, p)
	if err != nil {
		return Frontmatter{}, "", err
	}
	fm, body, err := Split(data)
	if err != nil {
		return Frontmatter{}, "", fmt.Errorf("%s: %w", p, err)
	}
	return fm, body, nil
}
