// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"testing"

	"github.com/blairham/go-pre-commit/v4/internal/gittest"
)

func TestMain(m *testing.M) { os.Exit(gittest.Main(m)) }
