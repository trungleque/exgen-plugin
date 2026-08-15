// Package target translates a Claude Code plugin into the on-disk conventions
// of each supported coding tool.
//
// Every target answers the same three questions: where do files go, which
// component kinds does the tool actually have, and what is lost in the
// crossing. The lossy answers matter most — Pi and Codex have no native
// subagents, Codex has no slash commands, and each tool names its tools
// differently — so translation is explicit and always reported.
package target

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/trungleque/exgen-plugin/cli/internal/install"
	"github.com/trungleque/exgen-plugin/cli/internal/plugin"
)

// Scope selects between a machine-wide and a repository-local install.
type Scope string

const (
	ScopeGlobal  Scope = "global"
	ScopeProject Scope = "project"
)

// Context is the environment a plan is computed against. Home and Dir are
// explicit rather than read from the process so that tests can plan into a
// temporary directory.
type Context struct {
	Scope Scope
	Home  string
	Dir   string
	Kinds map[plugin.Kind]bool // nil means all kinds
}

// NewContext builds a Context from the real environment.
func NewContext(scope Scope, kinds []plugin.Kind) (Context, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Context{}, fmt.Errorf("locate home directory: %w", err)
	}
	dir, err := os.Getwd()
	if err != nil {
		return Context{}, fmt.Errorf("locate working directory: %w", err)
	}
	ctx := Context{Scope: scope, Home: home, Dir: dir}
	if len(kinds) > 0 {
		ctx.Kinds = map[plugin.Kind]bool{}
		for _, k := range kinds {
			ctx.Kinds[k] = true
		}
	}
	return ctx, nil
}

// Wants reports whether a component kind is selected by --only.
func (c Context) Wants(k plugin.Kind) bool {
	if c.Kinds == nil {
		return true
	}
	return c.Kinds[k]
}

// Base is the root the target writes under for the active scope.
func (c Context) Base(globalRel, projectRel string) string {
	if c.Scope == ScopeProject {
		return filepath.Join(c.Dir, projectRel)
	}
	return filepath.Join(c.Home, globalRel)
}

// Support describes how a target handles one component kind.
type Support struct {
	Kind     plugin.Kind
	Native   bool   // the tool has this concept
	Location string // where it lands, tilde-abbreviated
	Note     string // how it is translated, when not a verbatim copy
}

// Target is one installable coding tool.
type Target interface {
	ID() string
	Title() string
	Docs() string
	Support(ctx Context) []Support
	Plan(p *plugin.Plugin, ctx Context) (*install.Plan, error)
}

var registry = map[string]Target{}
var order []string

func register(t Target) {
	registry[t.ID()] = t
	order = append(order, t.ID())
	sort.Strings(order)
}

// All returns every registered target in a stable order.
func All() []Target {
	out := make([]Target, 0, len(order))
	for _, id := range order {
		out = append(out, registry[id])
	}
	return out
}

// IDs lists registered target identifiers.
func IDs() []string {
	out := make([]string, len(order))
	copy(out, order)
	return out
}

// Lookup resolves a target id, accepting a few common aliases.
func Lookup(id string) (Target, error) {
	key := strings.ToLower(strings.TrimSpace(id))
	if alias, ok := aliases[key]; ok {
		key = alias
	}
	t, ok := registry[key]
	if !ok {
		return nil, fmt.Errorf("unknown target %q (known: %s, all)", id, strings.Join(IDs(), ", "))
	}
	return t, nil
}

var aliases = map[string]string{
	"claude-code":     "claude",
	"cc":              "claude",
	"anti":            "antigravity",
	"antigravity-cli": "antigravity",
	"ag":              "antigravity",
	"chatgpt":         "codex",
	"openai":          "codex",
	"pi-cli":          "pi",
}

// Resolve expands target arguments, where "all" means every target.
func Resolve(args []string) ([]Target, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no target given (known: %s, all)", strings.Join(IDs(), ", "))
	}
	seen := map[string]bool{}
	var out []Target
	for _, arg := range args {
		if strings.EqualFold(strings.TrimSpace(arg), "all") {
			for _, t := range All() {
				if !seen[t.ID()] {
					seen[t.ID()] = true
					out = append(out, t)
				}
			}
			continue
		}
		t, err := Lookup(arg)
		if err != nil {
			return nil, err
		}
		if !seen[t.ID()] {
			seen[t.ID()] = true
			out = append(out, t)
		}
	}
	return out, nil
}

// Abbrev shortens a path under home to ~/… for display.
func Abbrev(path, home string) string {
	if home != "" && strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + path[len(home):]
	}
	if cwd, err := os.Getwd(); err == nil && strings.HasPrefix(path, cwd+string(filepath.Separator)) {
		return "." + path[len(cwd):]
	}
	return path
}
