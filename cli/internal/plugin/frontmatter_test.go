package plugin

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSplitSeparatesFrontmatterFromBody(t *testing.T) {
	fm, body, err := Split([]byte("---\nname: demo\ndescription: A thing\n---\n\n# Heading\n\ntext\n"))
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if got, _ := fm.Get("name"); got != "demo" {
		t.Errorf("name = %q, want demo", got)
	}
	if !strings.HasPrefix(body, "# Heading") {
		t.Errorf("body = %q, want it to start at the heading", body)
	}
}

func TestSplitPreservesKeyOrder(t *testing.T) {
	fm, _, err := Split([]byte("---\nname: a\nargument-hint: b\ndescription: c\n---\nbody\n"))
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	want := []string{"name", "argument-hint", "description"}
	got := fm.Keys()
	if len(got) != len(want) {
		t.Fatalf("keys = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("keys = %v, want %v", got, want)
		}
	}
}

func TestSplitWithoutFrontmatterReturnsWholeFile(t *testing.T) {
	fm, body, err := Split([]byte("# Just markdown\n"))
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if fm.Len() != 0 {
		t.Errorf("Len = %d, want 0", fm.Len())
	}
	if body != "# Just markdown\n" {
		t.Errorf("body = %q", body)
	}
}

func TestSplitRejectsUnterminatedFrontmatter(t *testing.T) {
	if _, _, err := Split([]byte("---\nname: demo\nbody with no closing fence\n")); err == nil {
		t.Fatal("expected an error for unterminated frontmatter")
	}
}

// A body containing a --- rule must not be mistaken for the closing fence when
// the real fence came first.
func TestSplitStopsAtFirstClosingFence(t *testing.T) {
	_, body, err := Split([]byte("---\nname: demo\n---\nintro\n\n---\n\noutro\n"))
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if !strings.Contains(body, "outro") || !strings.HasPrefix(body, "intro") {
		t.Errorf("body = %q, want intro through outro", body)
	}
}

// Booleans must survive a round trip as booleans: quoting one would turn
// disable-model-invocation into a truthy string and silently change behaviour.
func TestRenderKeepsBooleansUnquoted(t *testing.T) {
	fm, _, err := Split([]byte("---\nname: demo\ndisable-model-invocation: true\n---\nbody\n"))
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	out := fm.Render()
	if !strings.Contains(out, "disable-model-invocation: true") {
		t.Errorf("Render = %q, want an unquoted true", out)
	}

	var probe struct {
		Disable bool `yaml:"disable-model-invocation"`
	}
	stripped := strings.TrimSuffix(strings.TrimPrefix(out, "---\n"), "---\n")
	if err := yaml.Unmarshal([]byte(stripped), &probe); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if !probe.Disable {
		t.Error("boolean did not survive the round trip")
	}
}

// Long descriptions must stay on one line; yaml.Marshal would fold them, and
// folded scalars trip simpler frontmatter readers.
func TestRenderKeepsLongValuesOnOneLine(t *testing.T) {
	long := "Use this skill whenever the user wants a structured prompt, an SPDD prompt or " +
		"REASONS canvas, a reviewable implementation blueprint, or asks to spec out a feature."
	var fm Frontmatter
	fm.Set("description", long)

	lines := strings.Split(strings.TrimSpace(fm.Render()), "\n")
	if len(lines) != 3 { // ---, description, ---
		t.Fatalf("Render produced %d lines, want 3:\n%s", len(lines), fm.Render())
	}

	var probe struct {
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(lines[1]), &probe); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if probe.Description != long {
		t.Errorf("description changed:\n got %q\nwant %q", probe.Description, long)
	}
}

// Values with YAML indicators must round-trip through quoting unharmed.
func TestRenderQuotesValuesWithIndicators(t *testing.T) {
	for _, value := range []string{
		"Implement a feature: strictly TDD",
		"[prompt id or path, e.g. 001]",
		"trailing space ",
		"true",
		"has # hash",
	} {
		var fm Frontmatter
		fm.Set("v", value)
		stripped := strings.TrimSuffix(strings.TrimPrefix(fm.Render(), "---\n"), "---\n")

		var probe struct {
			V string `yaml:"v"`
		}
		if err := yaml.Unmarshal([]byte(stripped), &probe); err != nil {
			t.Fatalf("re-parse %q: %v\nrendered: %s", value, err, stripped)
		}
		if probe.V != value {
			t.Errorf("round trip changed %q to %q", value, probe.V)
		}
	}
}

func TestRetainDropsUnknownKeysAndReportsThem(t *testing.T) {
	fm, _, err := Split([]byte("---\nname: a\nargument-hint: b\ndescription: c\ndisable-model-invocation: true\n---\nbody\n"))
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	dropped := fm.Retain("name", "description")
	if len(dropped) != 2 || dropped[0] != "argument-hint" || dropped[1] != "disable-model-invocation" {
		t.Errorf("dropped = %v, want [argument-hint disable-model-invocation] in file order", dropped)
	}
	if fm.Has("argument-hint") {
		t.Error("argument-hint survived Retain")
	}
	if !fm.Has("name") || !fm.Has("description") {
		t.Error("Retain removed a permitted key")
	}
}

// Clone must not share backing storage, or editing one target's frontmatter
// would corrupt the next target's.
func TestCloneIsIndependent(t *testing.T) {
	var original Frontmatter
	original.Set("name", "demo")
	original.Set("tools", "Read, Grep")

	clone := original.Clone()
	clone.Delete("tools")
	clone.Set("name", "changed")

	if !original.Has("tools") {
		t.Error("Delete on the clone removed a key from the original")
	}
	if got, _ := original.Get("name"); got != "demo" {
		t.Errorf("original name = %q, want demo", got)
	}
}

func TestDocumentOmitsFenceWhenNoFrontmatter(t *testing.T) {
	got := string(Document(Frontmatter{}, "just a body\n"))
	if strings.HasPrefix(got, "---") {
		t.Errorf("Document = %q, want no frontmatter fence", got)
	}
}

func TestIsTrue(t *testing.T) {
	fm, _, err := Split([]byte("---\na: true\nb: false\nc: hello\n---\nx\n"))
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if !fm.IsTrue("a") {
		t.Error("a should be true")
	}
	if fm.IsTrue("b") || fm.IsTrue("c") || fm.IsTrue("missing") {
		t.Error("only a should be true")
	}
}
