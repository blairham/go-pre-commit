// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/blairham/go-pre-commit/v4/internal/store"
)

// CleanCommand implements the "clean" command.
type CleanCommand struct {
	Meta *Meta
}

func (c *CleanCommand) Run(args []string) int {
	var opts ColorFlags
	if _, code, ok := parseArgs("clean", &opts, args, args); !ok {
		return code
	}

	s := store.New("")
	if err := s.Clean(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to clean: %v\n", err)
		return 1
	}
	fmt.Println("Cleaned pre-commit cache.")
	return 0
}

func (c *CleanCommand) Help() string {
	return strings.TrimSpace(`
Usage: pre-commit clean

  Remove the pre-commit cache directory and all cached hook repositories.
`)
}

func (c *CleanCommand) Synopsis() string {
	return "Clean out pre-commit files"
}
