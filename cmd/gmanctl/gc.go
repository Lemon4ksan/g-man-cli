// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package main

import (
	"fmt"

	"github.com/lemon4ksan/g-man-cli/internal/cli/builder"
	"github.com/lemon4ksan/g-man-cli/internal/cli/output"
	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
)

var gcCmd = builder.New("gc").
	Short("Force manual garbage collection and free physical memory").
	Long("Triggers runtime.GC() followed by debug.FreeOSMemory() in the background daemon, releasing unused physical memory to the Host OS.").
	Run(func(c *builder.Context) error {
		c.Renderer.Warning("Triggering manual Garbage Collection in daemon...")

		resp, err := c.Client.FreeMemory(c.Ctx, &pb.FreeMemoryRequest{})
		if err != nil {
			return fmt.Errorf("failed to trigger GC: %w", err)
		}

		return c.Render(resp, func() error {
			c.Renderer.Success(resp.GetMessage())
			fmt.Printf("New Memory Usage: %s\n", output.WarningStyle.Render(
				fmt.Sprintf("%.2f MB", float64(resp.GetMemoryBytes())/1024.0/1024.0),
			))

			return nil
		})
	})
