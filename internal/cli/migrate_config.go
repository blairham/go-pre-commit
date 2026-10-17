// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	flags "github.com/jessevdk/go-flags"
	"gopkg.in/yaml.v3"

	"github.com/blairham/go-pre-commit/v4/internal/fatal"
	"github.com/blairham/go-pre-commit/v4/internal/pyyaml"
)

// MigrateConfigCommand implements the "migrate-config" command.
type MigrateConfigCommand struct {
	Meta *Meta
}

func (c *MigrateConfigCommand) Run(args []string) int {
	var opts GlobalFlags
	_, err := flags.ParseArgs(&opts, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if err = adjustArgsAndChdir(rootArgs{Config: &opts.Config}); err != nil {
		return fatalError(err)
	}

	return migrateConfigExit(opts.Config, false, func(s string) { fmt.Print(s) })
}

// migrateConfigExit runs migrateConfig and reports a failure the way
// upstream's error_handler does, returning the exit code.
func migrateConfigExit(path string, quiet bool, say func(string)) int {
	if err := migrateConfig(path, quiet, say); err != nil {
		return reportError(err)
	}
	return 0
}

// migrateConfig is upstream's commands/migrate_config.migrate_config: the
// list-to-map rewrite, then sha->rev, python_venv->python and the old stage
// names, each rewritten in place in the source text. Its messages go to
// say, which is upstream's print().
func migrateConfig(path string, quiet bool, say func(string)) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	// Python's text-mode open() reads any line ending as "\n" and writes
	// "\n" back, so a rewritten file comes out with "\n" line endings.
	orig := universalNewlines(string(b))

	if _, loadErr := pyyaml.Load(orig); loadErr != nil {
		// cfgv's validate_context("File ...") around the load, reraised
		// as InvalidConfigError.
		return &fatal.Error{
			Type: "InvalidConfigError",
			Msg:  fmt.Sprintf("\n==> File %s\n=====> %s", path, loadErr),
		}
	}

	contents, err := migrateMap(orig)
	if err != nil {
		return err
	}
	contents, err = migrateComposed(contents)
	if err != nil {
		return err
	}

	if contents != orig {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			return err
		}
		say("Configuration has been migrated.\n")
	} else if !quiet {
		say("Configuration is already migrated.\n")
	}
	return nil
}

func universalNewlines(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

func isHeaderLine(line string) bool {
	return strings.HasPrefix(line, "#") || strings.HasPrefix(line, "---") || strings.TrimSpace(line) == ""
}

// migrateMap is upstream's _migrate_map: a config that is a bare list of
// repos gains a "repos:" key above it, indented only if it has to be.
func migrateMap(contents string) (string, error) {
	n, err := pyyaml.Parse(contents)
	if err != nil {
		return "", err
	}
	if n == nil || n.Kind != yaml.SequenceNode {
		return contents, nil
	}
	lines := strings.SplitAfter(contents, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	i := 0
	for i < len(lines) && isHeaderLine(lines[i]) {
		i++
	}
	header := strings.Join(lines[:i], "")
	rest := strings.Join(lines[i:], "")

	trial := header + "repos:\n" + rest
	if _, err := pyyaml.Parse(trial); err == nil {
		return trial, nil
	}
	return header + "repos:\n" + indentLines(rest, "    "), nil
}

// indentLines is textwrap.indent: prefix every line that is not blank.
func indentLines(s, prefix string) string {
	var b strings.Builder
	for _, line := range strings.SplitAfter(s, "\n") {
		if strings.TrimSpace(line) != "" {
			b.WriteString(prefix)
		}
		b.WriteString(line)
	}
	return b.String()
}

type yamlStep func(*yaml.Node) []*yaml.Node

func mappingKey(k string) yamlStep {
	return func(n *yaml.Node) []*yaml.Node {
		var out []*yaml.Node
		if n.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(n.Content); i += 2 {
				if key := n.Content[i]; key.Kind == yaml.ScalarNode && key.Value == k {
					out = append(out, key)
				}
			}
		}
		return out
	}
}

func mappingValue(k string) yamlStep {
	return func(n *yaml.Node) []*yaml.Node {
		var out []*yaml.Node
		if n.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(n.Content); i += 2 {
				if key := n.Content[i]; key.Kind == yaml.ScalarNode && key.Value == k {
					out = append(out, deref(n.Content[i+1]))
				}
			}
		}
		return out
	}
}

