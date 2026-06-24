// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package driver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/crafting"
	"github.com/lemon4ksan/g-man/pkg/log"
)

// RunMaintenance performs non-interactive duplicate weapon smelting, metal condensing, and sorting.
func (d *Driver) RunMaintenance(ctx context.Context, logger log.Logger) error {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return err
	}

	bpMod, err := d.getBackpackModule()
	if err != nil {
		return err
	}

	if !tf2Mod.Connected() {
		return errors.New("no active connection to TF2 Game Coordinator")
	}

	logger.InfoContext(ctx, "Starting automated non-interactive inventory maintenance...")

	logger.InfoContext(ctx, "Acknowledging all new/crafted items in the backpack...")

	if err := tf2Mod.AcknowledgeAll(ctx); err != nil {
		logger.ErrorContext(ctx, "Failed to acknowledge items", log.Err(err))
	}

	craftMgr := crafting.NewManager(bpMod, tf2Mod)

	logger.InfoContext(ctx, "Scanning for duplicate weapons to smelt...")

	classes := []string{"Scout", "Soldier", "Pyro", "Demoman", "Heavy", "Engineer", "Medic", "Sniper", "Spy"}
	totalSmelted := 0

	timer := time.NewTimer(500 * time.Millisecond)
	defer timer.Stop()

	for _, class := range classes {
		for {
			weapons := bpMod.FindWeaponsByClassForSmelting(class)
			if len(weapons) < 2 {
				break
			}

			w1, w2 := weapons[0], weapons[1]
			if !w1.IsTradable || !w2.IsTradable {
				logger.ErrorContext(ctx, "Refusing to smelt: weapons must be tradable",
					log.Uint64("id1", w1.ID),
					log.Uint64("id2", w2.ID),
				)

				break
			}

			logger.DebugContext(ctx, "Smelting duplicate weapons...",
				log.String("class", class),
				log.Uint64("item_1", w1.ID),
				log.Uint64("item_2", w2.ID),
			)

			_, err := craftMgr.SmeltClassWeapons(ctx, class)
			if err != nil {
				logger.ErrorContext(ctx, "Smelting failed", log.String("class", class), log.Err(err))
				break
			}

			totalSmelted++

			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}

			timer.Reset(500 * time.Millisecond)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
			}
		}
	}

	logger.InfoContext(ctx, "Duplicate weapon smelting completed", log.Int("operations", totalSmelted))
	logger.InfoContext(ctx, "Condensing low-grade metals...")

	crafts, err := craftMgr.CondenseMetal(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to condense metals", log.Err(err))
	} else {
		logger.InfoContext(ctx, "Metal condensation completed", log.Int("operations", crafts))
	}

	logger.InfoContext(ctx, "Executing custom continuous sorting...")

	if err := d.SortInventory(ctx, logger); err != nil {
		logger.ErrorContext(ctx, "Failed to sort backpack", log.Err(err))
		return err
	}

	logger.InfoContext(ctx, "Automated maintenance completed successfully")

	return nil
}

// SortInventory performs continuous tight sorting of the backpack.
func (d *Driver) SortInventory(ctx context.Context, logger log.Logger) error {
	bpMod, err := d.getBackpackModule()
	if err != nil {
		return err
	}

	return bpMod.ApplyLayout(ctx, backpack.DefaultLayout())
}

func (d *Driver) actionSortBackpack(ctx context.Context, params map[string]string) (string, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	if params["type"] == "gc" {
		sortType := uint32(1)
		if val, exists := params["sort_type"]; exists {
			if parsed, err := strconv.ParseUint(val, 10, 32); err == nil {
				sortType = uint32(parsed)
			}
		}

		if err := tf2Mod.SortBackpack(ctx, sortType); err != nil {
			return "", err
		}

		return "TF2 backpack sorted successfully via Game Coordinator auto-sort.", nil
	}

	if err := d.SortInventory(ctx, log.Discard); err != nil {
		return "", err
	}

	return "TF2 backpack sorted successfully via G-MAN continuous hierarchical sort.", nil
}

func (d *Driver) actionMaintenance(ctx context.Context, params map[string]string) (string, error) {
	if err := d.RunMaintenance(ctx, log.Discard); err != nil {
		return "", err
	}

	return "TF2 backpack maintenance (smelt, condense, sort) completed successfully.", nil
}

