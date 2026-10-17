// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	flags "github.com/jessevdk/go-flags"

	"github.com/blairham/go-pre-commit/v4/internal/config"
)

// This file reports argument errors the way upstream's argparse parser
// does (#175): the parser's usage block and "<prog>: error: <message>" on
// stderr, and exit 2. Parsing itself stays with go-flags; what is ported is
// argparse's view of each command -- the actions main.py declares, in its
// order -- which the usage line and the error wording are built from.
//
// Errors raised while a subcommand consumes its arguments (a missing value,
// an invalid choice, a required argument) are reported by that subcommand's
// parser. Arguments nothing consumed are reported afterwards by the
// top-level parser as "unrecognized arguments", because that is where
// argparse's parse_args finds them.

// argAction is one argparse action, as far as usage and errors need it.
//
//   - opts: option strings in upstream's order; none for a positional
//   - dest: a positional's name, or the dest an option's metavar derives from
//   - metavar: an explicit metavar
//   - nargs: "" one value; "0" a flag; "?" or "*"; "..." REMAINDER; "parser" subcommands
//   - group: in the (one) mutually exclusive group
//   - check: argparse's type= and choices= for one value, returning the
//     message after "argument <opts>: ", or ""
type argAction struct {
	check    func(string) string
	dest     string
	metavar  string
	nargs    string
	opts     []string
	choices  []string
	required bool
	group    bool
}

// argSpec is an argparse parser: its prog and its actions.
type argSpec struct {
	prog    string
	actions []argAction
}

func (a argAction) label() string {
	if len(a.opts) == 0 {
		return a.dest
	}
	return strings.Join(a.opts, "/")
}

func (a argAction) metavarFor() string {
	switch {
	case a.metavar != "":
		return a.metavar
	case a.choices != nil:
		return "{" + strings.Join(a.choices, ",") + "}"
	case len(a.opts) == 0:
		return a.dest
	}
	return strings.ToUpper(a.dest)
}

// formatArgs is argparse's _format_args.
func (a argAction) formatArgs() string {
	m := a.metavarFor()
	switch a.nargs {
	case "?":
		return "[" + m + "]"
	case "*":
		return "[" + m + " ...]"
	case "...":
		return "..."
	case "parser":
		return m + " ..."
	}
	return m
}

// usageParts is argparse's _get_actions_usage_parts_with_split: the
// optionals' parts, then the positionals', with the mutually exclusive
// group bracketed and separated by " |".
func (s argSpec) usageParts() (parts []string, posStart int) {
	var optionals, positionals []argAction
	for _, a := range s.actions {
		if len(a.opts) > 0 {
			optionals = append(optionals, a)
		} else {
			positionals = append(positionals, a)
		}
	}
	all := append(slices.Clone(optionals), positionals...)
	groupStart, groupEnd := -1, -1
	for i, a := range all {
		var part string
		if len(a.opts) == 0 {
			part = a.formatArgs()
		} else {
			part = a.opts[0]
			if a.nargs != "0" {
				part += " " + a.formatArgs()
			}
			if !a.required && !a.group {
				part = "[" + part + "]"
			}
		}
		if a.group {
			if groupStart < 0 {
				groupStart = i
			}
			groupEnd = i + 1
		}
		parts = append(parts, part)
	}
	if groupStart >= 0 {
		parts[groupStart] = "[" + parts[groupStart]
		for i := groupStart; i < groupEnd-1; i++ {
			parts[i] += " |"
		}
		parts[groupEnd-1] += "]"
	}
	return parts, len(optionals)
}

// usage is argparse's format_usage at the given width: "usage: ..." and a
// newline, wrapped as HelpFormatter._format_usage wraps it.
func (s argSpec) usage(width int) string {
	const prefix = "usage: "
	parts, posStart := s.usageParts()
	full := strings.Join(append([]string{s.prog}, parts...), " ")
	textWidth := width
	if len(prefix)+len(full) <= textWidth {
		return prefix + full + "\n"
	}

	optParts, posParts := parts[:posStart], parts[posStart:]
	getLines := func(parts []string, indent string, withPrefix bool) []string {
		var lines, line []string
		lineLen := len(indent) - 1
		if withPrefix {
			lineLen = len(prefix) - 1
		}
		for _, part := range parts {
			if lineLen+1+len(part) > textWidth && len(line) > 0 {
				lines = append(lines, indent+strings.Join(line, " "))
				line = nil
				lineLen = len(indent) - 1
			}
			line = append(line, part)
			lineLen += len(part) + 1
		}
		if len(line) > 0 {
			lines = append(lines, indent+strings.Join(line, " "))
		}
		if withPrefix && len(lines) > 0 {
			lines[0] = lines[0][len(indent):]
		}
		return lines
	}

	var lines []string
	if float64(len(prefix)+len(s.prog)) <= 0.75*float64(textWidth) {
		indent := strings.Repeat(" ", len(prefix)+len(s.prog)+1)
		switch {
		case len(optParts) > 0:
			lines = getLines(append([]string{s.prog}, optParts...), indent, true)
			lines = append(lines, getLines(posParts, indent, false)...)
		case len(posParts) > 0:
			lines = getLines(append([]string{s.prog}, posParts...), indent, true)
		default:
			lines = []string{s.prog}
		}
	} else {
		indent := strings.Repeat(" ", len(prefix))
		lines = getLines(append(slices.Clone(optParts), posParts...), indent, false)
		if len(lines) > 1 {
			lines = append(getLines(optParts, indent, false), getLines(posParts, indent, false)...)
		}
		lines = append([]string{s.prog}, lines...)
	}
	return prefix + strings.Join(lines, "\n") + "\n"
}

