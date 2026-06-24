// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package driver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man/pkg/trading/web"
)

func (d *Driver) actionMemprofile(_ context.Context, _ map[string]string) (string, error) {
	var sb strings.Builder
	sb.WriteString("\n=== MEMORY PROFILE ===\n")

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	fmt.Fprintf(&sb, "Alloc:       %s\n", formatBytes(m.Alloc))
	fmt.Fprintf(&sb, "TotalAlloc:  %s\n", formatBytes(m.TotalAlloc))
	fmt.Fprintf(&sb, "Sys:         %s\n", formatBytes(m.Sys))
	fmt.Fprintf(&sb, "HeapAlloc:   %s\n", formatBytes(m.HeapAlloc))
	fmt.Fprintf(&sb, "HeapSys:     %s\n", formatBytes(m.HeapSys))
	fmt.Fprintf(&sb, "HeapIdle:    %s\n", formatBytes(m.HeapIdle))
	fmt.Fprintf(&sb, "HeapInuse:   %s\n", formatBytes(m.HeapInuse))
	fmt.Fprintf(&sb, "HeapReleased:%s\n", formatBytes(m.HeapReleased))
	fmt.Fprintf(&sb, "HeapObjects: %d\n", m.HeapObjects)
	fmt.Fprintf(&sb, "StackSys:    %s\n", formatBytes(m.StackSys))
	fmt.Fprintf(&sb, "MSpanInuse:  %d\n", m.MSpanInuse)
	fmt.Fprintf(&sb, "MCacheInuse: %d\n", m.MCacheInuse)
	fmt.Fprintf(&sb, "Goroutines:  %d\n", runtime.NumGoroutine())
	fmt.Fprintf(&sb, "GCs:         %d\n", m.NumGC)
	fmt.Fprintf(&sb, "LastGC:      %s\n", time.Unix(0, int64(m.LastGC)).Format("15:04:05")) //nolint:gosec
	sb.WriteString("=======================\n")

	return sb.String(), nil
}

func (d *Driver) actionHealthCheck(_ context.Context, _ map[string]string) (string, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("\n=== HEALTH CHECK ===\n")

	gcConnected := tf2Mod.Connected()
	if gcConnected {
		sb.WriteString("GC Connection:  OK\n")
	} else {
		sb.WriteString("GC Connection:  DISCONNECTED\n")
	}

	webMod := web.From(d.client)
	if webMod != nil {
		sb.WriteString("Web Module:     OK\n")
	} else {
		sb.WriteString("Web Module:     NOT AVAILABLE\n")
	}

	itemCount := len(tf2Mod.Cache().GetItems())
	fmt.Fprintf(&sb, "Cached Items:   %d\n", itemCount)

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Fprintf(&sb, "Memory:         %s\n", formatBytes(m.Alloc))
	fmt.Fprintf(&sb, "Goroutines:     %d\n", runtime.NumGoroutine())

	sb.WriteString("====================\n")

	return sb.String(), nil
}

func (d *Driver) actionProfitReport(_ context.Context, params map[string]string) (string, error) {
	statsPath := params["stats_path"]
	if statsPath == "" {
		return "", errors.New("profit-report requires stats_path parameter")
	}

	days := 7
	if dStr, exists := params["days"]; exists {
		if v, err := strconv.Atoi(dStr); err == nil && v > 0 {
			days = v
		}
	}

	data, err := os.ReadFile(statsPath)
	if err != nil {
		return "", fmt.Errorf("failed to read stats file: %w", err)
	}

	var stats map[string]interface{}
	if err := json.Unmarshal(data, &stats); err != nil {
		return "", fmt.Errorf("failed to parse stats: %w", err)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "\n=== PROFIT REPORT (last %d days) ===\n", days)
	sb.WriteString("Stats file loaded successfully.\n")
	fmt.Fprintf(&sb, "Data size: %d bytes\n", len(data))
	sb.WriteString("=====================================\n")

	return sb.String(), nil
}

func (d *Driver) actionActivityLog(_ context.Context, _ map[string]string) (string, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("\n=== RECENT ACTIVITY ===\n")

	events := tf2Mod.Cache().GetItems()
	eventCount := len(events)
	fmt.Fprintf(&sb, "Cached inventory events: %d items\n", eventCount)

	sb.WriteString("======================\n")

	return sb.String(), nil
}

