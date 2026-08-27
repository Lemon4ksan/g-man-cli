// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package main

import (
	"fmt"

	"github.com/lemon4ksan/g-man-cli/internal/cli/builder"
	"github.com/lemon4ksan/g-man-cli/internal/cli/output"
	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
)

var statusCmd = builder.New("status").
	Short("Show daemon status, Steam connection and resource metrics").
	Long("Query the running g-mand daemon for live Steam session state, active Game Coordinator, wallet balance, uptime, and memory consumption.").
	Run(func(c *builder.Context) error {
		resp, err := c.Client.GetStatus(c.Ctx, &pb.GetStatusRequest{})
		if err != nil {
			return fmt.Errorf("failed to get daemon status: %w", err)
		}

		return c.Render(resp, func() error {
			fmt.Println(output.TitleStyle.Render("=== G-MAN DAEMON STATUS ==="))

			connStr := output.ErrorStyle.Render("OFFLINE [Steam]")
			if resp.GetConnected() {
				connStr = output.SuccessStyle.Render("ONLINE [Steam]")
			}

			fmt.Printf("Connection State: %s\n", connStr)

			steamID := resp.GetSteamId()
			if steamID == "" || steamID == "0" {
				steamID = output.MutedStyle.Render("Not logged in")
			}

			fmt.Printf("Steam ID:         %s\n", steamID)

			if resp.GetPersonaName() != "" {
				fmt.Printf("Profile Name:     %s\n", output.TitleStyle.Render(resp.GetPersonaName()))
			}

			if resp.GetWalletBalance() > 0 || resp.GetWalletCurrency() != "" {
				fmt.Printf("Wallet Balance:   %s\n", output.SuccessStyle.Render(
					fmt.Sprintf("%.2f %s", float64(resp.GetWalletBalance())/100.0, resp.GetWalletCurrency()),
				))
			}

			gameStr := output.MutedStyle.Render("None")
			if resp.GetCurrentAppid() != 0 {
				gameStr = fmt.Sprintf("%s (AppID %d)",
					output.SuccessStyle.Render(resp.GetCurrentAppName()),
					resp.GetCurrentAppid(),
				)
			}

			fmt.Printf("Active Game:      %s\n", gameStr)
			fmt.Printf("Daemon Uptime:    %s\n", output.WarningStyle.Render(resp.GetUptime()))
			fmt.Printf("Memory Usage:     %s\n", output.WarningStyle.Render(
				fmt.Sprintf("%.2f MB", float64(resp.GetMemoryBytes())/1024.0/1024.0),
			))

			return nil
		})
	})
