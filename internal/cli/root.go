package cli

import (
	"fmt"
	"os"

	mcli "github.com/mitchellh/cli"

	"github.com/blairham/go-pre-commit/internal/config"
)

// Run creates the CLI application and executes the command specified by args.
func Run(args []string) int {
	ui := &mcli.BasicUi{
		Reader:      os.Stdin,
		Writer:      os.Stdout,
		ErrorWriter: os.Stderr,
	}

	meta := &Meta{UI: ui}

	c := &mcli.CLI{
		Name:    "pre-commit",
		Version: "pre-commit " + config.Version,
		Args:    args,
		Commands: map[string]mcli.CommandFactory{
			"run":               func() (mcli.Command, error) { return &RunCommand{Meta: meta}, nil },
			"install":           func() (mcli.Command, error) { return &InstallCommand{Meta: meta}, nil },
			"uninstall":         func() (mcli.Command, error) { return &UninstallCommand{Meta: meta}, nil },
			"install-hooks":     func() (mcli.Command, error) { return &InstallHooksCommand{Meta: meta}, nil },
			"autoupdate":        func() (mcli.Command, error) { return &AutoupdateCommand{Meta: meta}, nil },
			"clean":             func() (mcli.Command, error) { return &CleanCommand{Meta: meta}, nil },
			"gc":                func() (mcli.Command, error) { return &GCCommand{Meta: meta}, nil },
			"init-templatedir":  func() (mcli.Command, error) { return &InitTemplateDirCommand{Meta: meta}, nil },
			"sample-config":     func() (mcli.Command, error) { return &SampleConfigCommand{Meta: meta}, nil },
			"try-repo":          func() (mcli.Command, error) { return &TryRepoCommand{Meta: meta}, nil },
			"validate-config":   func() (mcli.Command, error) { return &ValidateConfigCommand{Meta: meta}, nil },
			"validate-manifest": func() (mcli.Command, error) { return &ValidateManifestCommand{Meta: meta}, nil },
			"migrate-config":    func() (mcli.Command, error) { return &MigrateConfigCommand{Meta: meta}, nil },
			"hook-impl":         func() (mcli.Command, error) { return &HookImplCommand{Meta: meta}, nil },
			"hazmat cd": func() (mcli.Command, error) {
				return &HazmatCdCommand{Meta: meta}, nil
			},
			"hazmat ignore-exit-code": func() (mcli.Command, error) {
				return &HazmatIgnoreExitCodeCommand{Meta: meta}, nil
			},
			"hazmat n1": func() (mcli.Command, error) {
				return &HazmatN1Command{Meta: meta}, nil
			},
		},
		HiddenCommands: []string{
			"hook-impl",
			"hazmat cd",
			"hazmat ignore-exit-code",
			"hazmat n1",
		},
	}

	exitCode, err := c.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return exitCode
}
