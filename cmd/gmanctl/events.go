// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/lemon4ksan/g-man-cli/internal/cli/builder"
	"github.com/lemon4ksan/g-man-cli/internal/cli/output"
	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
)

var eventsCmd = builder.New("events").
	Short("Stream real-time daemon and game coordinator events").
	Long("Opens a reactive gRPC event stream subscribing to live Steam client and game coordinator events (Press Ctrl+C to stop).").
	WithTimeout(0).
	Run(func(c *builder.Context) error {
		c.Renderer.Warning("Streaming real-time daemon events... (Press Ctrl+C to exit)\n")

		stream, err := c.Client.StreamEvents(c.Ctx, &pb.StreamEventsRequest{})
		if err != nil {
			return fmt.Errorf("failed to start event stream: %w", err)
		}

		for {
			resp, err := stream.Recv()
			if err != nil {
				if c.Ctx.Err() != nil {
					return nil
				}

				return fmt.Errorf("stream connection lost: %w", err)
			}

			t := time.Unix(resp.GetTimestamp(), 0).Format("15:04:05")
			evType := resp.GetEventType()

			if idx := strings.LastIndex(evType, "."); idx != -1 {
				evType = evType[idx+1:]
			}

			evType = strings.TrimPrefix(evType, "*")

			if c.Renderer.JSON {
				_ = c.Renderer.PrintJSON(resp)
			} else {
				fmt.Printf("[%s] %s: %s\n",
					output.MutedStyle.Render(t),
					output.SuccessStyle.Render(evType),
					resp.GetPayloadJson(),
				)
			}
		}
	})
