// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package builder

import (
	"bytes"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestCommandBuilder_Build(t *testing.T) {
	cmd := New("testcmd").
		Short("Short desc").
		Long("Long description").
		Aliases("tc", "t").
		Args(cobra.ExactArgs(1)).
		WithTimeout(10 * time.Second).
		NoIPC().
		Run(func(c *Context) error {
			c.Renderer.Success("Executed: " + c.Args[0])
			return nil
		})

	if cmd.Use != "testcmd" {
		t.Errorf("expected Use 'testcmd', got %q", cmd.Use)
	}

	if cmd.Short != "Short desc" {
		t.Errorf("expected Short 'Short desc', got %q", cmd.Short)
	}

	if len(cmd.Aliases) != 2 || cmd.Aliases[0] != "tc" {
		t.Errorf("unexpected aliases: %v", cmd.Aliases)
	}

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"hello"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command execution failed: %v", err)
	}
}
