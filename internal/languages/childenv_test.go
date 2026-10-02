// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The hook's process must see the environment's PATH, not just have its own
// executable resolved against it: a hook that runs `node` or `python` by name
// needs the env's copy, as upstream's envcontext provides.
func TestRunHookCommandChildSeesEnvPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	marker := filepath.Join(t.TempDir(), "venv-marker", "bin")
	_, out, err := RunHookCommand(context.Background(), t.TempDir(), "sh -c 'echo $PATH'", nil, nil, []string{PrependPath(marker)})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); !strings.HasPrefix(got, marker+string(filepath.ListSeparator)) {
		t.Errorf("child PATH does not start with the env's bin dir %q:\n%s", marker, got)
	}
}