// argparseWidth is HelpFormatter's width: shutil.get_terminal_size's
// columns (COLUMNS, else the terminal on stdout, else 80), less 2.
func argparseWidth() int {
	if n, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && n > 0 {
		return n - 2
	}
	if n := stdoutColumns(); n > 0 {
		return n - 2
	}
	return 80 - 2
}

// argparseError is ArgumentParser.error: the usage, the message, exit 2.
func argparseError(s argSpec, msg string) int {
	fmt.Fprint(os.Stderr, s.usage(argparseWidth()))
	fmt.Fprintf(os.Stderr, "%s: error: %s\n", s.prog, msg)
	return 2
}

// --- the parsers main.py builds ---

// commandNames is the top-level parser's subcommand choices, in order.
var commandNames = []string{
	"autoupdate", "clean", "gc", "hazmat", "init-templatedir", "install",
	"install-hooks", "migrate-config", "run", "sample-config", "try-repo",
	"uninstall", "validate-config", "validate-manifest", "help", "hook-impl",
}

var topLevelSpec = argSpec{prog: "pre-commit", actions: []argAction{
	{opts: []string{"-h", "--help"}, nargs: "0"},
	{opts: []string{"-V", "--version"}, nargs: "0"},
	{dest: "command", metavar: "{" + strings.Join(commandNames, ",") + "}", nargs: "parser"},
}}

func hookTypeChoices() []string {
	var out []string
	for _, h := range config.AllHookTypes() {
		out = append(out, string(h))
	}
	slices.Sort(out)
	return out
}

func stageChoices() []string {
	return append(hookTypeChoices(), string(config.StageManual))
}

// pyReprList is argparse's ', '.join(map(repr, choices)), which the
// "choose from" list uses (the Python 3.13 the parity job runs).
func pyReprList(choices []string) string {
	quoted := make([]string, len(choices))
	for i, c := range choices {
		quoted[i] = config.PyRepr(c)
	}
	return strings.Join(quoted, ", ")
}

// checkChoice is argparse's _check_value.
func checkChoice(choices []string) func(string) string {
	return func(v string) string {
		if slices.Contains(choices, v) {
			return ""
		}
		return fmt.Sprintf("invalid choice: %s (choose from %s)", config.PyRepr(v), pyReprList(choices))
	}
}

var (
	helpAction  = argAction{opts: []string{"-h", "--help"}, nargs: "0"}
	colorAction = argAction{
		opts: []string{"--color"}, dest: "color", metavar: "{auto,always,never}",
		// type=color.use_color, which raises ValueError for anything else.
		check: func(v string) string {
			if v == "auto" || v == "always" || v == "never" {
				return ""
			}
			return "invalid use_color value: " + config.PyRepr(v)
		},
	}
	configAction   = argAction{opts: []string{"-c", "--config"}, dest: "config"}
	hookTypeAction = argAction{
		opts: []string{"-t", "--hook-type"}, choices: hookTypeChoices(),
		check: checkChoice(hookTypeChoices()),
	}
)