func sequenceItem(n *yaml.Node) []*yaml.Node {
	if n.Kind != yaml.SequenceNode {
		return nil
	}
	out := make([]*yaml.Node, len(n.Content))
	for i, c := range n.Content {
		out[i] = deref(c)
	}
	return out
}

// deref follows an alias to the node it names, as PyYAML's composed tree
// holds the anchored node itself where an alias appears.
func deref(n *yaml.Node) *yaml.Node {
	for n.Kind == yaml.AliasNode && n.Alias != nil {
		n = n.Alias
	}
	return n
}

// matchNodes is upstream's yaml_rewrite.match: the scalar nodes the steps
// reach from root.
func matchNodes(root *yaml.Node, steps []yamlStep) []*yaml.Node {
	gen := []*yaml.Node{root}
	for _, step := range steps {
		var next []*yaml.Node
		for _, n := range gen {
			next = append(next, step(n)...)
		}
		gen = next
	}
	var out []*yaml.Node
	for _, n := range gen {
		if n.Kind == yaml.ScalarNode {
			out = append(out, n)
		}
	}
	return out
}

// rewrite replaces the source bytes [start, end) with text.
type rewrite struct {
	text       string
	start, end int
}

// migrateComposed is upstream's _migrate_composed: it finds the scalars to
// rename in the composed tree and splices each replacement over that
// scalar's source text, so the rest of the file is untouched.
func migrateComposed(contents string) (string, error) {
	root, err := pyyaml.Parse(contents)
	if err != nil {
		return "", err
	}
	if root == nil {
		return contents, nil
	}

	type pending struct {
		node *yaml.Node
		s    string
	}
	var todo []pending

	repo := []yamlStep{mappingValue("repos"), sequenceItem}
	hook := slices.Concat(repo, []yamlStep{mappingValue("hooks"), sequenceItem})

	// sha -> rev
	for _, n := range matchNodes(root, slices.Concat(repo, []yamlStep{mappingKey("sha")})) {
		todo = append(todo, pending{n, "rev"})
	}
	// python_venv -> python
	for _, n := range matchNodes(root, slices.Concat(hook, []yamlStep{mappingValue("language")})) {
		if n.Value == "python_venv" {
			todo = append(todo, pending{n, "python"})
		}
	}
	// stages
	stages := matchNodes(root, []yamlStep{mappingValue("default_stages"), sequenceItem})
	stages = append(stages, matchNodes(root, slices.Concat(hook, []yamlStep{mappingValue("stages"), sequenceItem}))...)
	for _, n := range stages {
		switch n.Value {
		case "commit", "push", "merge-commit":
			todo = append(todo, pending{n, "pre-" + n.Value})
		}
	}

	offsets := lineOffsets(contents)
	seen := map[*yaml.Node]bool{}
	var rewrites []rewrite
	for _, p := range todo {
		// An anchored node reached twice through aliases is one span of
		// source; it is rewritten once.
		if seen[p.node] {
			continue
		}
		seen[p.node] = true
		start, end, ok := scalarSpan(contents, offsets, p.node)
		if !ok {
			continue
		}
		rewrites = append(rewrites, rewrite{start: start, end: end, text: preserveStyle(p.node, p.s)})
	}

	sort.SliceStable(rewrites, func(i, j int) bool { return rewrites[i].start < rewrites[j].start })
	var b strings.Builder
	pos := 0
	for _, r := range rewrites {
		b.WriteString(contents[pos:r.start])
		b.WriteString(r.text)
		pos = r.end
	}
	b.WriteString(contents[pos:])
	return b.String(), nil
}