func (d *Driver) actionCraftRecipeList(_ context.Context, _ map[string]string) (string, error) {
	bpMod, err := d.getBackpackModule()
	if err != nil {
		return "", err
	}

	s := bpMod.Schema().Get()

	var sb strings.Builder
	sb.WriteString("\n=== AVAILABLE CRAFTING RECIPES ===\n")

	categoryNames := map[schema.RecipeCategory]string{
		schema.RecipeCategoryCraftingItems: "Crafting (Metal)",
		schema.RecipeCategoryCommonItems:   "Common Items",
		schema.RecipeCategoryRareItems:     "Rare Items",
		schema.RecipeCategorySpecial:       "Special",
	}

	if s != nil {
		recipes := s.GetAllRecipes()
		if len(recipes) == 0 {
			sb.WriteString("  No recipes loaded from schema.\n")
			sb.WriteString("  Recipes are parsed from items_game.txt on schema refresh.\n")
		} else {
			grouped := make(map[schema.RecipeCategory][]*schema.RecipeDefinition)
			for _, r := range recipes {
				grouped[r.Category] = append(grouped[r.Category], r)
			}

			categories := []schema.RecipeCategory{
				schema.RecipeCategoryCraftingItems,
				schema.RecipeCategoryCommonItems,
				schema.RecipeCategoryRareItems,
				schema.RecipeCategorySpecial,
			}

			for _, cat := range categories {
				catRecipes := grouped[cat]
				if len(catRecipes) == 0 {
					continue
				}

				fmt.Fprintf(&sb, "\n  [%s]\n", categoryNames[cat])

				shown := 0
				for _, r := range catRecipes {
					if shown >= 20 {
						fmt.Fprintf(&sb, "  ... and %d more\n", len(catRecipes)-shown)
						break
					}

					status := ""
					if r.Disabled {
						status = " (disabled)"
					}

					fmt.Fprintf(&sb, "    %d: %s%s\n", r.DefIndex, r.Name, status)

					shown++
				}
			}
		}
	} else {
		sb.WriteString("  Schema not loaded.\n")
		sb.WriteString("\n  Standard metal recipes:\n")
		sb.WriteString("    3: Smelt 2 weapons -> 1 Scrap\n")
		sb.WriteString("    4: 3 Scrap -> 1 Reclaimed\n")
		sb.WriteString("    5: 3 Reclaimed -> 1 Refined\n")
	}

	sb.WriteString("\nUse 'craft' action with recipe ID and item IDs.\n")
	sb.WriteString("======================================\n")

	return sb.String(), nil
}

func (d *Driver) actionPriceCheck(ctx context.Context, params map[string]string) (string, error) {
	skuStr, exists := params["sku"]
	if !exists {
		return "", errors.New("price-check requires sku parameter")
	}

	pdbClient := pricedb.NewClient(nil)

	pricesSlice, err := pdbClient.GetItemsBulk(ctx, []string{skuStr})
	if err != nil {
		return "", fmt.Errorf("failed to fetch price: %w", err)
	}

	if len(pricesSlice) == 0 || pricesSlice[0] == nil {
		return "No price data found for SKU " + skuStr, nil
	}

	entry := pricesSlice[0]

	var sb strings.Builder
	fmt.Fprintf(&sb, "\n=== PRICE CHECK: %s ===\n", skuStr)
	fmt.Fprintf(&sb, "Buy:   %d keys, %.2f ref\n", entry.Buy.Keys, entry.Buy.Metal)
	fmt.Fprintf(&sb, "Sell:  %d keys, %.2f ref\n", entry.Sell.Keys, entry.Sell.Metal)
	fmt.Fprintf(&sb, "Source: %s\n", entry.Source)
	sb.WriteString("============================\n")

	return sb.String(), nil
}

func (d *Driver) actionSchema(_ context.Context, _ map[string]string) (string, error) {
	schemaMod := schema.From(d.client)
	if schemaMod == nil {
		return "", errors.New("schema module not registered in steam client")
	}

	sch := schemaMod.Get()
	if sch == nil {
		return "", errors.New("schema not loaded yet")
	}

	rawBytes, err := json.Marshal(sch.Raw)
	if err != nil {
		return "", fmt.Errorf("failed to marshal schema: %w", err)
	}

	return string(rawBytes), nil
}
