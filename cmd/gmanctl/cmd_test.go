// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCmd_Structure(t *testing.T) {
	expectedSubcommands := []string{
		"status",
		"stop",
		"gc",
		"events",
		"play",
		"exit-game",
		"exec",
		"update-prices",
		"nickname",
		"request",
		"guard",
	}

	for _, sub := range expectedSubcommands {
		found := false
		for _, c := range rootCmd.Commands() {
			if c.Name() == sub || strings.HasPrefix(c.Use, sub) {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("expected subcommand %q to be registered in rootCmd", sub)
		}
	}
}

func TestRootCmd_HelpOutput(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"--help"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("help execution failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "gmanctl [command]") {
		t.Errorf("expected usage output, got: %s", out)
	}
}

func TestGuardCmd_Subcommands(t *testing.T) {
	expectedGuardSubs := []string{
		"auth",
		"status",
		"code",
		"list",
		"accept",
		"cancel",
		"accept-all",
		"cancel-all",
		"transfer",
		"link",
		"import",
		"qr",
		"encrypt",
		"decrypt",
		"unlock",
		"encrypt-env",
		"decrypt-env",
	}

	for _, sub := range expectedGuardSubs {
		found := false
		for _, c := range guardCmd.Commands() {
			if c.Name() == sub || strings.HasPrefix(c.Use, sub) {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("expected guard subcommand %q to be registered", sub)
		}
	}
}
