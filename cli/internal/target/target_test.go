package target

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"

	"github.com/trungleque/exgen-plugin/cli/internal/install"
	"github.com/trungleque/exgen-plugin/cli/internal/plugin"
)

// fixture is a plugin exercising every feature a target has to translate: a
// skill with a bundled reference, a model-invisible skill, an agent with a
// Claude-only tool list and model alias, and a command using $ARGUMENTS.
func fixture(t *testing.T) *plugin.Plugin {
	t.Helper()
	dir := t.TempDir()

	write := func(rel, content string) {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write(".claude-plugin/plugin.json", `{"name":"demo","description":"Demo plugin","version":"1.2.3","license":"MIT"}`)
	write("skills/builder/SKILL.md", "---\nname: builder\ndescription: Builds things.\nargument-hint: \"[id]\"\n---\n\nSee `references/notes.md`.\n")
	write("skills/builder/references/notes.md", "reference body\n")
	write("skills/planner/SKILL.md", "---\nname: planner\ndescription: Plans things.\ndisable-model-invocation: true\nargument-hint: \"[feature]\"\n---\n\nPlan it.\n")
	write("agents/scout.md", "---\nname: scout\ndescription: Scouts the code.\ntools: Glob, Grep, LS, Read, TodoWrite\nmodel: sonnet\ncolor: yellow\n---\n\nYou are a scout.\n")
	write("commands/ship.md", "---\ndescription: Ships it.\nargument-hint: \"[env]\"\n---\n\nShip to $ARGUMENTS now.\n")

	p, err := plugin.LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	return p
}

func testContext(t *testing.T, scope Scope) Context {
	t.Helper()
	return Context{Scope: scope, Home: t.TempDir(), Dir: t.TempDir()}
}

// find returns the action whose path ends with suffix.
func find(t *testing.T, pl *install.Plan, suffix string) install.Action {
	t.Helper()
	for _, a := range pl.Actions {
		if strings.HasSuffix(filepath.ToSlash(a.Path), suffix) {
			return a
		}
	}
	var paths []string
	for _, a := range pl.Actions {
		paths = append(paths, filepath.ToSlash(a.Path))
	}
	t.Fatalf("no action ending in %q; have:\n  %s", suffix, strings.Join(paths, "\n  "))
	return install.Action{}
}

func absent(t *testing.T, pl *install.Plan, suffix string) {
	t.Helper()
	for _, a := range pl.Actions {
		if strings.HasSuffix(filepath.ToSlash(a.Path), suffix) {
			t.Fatalf("did not expect an action at %q", a.Path)
		}
	}
}

func planFor(t *testing.T, id string, ctx Context) (*plugin.Plugin, *install.Plan) {
	t.Helper()
	p := fixture(t)
	tg, err := Lookup(id)
	if err != nil {
		t.Fatal(err)
	}
	pl, err := tg.Plan(p, ctx)
	if err != nil {
		t.Fatalf("%s plan: %v", id, err)
	}
	return p, pl
}

func TestFixtureLoads(t *testing.T) {
	p := fixture(t)
	if p.Name != "demo" || p.Version != "1.2.3" {
		t.Errorf("manifest not read: %+v", p)
	}
	if len(p.Skills) != 2 || len(p.Agents) != 1 || len(p.Commands) != 1 {
		t.Fatalf("got %d skills, %d agents, %d commands", len(p.Skills), len(p.Agents), len(p.Commands))
	}
	builder := p.Skills[0]
	if builder.Name != "builder" || len(builder.Files) != 1 {
		t.Errorf("builder = %+v, want 1 bundled file", builder)
	}
	if builder.Files[0].RelPath != "references/notes.md" {
		t.Errorf("bundled file = %q", builder.Files[0].RelPath)
	}
}

// Claude Code is the source format, so its install must be byte-for-byte.
func TestClaudeCopiesVerbatim(t *testing.T) {
	p, pl := planFor(t, "claude", testContext(t, ScopeGlobal))

	skill := find(t, pl, ".claude/skills/builder/SKILL.md")
	want := plugin.Document(p.Skills[0].Frontmatter, p.Skills[0].Body)
	if string(skill.Data) != string(want) {
		t.Errorf("skill not verbatim:\n got %q\nwant %q", skill.Data, want)
	}
	if !strings.Contains(string(skill.Data), "argument-hint") {
		t.Error("Claude dropped argument-hint; it should keep every key")
	}

	agent := find(t, pl, ".claude/agents/scout.md")
	if !strings.Contains(string(agent.Data), "model: sonnet") {
		t.Error("Claude dropped the model alias it is the only tool to understand")
	}
	if !strings.Contains(string(agent.Data), "TodoWrite") {
		t.Error("Claude remapped tools; it should not")
	}

	find(t, pl, ".claude/skills/builder/references/notes.md")
	find(t, pl, ".claude/commands/ship.md")
}

func TestPiRewritesArgumentsInPrompts(t *testing.T) {
	_, pl := planFor(t, "pi", testContext(t, ScopeGlobal))

	prompt := find(t, pl, ".pi/agent/prompts/ship.md")
	body := string(prompt.Data)
	if strings.Contains(body, "$ARGUMENTS") {
		t.Error("$ARGUMENTS survived into a Pi prompt template")
	}
	if !strings.Contains(body, "Ship to $@ now.") {
		t.Errorf("prompt body = %q, want $@", body)
	}
}

func TestPiRemapsAgentToolsAndDropsModelAlias(t *testing.T) {
	_, pl := planFor(t, "pi", testContext(t, ScopeGlobal))
	agent := find(t, pl, ".pi/agent/agents/scout.md")

	fm, _, err := plugin.Split(agent.Data)
	if err != nil {
		t.Fatalf("generated agent is not parseable: %v", err)
	}
	tools, _ := fm.Get("tools")
	if tools != "find, grep, ls, read" {
		t.Errorf("tools = %q, want the Pi names in Claude order", tools)
	}
	if fm.Has("model") {
		t.Error("the Claude-only model alias should not reach Pi")
	}
	if got, _ := fm.Get("color"); got != "yellow" {
		t.Errorf("color = %q, want yellow (both pi-subagents forks support it)", got)
	}
	if !strings.Contains(strings.Join(agent.Notes, " "), "TodoWrite") {
		t.Errorf("notes should name the dropped tool, got %v", agent.Notes)
	}
	if len(pl.Notes) == 0 || !strings.Contains(pl.Notes[0], "pi-subagents") {
		t.Errorf("plan should tell the user to install pi-subagents, got %v", pl.Notes)
	}
}

// A skill Pi hides from the model has no other invocation path, so it needs a
// prompt template to stay reachable.
func TestPiAddsPromptShimForModelInvisibleSkill(t *testing.T) {
	_, pl := planFor(t, "pi", testContext(t, ScopeGlobal))

	shim := find(t, pl, ".pi/agent/prompts/planner.md")
	if !strings.Contains(string(shim.Data), "planner") {
		t.Errorf("shim should reference the skill, got %q", shim.Data)
	}
	if !strings.Contains(string(shim.Data), "$@") {
		t.Error("shim should forward arguments")
	}
	// builder is model-invocable, so it needs no shim.
	absent(t, pl, ".pi/agent/prompts/builder.md")
}

// A project install is committed and cloned elsewhere, so a generated
// cross-reference must not bake in an absolute path.
func TestPiProjectShimUsesRelativePath(t *testing.T) {
	ctx := testContext(t, ScopeProject)
	_, pl := planFor(t, "pi", ctx)

	shim := find(t, pl, ".pi/prompts/planner.md")
	if strings.Contains(string(shim.Data), ctx.Dir) {
		t.Errorf("project shim embeds an absolute path:\n%s", shim.Data)
	}
	if !strings.Contains(string(shim.Data), filepath.Join(".pi", "skills", "planner", "SKILL.md")) {
		t.Errorf("shim should point at the skill relatively, got %q", shim.Data)
	}
}

func TestPiStripsUnknownSkillFrontmatter(t *testing.T) {
	_, pl := planFor(t, "pi", testContext(t, ScopeGlobal))
	skill := find(t, pl, ".pi/agent/skills/builder/SKILL.md")

	fm, _, err := plugin.Split(skill.Data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if fm.Has("argument-hint") {
		t.Error("argument-hint is not in Pi's skill schema and should be dropped")
	}
	if !fm.Has("name") || !fm.Has("description") {
		t.Error("Pi's required skill fields must survive")
	}
	find(t, pl, ".pi/agent/skills/builder/references/notes.md")
}

func TestAntigravityMarksAgentsAsSubagents(t *testing.T) {
	_, pl := planFor(t, "antigravity", testContext(t, ScopeGlobal))
	agent := find(t, pl, "plugins/demo/agents/scout.md")

	var probe struct {
		Subagent bool   `yaml:"subagent"`
		Name     string `yaml:"name"`
		Tools    string `yaml:"tools"`
	}
	head := strings.SplitN(string(agent.Data), "---\n", 3)
	if err := yaml.Unmarshal([]byte(head[1]), &probe); err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	if !probe.Subagent {
		t.Error("without subagent: true the primary agent cannot invoke it")
	}
	if probe.Name != "scout" {
		t.Errorf("name = %q", probe.Name)
	}
	if probe.Tools != "" {
		t.Error("Claude tool names would not resolve in Antigravity and should be dropped")
	}
}

func TestAntigravityGlobalWritesAPluginManifest(t *testing.T) {
	_, pl := planFor(t, "antigravity", testContext(t, ScopeGlobal))
	manifest := find(t, pl, "plugins/demo/plugin.json")

	body := string(manifest.Data)
	for _, want := range []string{`"$schema"`, `"name": "demo"`, `"description"`} {
		if !strings.Contains(body, want) {
			t.Errorf("manifest missing %s:\n%s", want, body)
		}
	}
	if manifest.Kind != plugin.KindManifest {
		t.Errorf("manifest kind = %q, want manifest", manifest.Kind)
	}
}

// Project scope uses Antigravity's documented flat layout for every skill.
func TestAntigravityProjectUsesFlatLayout(t *testing.T) {
	_, pl := planFor(t, "antigravity", testContext(t, ScopeProject))

	find(t, pl, ".agents/skills/planner.md")
	find(t, pl, ".agents/skills/builder.md")
	absent(t, pl, ".agents/skills/builder/SKILL.md")

	// Bundled files still travel, one level below the skill file.
	find(t, pl, ".agents/skills/builder/references/notes.md")
}

// Moving the skill file up a level must move its links with it, or every
// reference to a bundled file dangles.
func TestAntigravityFlatLayoutRewritesLinks(t *testing.T) {
	_, pl := planFor(t, "antigravity", testContext(t, ScopeProject))
	skill := find(t, pl, ".agents/skills/builder.md")

	body := string(skill.Data)
	if !strings.Contains(body, "`builder/references/notes.md`") {
		t.Errorf("link not rewritten for the flat layout:\n%s", body)
	}
	if strings.Contains(body, "See `references/notes.md`") {
		t.Error("the original relative link survived and now points nowhere")
	}
	if !strings.Contains(strings.Join(skill.Notes, " "), "relative links rewritten") {
		t.Errorf("the rewrite should be reported, got %v", skill.Notes)
	}
}

// The global plugin layout keeps SKILL.md in its own directory, so links must
// be left exactly as written.
func TestAntigravityGlobalLeavesLinksAlone(t *testing.T) {
	_, pl := planFor(t, "antigravity", testContext(t, ScopeGlobal))
	skill := find(t, pl, "plugins/demo/skills/builder/SKILL.md")

	if !strings.Contains(string(skill.Data), "See `references/notes.md`") {
		t.Errorf("directory layout should not rewrite links:\n%s", skill.Data)
	}
}

func TestFlattenLinksHandlesCrossSkillReferences(t *testing.T) {
	siblings := []plugin.Skill{
		{Name: "implementation", Files: []plugin.File{{RelPath: "references/anti-patterns.md"}}},
		{Name: "review"},
	}
	body := "Check `../implementation/references/anti-patterns.md` before reviewing.\n"

	got, changed := flattenLinks(body, "review", nil, siblings)
	if !changed {
		t.Fatal("expected a rewrite")
	}
	if !strings.Contains(got, "`implementation/references/anti-patterns.md`") {
		t.Errorf("cross-skill link not rewritten: %q", got)
	}
	if strings.Contains(got, "../") {
		t.Errorf("the ../ prefix should be gone: %q", got)
	}
}

// A path that is already qualified must not be rewritten twice.
func TestFlattenLinksLeavesQualifiedPathsAlone(t *testing.T) {
	own := []plugin.File{{RelPath: "references/notes.md"}}
	body := "Bundled at `skills/builder/references/notes.md` in the source tree.\n"

	got, _ := flattenLinks(body, "builder", own, nil)
	if strings.Contains(got, "builder/builder/") {
		t.Errorf("double-rewrote an already-qualified path: %q", got)
	}
	if !strings.Contains(got, "`skills/builder/references/notes.md`") {
		t.Errorf("qualified path was altered: %q", got)
	}
}

func TestAntigravityInstallsCommandsAsSkills(t *testing.T) {
	_, pl := planFor(t, "antigravity", testContext(t, ScopeProject))
	skill := find(t, pl, ".agents/skills/ship.md")

	if skill.Kind != plugin.KindCommand {
		t.Errorf("kind = %q, want the action to remember it came from a command", skill.Kind)
	}
	if !strings.Contains(string(skill.Data), "name: ship") {
		t.Errorf("converted command needs a name, got:\n%s", skill.Data)
	}
	absent(t, pl, ".agents/commands/ship.md")
}

// The riskiest transform: Codex agents are TOML, not markdown.
func TestCodexAgentIsValidTOML(t *testing.T) {
	p, pl := planFor(t, "codex", testContext(t, ScopeGlobal))
	agent := find(t, pl, ".codex/agents/scout.toml")

	var parsed struct {
		Name                  string `toml:"name"`
		Description           string `toml:"description"`
		DeveloperInstructions string `toml:"developer_instructions"`
	}
	if _, err := toml.Decode(string(agent.Data), &parsed); err != nil {
		t.Fatalf("generated TOML does not parse: %v\n%s", err, agent.Data)
	}
	if parsed.Name != "scout" {
		t.Errorf("name = %q", parsed.Name)
	}
	if parsed.Description != p.Agents[0].Description {
		t.Errorf("description = %q, want %q", parsed.Description, p.Agents[0].Description)
	}
	if !strings.Contains(parsed.DeveloperInstructions, "You are a scout.") {
		t.Errorf("the prompt body must survive into developer_instructions, got %q", parsed.DeveloperInstructions)
	}
}

// Prompts routinely contain quotes, backslashes and code fences; none of them
// may break out of the multi-line string.
func TestCodexTOMLEscapesHostileBodies(t *testing.T) {
	body := "Quote: \"x\"\nBackslash: \\n literal\nFence: \"\"\" inside\nTrailing quote: \"\n"
	data := codexAgentTOML("edge", `desc with "quotes" and \backslash`, body)

	var parsed struct {
		Name                  string `toml:"name"`
		Description           string `toml:"description"`
		DeveloperInstructions string `toml:"developer_instructions"`
	}
	if _, err := toml.Decode(string(data), &parsed); err != nil {
		t.Fatalf("hostile body broke the TOML: %v\n%s", err, data)
	}
	if parsed.Description != `desc with "quotes" and \backslash` {
		t.Errorf("description mangled: %q", parsed.Description)
	}
	for _, want := range []string{`Quote: "x"`, `Backslash: \n literal`, `Fence: """ inside`} {
		if !strings.Contains(parsed.DeveloperInstructions, want) {
			t.Errorf("instructions lost %q:\n%q", want, parsed.DeveloperInstructions)
		}
	}
}

func TestCodexInstallsCommandsAsSkills(t *testing.T) {
	_, pl := planFor(t, "codex", testContext(t, ScopeGlobal))
	skill := find(t, pl, ".agents/skills/ship/SKILL.md")

	fm, body, err := plugin.Split(skill.Data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got, _ := fm.Get("name"); got != "ship" {
		t.Errorf("name = %q", got)
	}
	desc, _ := fm.Get("description")
	if !strings.Contains(desc, "ship") {
		t.Errorf("a converted command needs a description that can trigger it, got %q", desc)
	}
	if !strings.Contains(body, "$ARGUMENTS") {
		t.Error("Codex keeps $ARGUMENTS as-is; only Pi rewrites it")
	}
}

func TestCodexKeepsOnlyDocumentedSkillFields(t *testing.T) {
	_, pl := planFor(t, "codex", testContext(t, ScopeGlobal))
	skill := find(t, pl, ".agents/skills/planner/SKILL.md")

	fm, _, err := plugin.Split(skill.Data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if keys := fm.Keys(); len(keys) != 2 {
		t.Errorf("keys = %v, want just name and description", keys)
	}
}

func TestOnlyFlagLimitsComponentKinds(t *testing.T) {
	ctx := testContext(t, ScopeGlobal)
	ctx.Kinds = map[plugin.Kind]bool{plugin.KindSkill: true}
	_, pl := planFor(t, "claude", ctx)

	for _, a := range pl.Actions {
		if a.Kind != plugin.KindSkill {
			t.Errorf("--only skills still planned a %s: %s", a.Kind, a.Path)
		}
	}
}

func TestResolveExpandsAll(t *testing.T) {
	all, err := Resolve([]string{"all"})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != len(All()) {
		t.Errorf("all resolved to %d targets, want %d", len(all), len(All()))
	}

	deduped, err := Resolve([]string{"pi", "pi", "chatgpt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(deduped) != 2 {
		t.Errorf("got %d targets, want pi and codex", len(deduped))
	}
	if deduped[1].ID() != "codex" {
		t.Errorf("chatgpt should alias to codex, got %s", deduped[1].ID())
	}
}

func TestResolveRejectsUnknownTarget(t *testing.T) {
	if _, err := Resolve([]string{"emacs"}); err == nil {
		t.Fatal("expected an error for an unknown target")
	}
}

// Every target must produce a plan for every component kind it claims to
// support, so the capability table cannot drift from the planner.
func TestSupportTableMatchesPlan(t *testing.T) {
	for _, scope := range []Scope{ScopeGlobal, ScopeProject} {
		for _, tg := range All() {
			ctx := testContext(t, scope)
			p := fixture(t)
			pl, err := tg.Plan(p, ctx)
			if err != nil {
				t.Fatalf("%s/%s: %v", tg.ID(), scope, err)
			}
			planned := map[plugin.Kind]bool{}
			for _, a := range pl.Actions {
				planned[a.Kind] = true
			}
			for _, s := range tg.Support(ctx) {
				if !planned[s.Kind] {
					t.Errorf("%s/%s claims to support %ss but planned none", tg.ID(), scope, s.Kind)
				}
			}
		}
	}
}
