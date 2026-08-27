// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/lemon4ksan/g-man-cli/internal/cli/builder"
)

var rootCmd = &cobra.Command{
	Use:   "gmanctl",
	Short: "Official system control toolchain for the G-MAN Framework",
	Long: `G-man CLI is the official system control toolchain and daemon suite for G-man Steam automation pipelines.
It provides industrial-scale operations management, multi-game coordination, and direct inventory control over secure, low-latency gRPC channels.`,
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		_ = godotenv.Load(getEnvPath())
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	builder.GlobalIPCProvider = getIPCConnection

	rootCmd.PersistentFlags().StringVar(&flagSocket, "socket", "", "Custom Unix domain socket path")
	rootCmd.PersistentFlags().StringVar(&flagTCP, "tcp", "", "Custom TCP loopback address (e.g. 127.0.0.1:50051)")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output machine-readable JSON")
	rootCmd.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "Disable ANSI color output")

	rootCmd.AddCommand(
		statusCmd,
		stopCmd,
		gcCmd,
		eventsCmd,
		playCmd,
		exitGameCmd,
		execCmd,
		updatePricesCmd,
		nicknameCmd,
		requestCmd,
		guardCmd,
	)
}
