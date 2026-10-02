// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"runtime/debug"
	"strings"
)

// defaultVersion is what Version holds when no ldflag set it. Releases and
// `make build` set it with -X; `go install module@version` applies no
// ldflags, so without the fallback below such a build reports this literal
// whatever version it was built from.
const defaultVersion = "4.6.0"

// Version is the current version of go-pre-commit, set via ldflags at build
// time, or from the module's build info when they were not applied.
var Version = defaultVersion

func init() {
	if info, ok := debug.ReadBuildInfo(); ok {
		Version = versionFromBuildInfo(Version, info.Main.Version)
	}
}

// versionFromBuildInfo returns the version to report. An ldflag-set value
// always wins; otherwise a real module version (a tag or pseudo-version, not
// "(devel)") replaces the compile-time default, in the same form the ldflag
// uses — no leading "v".
func versionFromBuildInfo(current, mainVersion string) string {
	if current != defaultVersion {
		return current
	}
	if mainVersion == "" || mainVersion == "(devel)" {
		return current
	}
	return strings.TrimPrefix(mainVersion, "v")
}