// runOptions is _add_run_options.
func runOptions() []argAction {
	opt := func(dest string, opts ...string) argAction { return argAction{opts: opts, dest: dest} }
	stages := stageChoices()
	return []argAction{
		{dest: "hook", nargs: "?"},
		{opts: []string{"--verbose", "-v"}, nargs: "0"},
		{opts: []string{"--all-files", "-a"}, nargs: "0", group: true},
		{opts: []string{"--files"}, dest: "files", nargs: "*", group: true},
		{opts: []string{"--show-diff-on-failure"}, nargs: "0"},
		{opts: []string{"--fail-fast"}, nargs: "0"},
		{
			opts: []string{"--hook-stage"}, choices: stages,
			// type=transform_stage runs first, then choices.
			check: func(v string) string {
				if to, ok := map[string]string{"commit": "pre-commit", "merge-commit": "pre-merge-commit", "push": "pre-push"}[v]; ok {
					v = to
				}
				return checkChoice(stages)(v)
			},
		},
		opt("remote_branch", "--remote-branch"),
		opt("local_branch", "--local-branch"),
		opt("from_ref", "--from-ref", "--source", "-s"),
		opt("to_ref", "--to-ref", "--origin", "-o"),
		opt("pre_rebase_upstream", "--pre-rebase-upstream"),
		opt("pre_rebase_branch", "--pre-rebase-branch"),
		opt("commit_msg_filename", "--commit-msg-filename"),
		opt("prepare_commit_message_source", "--prepare-commit-message-source"),
		opt("commit_object_name", "--commit-object-name"),
		opt("remote_name", "--remote-name"),
		opt("remote_url", "--remote-url"),
		opt("checkout_type", "--checkout-type"),
		opt("is_squash_merge", "--is-squash-merge"),
		opt("rewrite_command", "--rewrite-command"),
	}
}

// commandSpecs is each subparser main.py adds, keyed by command name.
var commandSpecs = map[string]argSpec{
	"autoupdate": {prog: "pre-commit autoupdate", actions: []argAction{
		helpAction, colorAction, configAction,
		{opts: []string{"--bleeding-edge"}, nargs: "0"},
		{opts: []string{"--freeze"}, nargs: "0"},
		{opts: []string{"--repo"}, metavar: "REPO"},
		{opts: []string{"-j", "--jobs"}, dest: "jobs", check: func(v string) string {
			if _, err := strconv.Atoi(strings.TrimSpace(v)); err != nil {
				return "invalid int value: " + config.PyRepr(v)
			}
			return ""
		}},
	}},
	"clean": {prog: "pre-commit clean", actions: []argAction{helpAction, colorAction}},
	"gc":    {prog: "pre-commit gc", actions: []argAction{helpAction, colorAction}},
	"init-templatedir": {prog: "pre-commit init-templatedir", actions: []argAction{
		helpAction, colorAction, configAction,
		{dest: "directory"},
		{opts: []string{"--no-allow-missing-config"}, nargs: "0"},
		hookTypeAction,
	}},
	"install": {prog: "pre-commit install", actions: []argAction{
		helpAction, colorAction, configAction,
		{opts: []string{"-f", "--overwrite"}, nargs: "0"},
		{opts: []string{"--install-hooks"}, nargs: "0"},
		hookTypeAction,
		{opts: []string{"--allow-missing-config"}, nargs: "0"},
	}},
	"install-hooks":  {prog: "pre-commit install-hooks", actions: []argAction{helpAction, colorAction, configAction}},
	"migrate-config": {prog: "pre-commit migrate-config", actions: []argAction{helpAction, colorAction, configAction}},
	"run": {prog: "pre-commit run", actions: append(
		[]argAction{helpAction, colorAction, configAction}, runOptions()...,
	)},
	"sample-config": {prog: "pre-commit sample-config", actions: []argAction{helpAction, colorAction}},
	"try-repo": {prog: "pre-commit try-repo", actions: append([]argAction{
		helpAction, colorAction, configAction,
		{dest: "repo"},
		{opts: []string{"--ref", "--rev"}, dest: "ref"},
	}, runOptions()...)},
	"uninstall": {prog: "pre-commit uninstall", actions: []argAction{
		helpAction, colorAction, configAction, hookTypeAction,
	}},
	"validate-config": {prog: "pre-commit validate-config", actions: []argAction{
		helpAction, colorAction, {dest: "filenames", nargs: "*"},
	}},
	"validate-manifest": {prog: "pre-commit validate-manifest", actions: []argAction{
		helpAction, colorAction, {dest: "filenames", nargs: "*"},
	}},
	"hook-impl": {prog: "pre-commit hook-impl", actions: []argAction{
		helpAction, colorAction, configAction,
		{opts: []string{"--hook-type"}, dest: "hook_type", required: true},
		{opts: []string{"--hook-dir"}, dest: "hook_dir"},
		{opts: []string{"--skip-on-missing-config"}, nargs: "0"},
		{dest: "rest", nargs: "..."},
	}},
}

func (s argSpec) find(opt string) (argAction, bool) {
	for _, a := range s.actions {
		if slices.Contains(a.opts, opt) {
			return a, true
		}
	}
	return argAction{}, false
}

// --- parsing ---

// flagNameRe pulls the option out of go-flags' "`-c, --config'" quoting.
var (
	flagNameRe = regexp.MustCompile("`(?:-(\\w), )?(-{1,2}[\\w-]+)'")
)

