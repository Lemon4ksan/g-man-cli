// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package main

import (
	"fmt"

	"github.com/lemon4ksan/g-man-cli/internal/cli/builder"
	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
)

var stopCmd = builder.New("stop").
	Short("Stop the background daemon gracefully").
	Long("Sends a graceful termination signal to g-mand, syncing all pending storage files and disconnecting cleanly from Steam.").
	Run(func(c *builder.Context) error {
		c.Renderer.Warning("Stopping g-man daemon...")

		resp, err := c.Client.StopDaemon(c.Ctx, &pb.StopDaemonRequest{})
		if err != nil {
			return fmt.Errorf("failed to stop daemon: %w", err)
		}

		return c.Render(resp, func() error {
			c.Renderer.Success(resp.GetMessage())
			return nil
		})
	})
