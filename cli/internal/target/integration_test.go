package target

import (
	"os"
	"strings"
	"testing"

	"github.com/trungleque/exgen-plugin/cli/internal/plugin"
)

// realPlugin is the plugin this repository ships. Planning against it catches
// translation bugs that a synthetic fixture cannot — notably the review skill's
// link into the implementation skill's bundled reference.
const realPlugin = "../../../plugins/exgen"

func loadReal(t *testing.T) *plugin.Plugin {
	t.Helper()
	if _, err := os.Stat(realPlugin); err != nil {
		t.Skip("running outside the repository")
	}
	p, err := plugin.LoadDir(realPlugin)
	if err != nil {
		t.Fatalf("load %s: %v", realPlugin, err)
	}
	return p
}

// review/SKILL.md points at ../implementation/references/testing-anti-patterns.md.
// That path is correct wherever skills keep sibling directories, and wrong in
// Antigravity's flat project layout, where it has to lose the ../ prefix.
func TestRealPluginCrossSkillReferenceSurvivesEveryLayout(t *testing.T) {
	p := loadReal(t)

	const original = "../implementation/references/testing-anti-patterns.md"
	const flattened = "implementation/references/testing-anti-patterns.md"

	var review plugin.Skill
	for _, s := range p.Skills {
		if s.Name == "review" {
			review = s
		}
	}
	if !strings.Contains(review.Body, original) {
		t.Skipf("review no longer references %s; update this test", original)
	}

	cases := []struct {
		target   string
		scope    Scope
		suffix   string
		wantPath string
	}{
		{"claude", ScopeGlobal, ".claude/skills/review/SKILL.md", original},
		{"pi", ScopeGlobal, ".pi/agent/skills/review/SKILL.md", original},
		{"codex", ScopeGlobal, ".agents/skills/review/SKILL.md", original},
		{"antigravity", ScopeGlobal, "plugins/exgen/skills/review/SKILL.md", original},
		{"antigravity", ScopeProject, ".agents/skills/review.md", flattened},
	}

	for _, tc := range cases {
		t.Run(tc.target+"/"+string(tc.scope), func(t *testing.T) {
			tg, err := Lookup(tc.target)
			if err != nil {
				t.Fatal(err)
			}
			pl, err := tg.Plan(p, testContext(t, tc.scope))
			if err != nil {
				t.Fatal(err)
			}
			body := string(find(t, pl, tc.suffix).Data)

			if !strings.Contains(body, tc.wantPath) {
				t.Errorf("want reference %q, not found in the installed body", tc.wantPath)
			}
			if tc.wantPath == flattened && strings.Contains(body, original) {
				t.Error("the ../ form survived into the flat layout and now dangles")
			}
		})
	}
}

// The bundled reference must land where the rewritten link points.
func TestRealPluginBundledReferenceLandsWhereLinked(t *testing.T) {
	p := loadReal(t)
	tg, _ := Lookup("antigravity")

	pl, err := tg.Plan(p, testContext(t, ScopeProject))
	if err != nil {
		t.Fatal(err)
	}
	find(t, pl, ".agents/skills/implementation/references/testing-anti-patterns.md")
	find(t, pl, ".agents/skills/implementation.md")
}

// Every skill must arrive with the two fields each tool requires to trigger it.
func TestRealPluginKeepsTriggerFieldsEverywhere(t *testing.T) {
	p := loadReal(t)

	for _, tg := range All() {
		for _, scope := range []Scope{ScopeGlobal, ScopeProject} {
			pl, err := tg.Plan(p, testContext(t, scope))
			if err != nil {
				t.Fatalf("%s/%s: %v", tg.ID(), scope, err)
			}
			for _, a := range pl.Actions {
				if a.Kind != plugin.KindSkill || a.Role != "" {
					continue
				}
				fm, _, err := plugin.Split(a.Data)
				if err != nil {
					t.Errorf("%s/%s %s: unparseable: %v", tg.ID(), scope, a.Name, err)
					continue
				}
				if name, _ := fm.Get("name"); name == "" {
					t.Errorf("%s/%s %s: lost its name", tg.ID(), scope, a.Name)
				}
				if desc, _ := fm.Get("description"); desc == "" {
					t.Errorf("%s/%s %s: lost its description and would never trigger", tg.ID(), scope, a.Name)
				}
			}
		}
	}
}
