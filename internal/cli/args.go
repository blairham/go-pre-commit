package cli

// expandFilesFlag rewrites the greedy `--files a b c` form into the repeated
// `--files a --files b --files c` form that go-flags understands.
//
// Upstream declares `--files` with argparse's `nargs='*'`, so one flag swallows
// every following argument until the next option. go-flags has no equivalent:
// a []string field means "repeat the flag", and it binds exactly one value per
// occurrence. Without this rewrite `run --files a b` parses as --files=a plus a
// stray positional, and the stray is then reported as a second hook id --
// "expected at most 1 argument, got 2" -- while upstream runs the hooks on both
// files. `--files` with several paths is the ordinary way to scope a run, so
// the difference shows up immediately in CI and editor integrations.
//
// The semantics below are argparse's, confirmed against upstream 4.6.2:
//
//	--files a b c      -> [a b c]        greedy to the end
//	hook --files a b   -> [a b]          a positional before the flag is kept
//	--files a b hook   -> [a b hook]     greedy really does swallow the hook id
//	--files=a b        -> [a], pos b     the joined form takes exactly one
//	--files a --other  -> [a]            stops at the next option
//	--files a -- b     -> [a], pos b     stops at the terminator
//	--files            -> []             empty, same as not passing it
//
// A value that begins with "-" therefore cannot be passed as a filename. That
// is argparse's limitation too, and matching it is the point.
func expandFilesFlag(args []string) []string {
	out := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Everything after a bare "--" is positional; leave the tail untouched.
		if arg == "--" {
			out = append(out, args[i:]...)
			return out
		}

		// Only the space-separated form is greedy. `--files=a` binds one value
		// and go-flags already handles it.
		if arg != "--files" {
			out = append(out, arg)
			continue
		}

		// Consume values until the next option or terminator.
		j := i + 1
		for ; j < len(args) && !isOptionLike(args[j]); j++ {
			out = append(out, "--files", args[j])
		}

		// `--files` with no values is an empty list upstream, which behaves the
		// same as omitting it. Dropping the flag reproduces that; keeping it
		// would make go-flags demand an argument.
		i = j - 1
	}

	return out
}

// isOptionLike reports whether arg would stop argparse from consuming further
// nargs='*' values. Any token beginning with "-" qualifies, including "--" and
// "-" itself.
func isOptionLike(arg string) bool {
	return len(arg) > 0 && arg[0] == '-'
}
