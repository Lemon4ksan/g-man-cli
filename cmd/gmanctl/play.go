// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package main

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/lemon4ksan/g-man-cli/internal/cli/builder"
	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
)

var ErrMissingAppID = errors.New("missing AppID argument")

var playCmd = builder.New("play <appid>").
	Short("Launch game session & initialize Game Coordinator").
	Long("Spins up the designated game session on Steam (e.g. 440 for TF2, 730 for CS2) and initializes the respective Game Coordinator protocol driver.").
	Args(cobra.ExactArgs(1)).
	Run(func(c *builder.Context) error {
		appID64, err := strconv.ParseUint(c.Args[0], 10, 32)
		if err != nil {
			return fmt.Errorf("invalid AppID %q: must be an integer", c.Args[0])
		}

		appID := uint32(appID64)

		c.Renderer.Warningf("Launching session for game AppID %d...", appID)

		resp, err := c.Client.PlayGame(c.Ctx, &pb.PlayGameRequest{Appid: appID})
		if err != nil {
			return fmt.Errorf("failed to play game: %w", err)
		}

		return c.Render(resp, func() error {
			c.Renderer.Success(resp.GetMessage())
			return nil
		})
	})

var exitGameCmd = builder.New("exit-game").
	Short("Close active game session, return to simple online mode").
	Long("Closes the active Game Coordinator connection and leaves the game session while maintaining the core Steam online presence.").
	Run(func(c *builder.Context) error {
		c.Renderer.Warning("Stopping playing session and exiting game...")

		resp, err := c.Client.ExitGame(c.Ctx, &pb.ExitGameRequest{})
		if err != nil {
			return fmt.Errorf("failed to exit game: %w", err)
		}

		return c.Render(resp, func() error {
			c.Renderer.Success(resp.GetMessage())
			return nil
		})
	})
