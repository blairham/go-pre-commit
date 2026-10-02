// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"testing"

	"github.com/blairham/go-pre-commit/v4/internal/gittest"
)

// actAsBinaryEnv makes the test binary behave as the pre-commit binary, so a
// test can install it as a real git hook and drive it through `git commit`.
const actAsBinaryEnv = "GO_PRE_COMMIT_TEST_ACT_AS_BINARY"

func TestMain(m *testing.M) {
	if os.Getenv(actAsBinaryEnv) == "1" {
		os.Exit(Run(os.Args[1:], BuildInfo{}))
	}
	os.Exit(gittest.Main(m))
}
