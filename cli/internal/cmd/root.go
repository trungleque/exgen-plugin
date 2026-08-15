// Package cmd wires the cobra command tree.
package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/trungleque/exgen-plugin/cli/internal/bundled"
	"github.com/trungleque/exgen-plugin/cli/internal/plugin"
	"github.com/trungleque/exgen-plugin/cli/internal/target"
)

// Version is stamped at build time with -ldflags; see version().
var Version = ""

// version resolves what the binary reports, preferring the stamped value and
// falling back to the embedded plugin's own version so that a `go install`
// build still names a release rather than "dev".
func version() string {
	if Version != "" {
		return Version
	}
	if v := bundled.Version(); v != "" {
		return v
	}
	return "dev"
}

// sourceFlags are shared by every command that needs a plugin.
type sourceFlags struct {
	from    string
	global  bool
	project bool
	only    []string
}

func (f *sourceFlags) register(c *cobra.Command) {
	c.Flags().StringVar(&f.from, "from", "", "path to a Claude Code plugin directory (default: the embedded exgen plugin)")
	c.Flags().BoolVar(&f.global, "global", false, "install for the current user (default)")
	c.Flags().BoolVar(&f.project, "project", false, "install into the current repository instead")
	c.Flags().StringSliceVar(&f.only, "only", nil, "limit to component kinds: skills, agents, commands")
}

func (f *sourceFlags) scope() (target.Scope, error) {
	if f.global && f.project {
		return "", fmt.Errorf("--global and --project are mutually exclusive")
	}
	if f.project {
		return target.ScopeProject, nil
	}
	return target.ScopeGlobal, nil
}

func (f *sourceFlags) kinds() ([]plugin.Kind, error) {
	var kinds []plugin.Kind
	for _, raw := range f.only {
		switch strings.ToLower(strings.TrimSpace(strings.TrimSuffix(raw, "s"))) {
		case "skill":
			kinds = append(kinds, plugin.KindSkill)
		case "agent":
			kinds = append(kinds, plugin.KindAgent)
		case "command":
			kinds = append(kinds, plugin.KindCommand)
		default:
			return nil, fmt.Errorf("unknown component kind %q (want: skills, agents, commands)", raw)
		}
	}
	return kinds, nil
}

func (f *sourceFlags) load() (*plugin.Plugin, error) {
	if f.from != "" {
		return plugin.LoadDir(f.from)
	}
	return bundled.Plugin()
}

func (f *sourceFlags) context() (target.Context, error) {
	scope, err := f.scope()
	if err != nil {
		return target.Context{}, err
	}
	kinds, err := f.kinds()
	if err != nil {
		return target.Context{}, err
	}
	return target.NewContext(scope, kinds)
}

// manifestPath is where the record of an install lives: beside the user's
// other config for a global install, and inside the repository for a project
// one, so a checked-in install carries its own record.
func manifestPath(ctx target.Context) string {
	root := ctx.Home
	if ctx.Scope == target.ScopeProject {
		root = ctx.Dir
	}
	return filepath.Join(root, ".exgen", "manifest.json")
}

// NewRootCommand builds the command tree.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "exgen",
		Short: "Install Claude Code plugin skills, agents and commands into other coding tools",
		Long: strings.TrimSpace(`
exgen installs the skills, agents and commands of a Claude Code plugin into the
coding tool you actually use — Claude Code, Pi, the Antigravity CLI, or Codex /
ChatGPT — translating each component into that tool's on-disk conventions and
reporting anything that could not cross over intact.

With no --from, it installs the exgen spec-driven development toolkit that ships
inside this binary.`),
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version(),
	}
	root.AddCommand(newInstallCommand())
	root.AddCommand(newUninstallCommand())
	root.AddCommand(newListCommand())
	root.AddCommand(newTargetsCommand())
	return root
}

// Execute runs the CLI.
func Execute() error { return NewRootCommand().Execute() }
