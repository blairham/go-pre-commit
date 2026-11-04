// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf16"
)

// The files upstream's _hook_install leaves in an environment once it is
// installed and healthy, and that _hook_installed looks for. Writing and
// reading the same ones is what lets an environment either tool installed
// serve the other from a shared store.
const (
	installStateV1 = ".install_state_v1"
	installStateV2 = ".install_state_v2"
	// legacyInstallState is the file this port wrote before it wrote
	// upstream's: install_state_v2, holding the hook's InstallKey.
	legacyInstallState = "install_state_v2"
)

// installStateMatches is the state half of upstream's _hook_installed: the
// environment at envPath has .install_state_v2, or a .install_state_v1 that
// records h's additional_dependencies. An environment this port installed
// before it wrote upstream's files counts too, by its own file, so upgrading
// reinstalls nothing -- a CI cache restored from an older release included.
func installStateMatches(envPath string, h *Hook) bool {
	if _, err := os.Stat(filepath.Join(envPath, installStateV2)); err == nil {
		return true
	}
	if data, err := os.ReadFile(filepath.Join(envPath, installStateV1)); err == nil &&
		installStateV1Matches(data, h.AdditionalDependencies) {
		return true
	}
	data, err := os.ReadFile(filepath.Join(envPath, legacyInstallState))
	return err == nil && string(data) == h.InstallKey()
}

// installStateV1Matches is upstream's `_read_state(venv) ==
// _state(hook.additional_dependencies)`: the file holds exactly
// {"additional_dependencies": deps}.
func installStateV1Matches(data []byte, deps []string) bool {
	var state map[string]json.RawMessage
	if json.Unmarshal(data, &state) != nil || len(state) != 1 {
		return false
	}
	raw, ok := state["additional_dependencies"]
	if !ok || !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("[")) {
		return false
	}
	var got []string
	return json.Unmarshal(raw, &got) == nil && slices.Equal(got, deps)
}

// writeInstallState ends an install as upstream's _hook_install does:
// .install_state_v1 is written to a staging file and moved into place, then
// .install_state_v2 is created empty.
func writeInstallState(envPath string, deps []string) error {
	v1 := filepath.Join(envPath, installStateV1)
	staging := v1 + "staging"
	if err := os.WriteFile(staging, installStateV1JSON(deps), 0o644); err != nil {
		return err
	}
	if err := os.Rename(staging, v1); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(envPath, installStateV2), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}

// installStateV1JSON is json.dumps({'additional_dependencies': deps}), byte
// for byte: Python's separators, and ensure_ascii's escapes.
func installStateV1JSON(deps []string) []byte {
	var b strings.Builder
	b.WriteString(`{"additional_dependencies": [`)
	for i, dep := range deps {
		if i > 0 {
			b.WriteString(", ")
		}
		writePyJSONString(&b, dep)
	}
	b.WriteString("]}")
	return []byte(b.String())
}

// writePyJSONString writes s as Python's json module encodes a str with
// ensure_ascii: everything outside printable ASCII is escaped, as \uXXXX in
// lowercase hex, with a surrogate pair beyond the BMP.
func writePyJSONString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			switch {
			case r >= ' ' && r <= '~':
				b.WriteRune(r)
			case r > 0xffff:
				r1, r2 := utf16.EncodeRune(r)
				fmt.Fprintf(b, `\u%04x\u%04x`, r1, r2)
			default:
				fmt.Fprintf(b, `\u%04x`, r)
			}
		}
	}
	b.WriteByte('"')
}
