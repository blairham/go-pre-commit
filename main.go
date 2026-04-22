package main

import (
	"os"
	"runtime/debug"

	"github.com/blairham/go-pre-commit/v4/internal/cli"
)

// Build-time variables set via ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func init() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	if version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}

	if commit == "none" {
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				if len(s.Value) >= 7 {
					commit = s.Value[:7]
				}
			case "vcs.time":
				if date == "unknown" {
					date = s.Value
				}
			}
		}
	}
}

func main() {
	os.Exit(cli.Run(os.Args[1:], cli.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}))
}
