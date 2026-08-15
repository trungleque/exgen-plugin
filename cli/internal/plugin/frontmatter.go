package plugin

import (
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Frontmatter is an ordered YAML mapping lifted from the head of a component
// file. Order is preserved so that rewritten files stay diffable against their
// source, and values keep their original YAML node so that a `true` does not
// silently become the string "true" on the way out.
type Frontmatter struct{ items []fmItem }

type fmItem struct {
	key  string
	node *yaml.Node
}

// Split separates YAML frontmatter from the markdown body. A file with no
// frontmatter is not an error: it yields an empty Frontmatter and the whole
// file as the body.
func Split(src []byte) (Frontmatter, string, error) {
	text := strings.ReplaceAll(string(src), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return Frontmatter{}, text, nil
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return Frontmatter{}, text, &ParseError{Msg: "unterminated frontmatter: no closing ---"}
	}

	head := strings.Join(lines[1:end], "\n")
	body := strings.TrimPrefix(strings.Join(lines[end+1:], "\n"), "\n")

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(head), &doc); err != nil {
		return Frontmatter{}, body, &ParseError{Msg: "invalid frontmatter YAML: " + err.Error()}
	}
	if len(doc.Content) == 0 {
		return Frontmatter{}, body, nil
	}
	mapping := doc.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return Frontmatter{}, body, &ParseError{Msg: "frontmatter must be a YAML mapping"}
	}

	var f Frontmatter
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		f.items = append(f.items, fmItem{key: mapping.Content[i].Value, node: mapping.Content[i+1]})
	}
	return f, body, nil
}

// ParseError describes a malformed component file.
type ParseError struct{ Msg string }

func (e *ParseError) Error() string { return e.Msg }

// Get returns the scalar value for key.
func (f Frontmatter) Get(key string) (string, bool) {
	for _, it := range f.items {
		if it.key == key {
			return it.node.Value, true
		}
	}
	return "", false
}

// Has reports whether key is present.
func (f Frontmatter) Has(key string) bool {
	_, ok := f.Get(key)
	return ok
}

// IsTrue reports whether key holds a truthy scalar.
func (f Frontmatter) IsTrue(key string) bool {
	v, ok := f.Get(key)
	return ok && (v == "true" || v == "yes" || v == "on")
}

// Keys lists the keys in file order.
func (f Frontmatter) Keys() []string {
	keys := make([]string, 0, len(f.items))
	for _, it := range f.items {
		keys = append(keys, it.key)
	}
	return keys
}

// Len reports the number of keys.
func (f Frontmatter) Len() int { return len(f.items) }

// Clone returns a deep-enough copy: the item slice is fresh, so Set and Delete
// on the copy cannot disturb the original's ordering.
func (f Frontmatter) Clone() Frontmatter {
	out := Frontmatter{items: make([]fmItem, len(f.items))}
	copy(out.items, f.items)
	return out
}

// Set replaces key in place, or appends it if absent.
func (f *Frontmatter) Set(key, value string) {
	node := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
	for i := range f.items {
		if f.items[i].key == key {
			f.items[i].node = node
			return
		}
	}
	f.items = append(f.items, fmItem{key: key, node: node})
}

// SetBool sets key to an unquoted YAML boolean.
func (f *Frontmatter) SetBool(key string, value bool) {
	node := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(value)}
	for i := range f.items {
		if f.items[i].key == key {
			f.items[i].node = node
			return
		}
	}
	f.items = append(f.items, fmItem{key: key, node: node})
}

// Delete removes key if present and reports whether it was there.
func (f *Frontmatter) Delete(key string) bool {
	for i := range f.items {
		if f.items[i].key == key {
			f.items = append(f.items[:i], f.items[i+1:]...)
			return true
		}
	}
	return false
}

// Retain drops every key not in allow, returning the dropped keys in file
// order so callers can report exactly what a target could not represent.
func (f *Frontmatter) Retain(allow ...string) []string {
	permitted := make(map[string]bool, len(allow))
	for _, k := range allow {
		permitted[k] = true
	}
	var dropped []string
	kept := f.items[:0:0]
	for _, it := range f.items {
		if permitted[it.key] {
			kept = append(kept, it)
			continue
		}
		dropped = append(dropped, it.key)
	}
	f.items = kept
	return dropped
}

// Render emits the frontmatter block, delimiters included, or "" when empty.
//
// Scalars are written on a single line: yaml.Marshal would fold long values
// like a skill description across lines, which is valid YAML but trips naive
// frontmatter readers in the wild. Anything that is not plainly safe is
// double-quoted, which is always valid.
func (f Frontmatter) Render() string {
	if len(f.items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("---\n")
	for _, it := range f.items {
		if it.node.Kind == yaml.ScalarNode {
			b.WriteString(it.key)
			b.WriteString(": ")
			b.WriteString(encodeScalar(it.node))
			b.WriteString("\n")
			continue
		}
		nested, err := yaml.Marshal(it.node)
		if err != nil {
			continue
		}
		b.WriteString(it.key)
		b.WriteString(":\n")
		for _, line := range strings.Split(strings.TrimRight(string(nested), "\n"), "\n") {
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	b.WriteString("---\n")
	return b.String()
}

// plainSafe matches values that need no quoting. The character class excludes
// every YAML indicator that would change the meaning of a plain scalar — most
// importantly ':' and '#' — while allowing commas so that a comma-separated
// list like "find, grep, ls" stays readable.
var plainSafe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9 ,_./+-]*[A-Za-z0-9_./+-]$|^[A-Za-z]$`)

func encodeScalar(node *yaml.Node) string {
	// Non-string tags carry meaning: quoting `true` would turn a bool into a
	// string and silently disable flags like disable-model-invocation.
	switch node.Tag {
	case "!!bool", "!!int", "!!float", "!!null":
		return node.Value
	}
	v := node.Value
	if v == "" {
		return `""`
	}
	switch strings.ToLower(v) {
	case "true", "false", "null", "yes", "no", "on", "off", "~":
		return strconv.Quote(v)
	}
	if plainSafe.MatchString(v) {
		return v
	}
	return strconv.Quote(v)
}

// Document reassembles a component file from frontmatter and body.
func Document(f Frontmatter, body string) []byte {
	head := f.Render()
	body = strings.TrimLeft(body, "\n")
	if head == "" {
		return []byte(body)
	}
	if body == "" {
		return []byte(head)
	}
	return []byte(head + "\n" + body)
}
