// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package config

import "testing"

func TestVersionFromBuildInfo(t *testing.T) {
	for _, tc := range []struct {
		name, current, main, want string
	}{
		// The bug: `go install …@v4.6.7` applies no ldflags.
		{"go install of a tag", defaultVersion, "v4.6.7", "4.6.7"},
		{"pseudo-version", defaultVersion, "v4.6.8-0.20261002181500-8f32f72bb3a8", "4.6.8-0.20261002181500-8f32f72bb3a8"},
		{"dirty vcs build", defaultVersion, "v4.6.8+dirty", "4.6.8+dirty"},
		{"devel build keeps the default", defaultVersion, "(devel)", defaultVersion},
		{"no build info", defaultVersion, "", defaultVersion},
		// A release sets Version with -X; build info must never override it.
		{"ldflag wins", "4.6.8", "v9.9.9", "4.6.8"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := versionFromBuildInfo(tc.current, tc.main); got != tc.want {
				t.Errorf("versionFromBuildInfo(%q, %q) = %q, want %q", tc.current, tc.main, got, tc.want)
			}
		})
	}
}
