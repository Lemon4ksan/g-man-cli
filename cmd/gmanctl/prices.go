// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lemon4ksan/g-man-cli/internal/cli/builder"
	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
)

var (
	ErrMissingPriceEntry  = errors.New("missing price entry")
	ErrInvalidPriceFormat = errors.New(
		"invalid price format: expected sku=buy_keys,buy_metal,sell_keys,sell_metal",
	)
	ErrInvalidPriceValuesCount = errors.New("invalid price values count: expected 4 comma-separated values")
)

var updatePricesCmd = builder.New("update-prices <entry> [entry...]").
	Short("Update manual pricing database entries").
	Long(`Updates manual price overrides in the daemon pricing registry.
Format: <sku>=<buy_keys>,<buy_metal>,<sell_keys>,<sell_metal>
Example: gmanctl update-prices 5021;6=1,0.00,1,10.00`).
	Args(cobra.MinimumNArgs(1)).
	Run(func(c *builder.Context) error {
		prices := make(map[string]*pb.ManualPriceEntry, len(c.Args))

		for _, arg := range c.Args {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) != 2 {
				return ErrInvalidPriceFormat
			}

			skuStr := parts[0]

			priceVals := strings.Split(parts[1], ",")
			if len(priceVals) != 4 {
				return ErrInvalidPriceValuesCount
			}

			buyKeys, err := strconv.ParseUint(priceVals[0], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid buyKeys in %q: %w", arg, err)
			}

			buyMetal, err := strconv.ParseFloat(priceVals[1], 64)
			if err != nil {
				return fmt.Errorf("invalid buyMetal in %q: %w", arg, err)
			}

			sellKeys, err := strconv.ParseUint(priceVals[2], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid sellKeys in %q: %w", arg, err)
			}

			sellMetal, err := strconv.ParseFloat(priceVals[3], 64)
			if err != nil {
				return fmt.Errorf("invalid sellMetal in %q: %w", arg, err)
			}

			prices[skuStr] = &pb.ManualPriceEntry{
				BuyKeys:   uint32(buyKeys),
				BuyMetal:  buyMetal,
				SellKeys:  uint32(sellKeys),
				SellMetal: sellMetal,
			}
		}

		c.Renderer.Warning("Sending price updates to daemon...")

		resp, err := c.Client.UpdateManualPrices(c.Ctx, &pb.UpdateManualPricesRequest{Prices: prices})
		if err != nil {
			return fmt.Errorf("failed to update prices: %w", err)
		}

		return c.Render(resp, func() error {
			if resp.GetSuccess() {
				c.Renderer.Success(resp.GetMessage())
			} else {
				c.Renderer.Errorf("Failed: %s", resp.GetMessage())
			}

			return nil
		})
	})

var nicknameCmd = builder.New("nickname <steamid> [nickname]").
	Short("Set or clear custom nickname for a Steam friend").
	Long("Sets a custom friend nickname on Steam. If nickname is omitted, clears the existing nickname.").
	Args(cobra.MinimumNArgs(1)).
	Run(func(c *builder.Context) error {
		steamID, err := strconv.ParseUint(c.Args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid SteamID %q: %w", c.Args[0], err)
		}

		var nickname string
		if len(c.Args) >= 2 {
			nickname = strings.Join(c.Args[1:], " ")
		}

		resp, err := c.Client.SetFriendNickname(c.Ctx, &pb.SetFriendNicknameRequest{
			SteamId:  steamID,
			Nickname: nickname,
		})
		if err != nil {
			return fmt.Errorf("failed to set nickname: %w", err)
		}

		return c.Render(resp, func() error {
			if resp.GetSuccess() {
				c.Renderer.Success(resp.GetMessage())
			} else {
				c.Renderer.Errorf("Failed: %s", resp.GetMessage())
			}

			return nil
		})
	})
