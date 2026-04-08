package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/blairham/go-pre-commit/internal/store"
	flags "github.com/jessevdk/go-flags"
)

// CleanCommand implements the "clean" command.
type CleanCommand struct {
	Meta *Meta
}

func (c *CleanCommand) Run(args []string) int {
	var opts GlobalFlags
	_, err := flags.ParseArgs(&opts, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
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