func (d *Driver) actionCraftMetal(ctx context.Context, params map[string]string) (string, error) {
	bpMod, err := d.getBackpackModule()
	if err != nil {
		return "", err
	}

	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	mgr := crafting.NewManager(bpMod, tf2Mod)

	targetType := "reclaimed"
	if val, exists := params["type"]; exists {
		targetType = val
	}

	var (
		created  []uint64
		craftErr error
	)

	if targetType == "scrap" {
		created, craftErr = mgr.CombineMetal(ctx, crafting.DefIndexScrap)
	} else {
		created, craftErr = mgr.CombineMetal(ctx, crafting.DefIndexReclaimed)
	}

	if craftErr != nil {
		return "", fmt.Errorf("official crafting combination failed: %w", craftErr)
	}

	return fmt.Sprintf(
		"Successfully combined metal using official crafting manager. Created item IDs: %v",
		created,
	), nil
}

func (d *Driver) actionCondenseMetal(ctx context.Context, params map[string]string) (string, error) {
	bpMod, err := d.getBackpackModule()
	if err != nil {
		return "", err
	}

	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	mgr := crafting.NewManager(bpMod, tf2Mod)

	crafts, err := mgr.CondenseMetal(ctx)
	if err != nil {
		return "", err
	}

	return strconv.Itoa(crafts), nil
}

func (d *Driver) actionMakeChange(ctx context.Context, params map[string]string) (string, error) {
	targetDefIndexStr, exists := params["target_defindex"]
	if !exists {
		return "", errors.New("make-change requires target_defindex parameter")
	}

	targetDefIndex, err := strconv.ParseUint(targetDefIndexStr, 10, 32)
	if err != nil {
		return "", err
	}

	targetCountStr, exists := params["target_count"]
	if !exists {
		return "", errors.New("make-change requires target_count parameter")
	}

	targetCount, err := strconv.Atoi(targetCountStr)
	if err != nil {
		return "", err
	}

	bpMod, err := d.getBackpackModule()
	if err != nil {
		return "", err
	}

	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	mgr := crafting.NewManager(bpMod, tf2Mod)
	if err := mgr.MakeChange(ctx, uint32(targetDefIndex), targetCount); err != nil {
		return "", err
	}

	return "Successfully made change.", nil
}

func (d *Driver) actionSmeltWeapons(ctx context.Context, params map[string]string) (string, error) {
	class, exists := params["class"]
	if !exists {
		return "", errors.New("smelt-weapons requires class parameter")
	}

	bpMod, err := d.getBackpackModule()
	if err != nil {
		return "", err
	}

	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	mgr := crafting.NewManager(bpMod, tf2Mod)

	res, err := mgr.SmeltClassWeapons(ctx, class)
	if err != nil {
		return "", err
	}

	data, err := json.Marshal(res)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (d *Driver) actionCraft(ctx context.Context, params map[string]string) (string, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	recipeStr, exists := params["recipe"]
	if !exists {
		return "", errors.New("craft requires recipe parameter")
	}

	recipe, err := strconv.ParseInt(recipeStr, 10, 16)
	if err != nil {
		return "", fmt.Errorf("invalid recipe: %w", err)
	}

	itemsStr, exists := params["items"]
	if !exists {
		return "", errors.New("craft requires items parameter")
	}

	var items []uint64
	if err := json.Unmarshal([]byte(itemsStr), &items); err != nil {
		return "", fmt.Errorf("invalid items: %w", err)
	}

	created, err := tf2Mod.Craft(ctx, items, int16(recipe))
	if err != nil {
		return "", fmt.Errorf("GC craft failed: %w", err)
	}

	createdBytes, err := json.Marshal(created)
	if err != nil {
		return "", fmt.Errorf("failed to marshal created items: %w", err)
	}

	return string(createdBytes), nil
}

func (d *Driver) actionTargetedSmelt(ctx context.Context, params map[string]string) (string, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	itemID1Str, exists := params["item_id1"]
	if !exists {
		return "", errors.New("targeted-smelt requires item_id1 parameter")
	}

	itemID1, err := strconv.ParseUint(itemID1Str, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid item_id1: %w", err)
	}

	itemID2Str, exists := params["item_id2"]
	if !exists {
		return "", errors.New("targeted-smelt requires item_id2 parameter")
	}

	itemID2, err := strconv.ParseUint(itemID2Str, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid item_id2: %w", err)
	}

	created, err := tf2Mod.Craft(ctx, []uint64{itemID1, itemID2}, 3)
	if err != nil {
		return "", fmt.Errorf("targeted smelting failed: %w", err)
	}

	return fmt.Sprintf("Successfully smelted items %d and %d. Created item IDs: %v", itemID1, itemID2, created), nil
}
