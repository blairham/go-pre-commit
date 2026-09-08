package cli

import (
	"reflect"
	"testing"
)

// Every case here was confirmed against Python pre-commit 4.6.2's argparse
// before it was written down. `want` is the argument list go-flags must see in
// order to end up with the same (files, positionals) split upstream produces.
func TestExpandFilesFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "greedy to the end",
			args: []string{"--files", "a", "b", "c"},
			want: []string{"--files", "a", "--files", "b", "--files", "c"},
		},
		{
			name: "positional before the flag is preserved",
			args: []string{"hook", "--files", "a", "b"},
			want: []string{"hook", "--files", "a", "--files", "b"},
		},
		{
			name: "greedy swallows a trailing hook id, as upstream does",
			args: []string{"--files", "a", "b", "hook"},
			want: []string{"--files", "a", "--files", "b", "--files", "hook"},
		},
		{
			name: "joined form takes exactly one value",
			args: []string{"--files=a", "b"},
			want: []string{"--files=a", "b"},
		},
		{
			name: "stops at the next option",
			args: []string{"--files", "a", "--verbose"},
			want: []string{"--files", "a", "--verbose"},
		},
		{
			name: "stops at a short option",
			args: []string{"--files", "a", "-v"},
			want: []string{"--files", "a", "-v"},
		},
		{
			name: "stops at the terminator and keeps it",
			args: []string{"--files", "a", "--", "b"},
			want: []string{"--files", "a", "--", "b"},
		},
		{
			name: "no values yields an empty list, so the flag is dropped",
			args: []string{"--files"},
			want: []string{},
		},
		{
			name: "no values before another option",
			args: []string{"--files", "--verbose"},
			want: []string{"--verbose"},
		},
		{
			name: "nothing to do",
			args: []string{"hook", "--all-files"},
			want: []string{"hook", "--all-files"},
		},
		{
			name: "empty",
			args: []string{},
			want: []string{},
		},
		{
			name: "after a terminator the tail is untouched",
			args: []string{"--", "--files", "a", "b"},
			want: []string{"--", "--files", "a", "b"},
		},
		{
			name: "two occurrences each expand",
			args: []string{"--files", "a", "b", "--verbose", "--files", "c", "d"},
			want: []string{"--files", "a", "--files", "b", "--verbose", "--files", "c", "--files", "d"},
		},
		{
			name: "repeated flag form still works",
			args: []string{"--files", "a", "--files", "b"},
			want: []string{"--files", "a", "--files", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := expandFilesFlag(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("expandFilesFlag(%q)\n got %q\nwant %q", tt.args, got, tt.want)
			}
		})
	}
}