// preserveStyle is upstream's _preserve_style: s in the node's quotes.
func preserveStyle(n *yaml.Node, s string) string {
	switch {
	case n.Style&yaml.SingleQuotedStyle != 0:
		return "'" + s + "'"
	case n.Style&yaml.DoubleQuotedStyle != 0:
		return `"` + s + `"`
	}
	return s
}

// lineOffsets maps each line yaml.v3 counts to its byte offset in s.
func lineOffsets(s string) []int {
	offsets := []int{0}
	for i, r := range s {
		switch r {
		case '\n', '\u0085', ' ', ' ':
			offsets = append(offsets, i+len(string(r)))
		}
	}
	return offsets
}

// scalarSpan is the byte span PyYAML's start_mark and end_mark give a
// scalar: from its first property (anchor or tag) to the end of its text.
// A block scalar, or a node whose source is not where it should be, has no
// span and is left alone.
func scalarSpan(src string, offsets []int, n *yaml.Node) (start, end int, ok bool) {
	if n.Line < 1 || n.Line > len(offsets) || n.Column < 1 {
		return 0, 0, false
	}
	pos := offsets[n.Line-1]
	for col := 1; col < n.Column; col++ {
		if pos >= len(src) {
			return 0, 0, false
		}
		_, w := utf8.DecodeRuneInString(src[pos:])
		pos += w
	}
	start = pos

	// Skip properties: "&anchor" and "!tag", each followed by whitespace.
	for pos < len(src) && (src[pos] == '&' || src[pos] == '!') {
		for pos < len(src) && !strings.ContainsRune(" \t\n", rune(src[pos])) {
			pos++
		}
		for pos < len(src) && strings.ContainsRune(" \t\n", rune(src[pos])) {
			pos++
		}
	}

	switch {
	case n.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0:
		return 0, 0, false
	case n.Style&yaml.SingleQuotedStyle != 0:
		if pos >= len(src) || src[pos] != '\'' {
			return 0, 0, false
		}
		for i := pos + 1; i < len(src); i++ {
			if src[i] != '\'' {
				continue
			}
			if i+1 < len(src) && src[i+1] == '\'' {
				i++
				continue
			}
			return start, i + 1, true
		}
		return 0, 0, false
	case n.Style&yaml.DoubleQuotedStyle != 0:
		if pos >= len(src) || src[pos] != '"' {
			return 0, 0, false
		}
		for i := pos + 1; i < len(src); i++ {
			switch src[i] {
			case '\\':
				i++
			case '"':
				return start, i + 1, true
			}
		}
		return 0, 0, false
	}
	if !strings.HasPrefix(src[pos:], n.Value) {
		return 0, 0, false
	}
	return start, pos + len(n.Value), true
}

func (c *MigrateConfigCommand) Help() string {
	return strings.TrimSpace(`
Usage: pre-commit migrate-config [options]

  Migrate list configuration to new map configuration.

Options:

  -c, --config=FILE   Path to alternate config file.
      --color=MODE    Whether to use color (auto, always, never).
`)
}

func (c *MigrateConfigCommand) Synopsis() string {
	return "Migrate list configuration to new map configuration"
}

// pyStdout is what Python's print() does to sys.stdout: on a terminal a
// line goes out at once, but into a pipe or a file it waits in the text
// buffer until the process exits -- after everything upstream writes with
// output.write_line, which goes straight to sys.stdout.buffer.
type pyStdout struct {
	pending strings.Builder
	tty     bool
}

func newPyStdout() *pyStdout {
	fi, err := os.Stdout.Stat()
	return &pyStdout{tty: err == nil && fi.Mode()&os.ModeCharDevice != 0}
}

func (p *pyStdout) Print(s string) {
	if p.tty {
		fmt.Print(s)
		return
	}
	p.pending.WriteString(s)
}

// Flush is the interpreter's exit: whatever print() buffered goes out.
func (p *pyStdout) Flush() {
	fmt.Print(p.pending.String())
	p.pending.Reset()
}