// parseArgs parses a subcommand's arguments into opts with go-flags and
// reports anything argparse would reject as argparse does. It returns the
// positionals, or ok == false and the exit code to return.
//
// args is what the command received; parseArgs is what go-flags is handed
// (run and try-repo expand `--files a b` first).
func parseArgs(cmd string, opts any, args, parseArgs []string) (positionals []string, code int, ok bool) {
	spec := commandSpecs[cmd]

	head, tail := parseArgs, []string(nil)
	if i := slices.Index(parseArgs, "--"); i >= 0 {
		head, tail = parseArgs[:i], parseArgs[i+1:]
	}

	// What argparse checks as it consumes each argument, in order.
	if msg := checkValues(spec, args); msg != "" {
		return nil, argparseError(spec, msg), false
	}

	p := flags.NewParser(opts, flags.HelpFlag|flags.IgnoreUnknown)
	rest, err := p.ParseArgs(head)
	if err != nil {
		var fe *flags.Error
		if errors.As(err, &fe) && fe.Type == flags.ErrHelp {
			fmt.Println(fe.Message)
			return nil, 0, false
		}
		return nil, argparseError(spec, flagsErrorMessage(spec, err)), false
	}

	// Split what go-flags left into positionals and the arguments argparse
	// would leave unrecognized, in the order they were given.
	maxPos, minPos := 0, 0
	var required []string
	for _, a := range spec.actions {
		if len(a.opts) > 0 {
			continue
		}
		switch a.nargs {
		case "":
			maxPos++
			minPos++
		case "?":
			maxPos++
		case "*", "...":
			maxPos = -1
		}
	}
	var unrecognized []string
	for _, tok := range rest {
		switch {
		case len(tok) > 1 && tok[0] == '-':
			unrecognized = append(unrecognized, tok)
		case maxPos < 0 || len(positionals) < maxPos:
			positionals = append(positionals, tok)
		default:
			unrecognized = append(unrecognized, tok)
		}
	}
	for _, tok := range tail {
		if maxPos < 0 || len(positionals) < maxPos {
			positionals = append(positionals, tok)
		} else {
			unrecognized = append(unrecognized, tok)
		}
	}

	if len(positionals) < minPos {
		n := 0
		for _, a := range spec.actions {
			if len(a.opts) == 0 && a.nargs == "" {
				if n >= len(positionals) {
					required = append(required, a.dest)
				}
				n++
			}
		}
		return nil, argparseError(spec, "the following arguments are required: "+strings.Join(required, ", ")), false
	}
	if len(unrecognized) > 0 {
		return nil, argparseError(topLevelSpec, "unrecognized arguments: "+strings.Join(unrecognized, " ")), false
	}
	return positionals, 0, true
}

// flagsErrorMessage is argparse's message for what go-flags still rejects
// once checkValues has passed: a required option that is missing.
func flagsErrorMessage(spec argSpec, err error) string {
	var fe *flags.Error
	if !errors.As(err, &fe) {
		return err.Error()
	}
	if m := flagNameRe.FindStringSubmatch(fe.Message); m != nil && fe.Type == flags.ErrRequired {
		if a, ok := spec.find(m[2]); ok {
			return "the following arguments are required: " + a.opts[0]
		}
	}
	return fe.Message
}

// checkValues runs the type= and choices= checks argparse applies to each
// value as it is consumed, and its mutually exclusive group check, over the
// arguments in order. It returns the first message, or "".
func checkValues(spec argSpec, args []string) string {
	var groupSeen *argAction
	for i := 0; i < len(args); i++ {
		tok := args[i]
		if tok == "--" {
			return ""
		}
		if len(tok) < 2 || tok[0] != '-' {
			continue
		}
		name, value, hasValue := strings.Cut(tok, "=")
		a, ok := spec.find(name)
		if !ok {
			continue
		}
		if a.nargs == "0" && hasValue {
			return "argument " + a.label() + ": ignored explicit argument " + config.PyRepr(value)
		}
		if a.group {
			if groupSeen != nil && groupSeen.label() != a.label() {
				return "argument " + a.label() + ": not allowed with argument " + groupSeen.label()
			}
			seen := a
			groupSeen = &seen
		}
		if a.nargs == "0" || a.nargs == "*" {
			continue
		}
		if !hasValue {
			if i+1 >= len(args) || isOptionLike(args[i+1]) {
				if a.nargs == "" {
					return "argument " + a.label() + ": expected one argument"
				}
				continue
			}
			i++
			value = args[i]
		}
		if a.check != nil {
			if msg := a.check(value); msg != "" {
				return "argument " + a.label() + ": " + msg
			}
		}
	}
	return ""
}
