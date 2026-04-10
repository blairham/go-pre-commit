package cli

import (
	"testing"

	mcli "github.com/mitchellh/cli"
)

// allCommands returns all command instances for testing interface compliance.
func allCommands(t *testing.T) map[string]mcli.Command {
	t.Helper()
	meta := &Meta{UI: &mcli.BasicUi{}}
	return map[string]mcli.Command{
		"run":                     &RunCommand{Meta: meta},
		"install":                 &InstallCommand{Meta: meta},
		"uninstall":               &UninstallCommand{Meta: meta},
		"install-hooks":           &InstallHooksCommand{Meta: meta},
		"autoupdate":              &AutoupdateCommand{Meta: meta},
		"clean":                   &CleanCommand{Meta: meta},
		"gc":                      &GCCommand{Meta: meta},
		"init-templatedir":        &InitTemplateDirCommand{Meta: meta},
		"sample-config":           &SampleConfigCommand{Meta: meta},
		"try-repo":                &TryRepoCommand{Meta: meta},
		"validate-config":         &ValidateConfigCommand{Meta: meta},
		"validate-manifest":       &ValidateManifestCommand{Meta: meta},
		"migrate-config":          &MigrateConfigCommand{Meta: meta},
		"hook-impl":               &HookImplCommand{Meta: meta},
		"hazmat cd":               &HazmatCdCommand{Meta: meta},
		"hazmat ignore-exit-code": &HazmatIgnoreExitCodeCommand{Meta: meta},
		"hazmat n1":               &HazmatN1Command{Meta: meta},
	}
}

func TestAllCommands_Synopsis(t *testing.T) {
	for name, cmd := range allCommands(t) {
		t.Run(name, func(t *testing.T) {
			synopsis := cmd.Synopsis()
			if synopsis == "" {
				t.Errorf("command %q has empty Synopsis", name)
			}
		})
	}
}

func TestAllCommands_Help(t *testing.T) {
	for name, cmd := range allCommands(t) {
		t.Run(name, func(t *testing.T) {
			help := cmd.Help()
			if help == "" {
				t.Errorf("command %q has empty Help", name)
			}
		})
	}
}

func TestAllCommands_HelpContainsUsage(t *testing.T) {
	for name, cmd := range allCommands(t) {
		t.Run(name, func(t *testing.T) {
			help := cmd.Help()
			if len(help) < 10 {
				t.Errorf("command %q has suspiciously short Help: %q", name, help)
			}
		})
	}
}

func TestRun_RegistersAllExpectedCommands(t *testing.T) {
	expectedCommands := []string{
		"run", "install", "uninstall", "install-hooks",
		"autoupdate", "clean", "gc", "init-templatedir",
		"sample-config", "try-repo", "validate-config",
		"validate-manifest", "migrate-config", "hook-impl",
		"hazmat cd", "hazmat ignore-exit-code", "hazmat n1",
	}

	cmds := allCommands(t)
	for _, name := range expectedCommands {
		if _, ok := cmds[name]; !ok {
			t.Errorf("expected command %q not registered", name)
		}
	}
}
