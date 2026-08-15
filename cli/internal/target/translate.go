package target

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/trungleque/exgen-plugin/cli/internal/plugin"
)

// piTools maps Claude Code tool names onto Pi's. Anything absent has no Pi
// equivalent and is dropped: keeping an unrecognised name in an allowlist
// would narrow the agent to nothing, while dropping the whole list would
// silently widen a read-only agent's reach.
// Only names confirmed in Pi's own documentation appear here — guessing at a
// name would produce an allowlist entry that matches nothing.
var piTools = map[string]string{
	"read":      "read",
	"glob":      "find",
	"grep":      "grep",
	"ls":        "ls",
	"bash":      "bash",
	"write":     "write",
	"edit":      "edit",
	"multiedit": "edit",
}

// splitList parses a frontmatter list written either as "a, b, c" or as a YAML
// flow sequence.
func splitList(raw string) []string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(strings.Trim(strings.TrimSpace(part), `"'`))
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// mapTools translates a Claude tool list, returning the translated list and
// the names that had no equivalent.
func mapTools(raw string, table map[string]string) (kept []string, dropped []string) {
	seen := map[string]bool{}
	for _, name := range splitList(raw) {
		mapped, ok := table[strings.ToLower(name)]
		if !ok {
			dropped = append(dropped, name)
			continue
		}
		if !seen[mapped] {
			seen[mapped] = true
			kept = append(kept, mapped)
		}
	}
	return kept, dropped
}

// argumentsPattern matches Claude Code's whole-argument placeholder.
var argumentsPattern = regexp.MustCompile(`\$ARGUMENTS\b`)

// toPiArguments rewrites Claude's $ARGUMENTS to Pi's $@. Positional $1..$9
// already mean the same thing in both.
func toPiArguments(body string) (string, bool) {
	if !argumentsPattern.MatchString(body) {
		return body, false
	}
	return argumentsPattern.ReplaceAllString(body, "$$@"), true
}

// dropClaudeModel removes a model alias that only Claude Code resolves.
// "sonnet" is not a model id on any other tool, and leaving it in place makes
// the agent fail to start rather than fall back.
func dropClaudeModel(fm *plugin.Frontmatter) string {
	value, ok := fm.Get("model")
	if !ok {
		return ""
	}
	if strings.Contains(value, "/") {
		return "" // already a fully-qualified id, leave it alone
	}
	fm.Delete("model")
	return value
}

// skillDoc renders a skill for a target that keeps the SKILL.md convention,
// retaining only the frontmatter keys that target documents.
func skillDoc(s plugin.Skill, allow []string) ([]byte, []string) {
	fm := s.Frontmatter.Clone()
	if !fm.Has("name") {
		fm.Set("name", s.Name)
	}
	dropped := fm.Retain(allow...)
	return plugin.Document(fm, s.Body), dropped
}

// commandAsSkill converts a slash command into a skill, for tools that have no
// command concept. The command body becomes the skill body; the description
// gains a trigger hint because a skill is selected by its description where a
// command was selected by name.
func commandAsSkill(c plugin.Command) plugin.Skill {
	name := strings.ReplaceAll(c.Name, "/", "-")
	desc := c.Description
	if desc == "" {
		desc = fmt.Sprintf("Runs the %s workflow.", name)
	}
	desc = fmt.Sprintf("%s Use when the user asks to run %q or describes that task.", strings.TrimRight(desc, ". ")+".", name)

	fm := c.Frontmatter.Clone()
	fm.Set("name", name)
	fm.Set("description", desc)

	body := c.Body
	if hint, ok := c.Frontmatter.Get("argument-hint"); ok && hint != "" {
		body = fmt.Sprintf("%s\n\nExpected input: %s\n", strings.TrimRight(body, "\n"), hint)
	}
	return plugin.Skill{Name: name, Description: desc, Frontmatter: fm, Body: body}
}

// agentAsSkill converts a subagent into a skill for tools with no subagent
// concept. The system prompt survives verbatim; what changes is that the
// primary agent runs it inline instead of delegating to a child session.
func agentAsSkill(a plugin.Agent) plugin.Skill {
	desc := a.Description
	if desc == "" {
		desc = fmt.Sprintf("The %s analysis role.", a.Name)
	}
	fm := a.Frontmatter.Clone()
	fm.Retain("name", "description")
	fm.Set("name", a.Name)
	fm.Set("description", desc)

	body := fmt.Sprintf(
		"%s\n\n---\n\nThis capability began as a Claude Code subagent. Apply the role above "+
			"within the current session; there is no separate child agent to delegate to.\n",
		strings.TrimRight(a.Body, "\n"),
	)
	return plugin.Skill{Name: a.Name, Description: desc, Frontmatter: fm, Body: body}
}

// flattenLinks rewrites the relative paths in a skill body for a layout where
// the skill file sits one level above its bundled assets — <name>.md alongside
// <name>/references/… rather than <name>/SKILL.md alongside references/….
//
// Both shapes a plugin uses in practice are handled: a skill pointing at its
// own bundled file, and a skill pointing into a sibling skill's bundle with a
// ../ prefix. Cross-skill links are rewritten first so that the own-file pass
// cannot match inside a path it has already qualified.
func flattenLinks(body, name string, own []plugin.File, siblings []plugin.Skill) (string, bool) {
	changed := false

	for _, sibling := range siblings {
		if sibling.Name == name {
			continue
		}
		for _, f := range sibling.Files {
			from := "../" + sibling.Name + "/" + f.RelPath
			if strings.Contains(body, from) {
				body = strings.ReplaceAll(body, from, sibling.Name+"/"+f.RelPath)
				changed = true
			}
		}
	}

	for _, f := range own {
		// The prefix group keeps already-qualified paths (…/references/x.md)
		// from being rewritten a second time.
		re := regexp.MustCompile(`(^|[^\w/.-])` + regexp.QuoteMeta(f.RelPath))
		if rewritten := re.ReplaceAllString(body, "${1}"+name+"/"+f.RelPath); rewritten != body {
			body = rewritten
			changed = true
		}
	}

	return body, changed
}

// noteDropped renders a "dropped x, y" fragment, or "" when nothing was.
func noteDropped(label string, names []string) string {
	if len(names) == 0 {
		return ""
	}
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	return fmt.Sprintf("%s dropped: %s", label, strings.Join(sorted, ", "))
}
