// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package driver

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

// findItemInCache searches the TF2 module's SOCache for an item by asset ID.
func findItemInCache(tf2Mod *tf2.TF2, assetID uint64) *tf2.Item {
	for _, item := range tf2Mod.Cache().GetItems() {
		if item.ID == assetID {
			return item
		}
	}

	return nil
}

func (d *Driver) actionDeleteItem(ctx context.Context, params map[string]string) (string, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	itemIDStr, exists := params["item_id"]
	if !exists {
		return "", errors.New("delete-item requires item_id parameter")
	}

	itemID, err := strconv.ParseUint(itemIDStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid item_id: %w", err)
	}

	if err := tf2Mod.DeleteItem(ctx, itemID); err != nil {
		return "", err
	}

	return fmt.Sprintf("Successfully deleted item %d via official module.", itemID), nil
}

func (d *Driver) actionUseItem(ctx context.Context, params map[string]string) (string, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	bpMod, err := d.getBackpackModule()
	if err != nil {
		return "", err
	}

	s := bpMod.Schema().Get()

	itemIDStr, exists := params["item_id"]
	if !exists {
		return "", errors.New("use-item requires item_id parameter")
	}

	itemID, err := strconv.ParseUint(itemIDStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid item_id: %w", err)
	}

	if s != nil {
		targetItem := findItemInCache(tf2Mod, itemID)
		if targetItem != nil {
			targetSch := s.ItemByDef(int(targetItem.DefIndex))
			if targetSch != nil && targetSch.Capabilities != nil && !targetSch.Capabilities.CanConsume {
				return "", fmt.Errorf(
					"item %d (defindex %d, class %q) is not consumable",
					itemID, targetItem.DefIndex, targetSch.ItemClass,
				)
			}
		}
	}

	if err := tf2Mod.UseItem(ctx, itemID); err != nil {
		return "", err
	}

	return fmt.Sprintf("Successfully used item %d via official module.", itemID), nil
}

func (d *Driver) actionApplyTool(ctx context.Context, params map[string]string) (string, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	bpMod, err := d.getBackpackModule()
	if err != nil {
		return "", err
	}

	s := bpMod.Schema().Get()

	toolIDStr, exists := params["tool_id"]
	if !exists {
		return "", errors.New("apply-tool requires tool_id parameter")
	}

	toolID, err := strconv.ParseUint(toolIDStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid tool_id: %w", err)
	}

	itemIDStr, exists := params["item_id"]
	if !exists {
		return "", errors.New("apply-tool requires item_id parameter")
	}

	itemID, err := strconv.ParseUint(itemIDStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid item_id: %w", err)
	}

	actionType, exists := params["type"]
	if !exists {
		return "", errors.New("apply-tool requires type parameter")
	}

	value := params["value"]

	if s != nil {
		targetItem := findItemInCache(tf2Mod, itemID)
		if targetItem != nil {
			targetSch := s.ItemByDef(int(targetItem.DefIndex))
			if targetSch != nil && targetSch.Capabilities != nil {
				if !targetSch.Capabilities.CanApplyTool(actionType) {
					return "", fmt.Errorf(
						"item %d (defindex %d, class %q) does not support tool type %q",
						itemID, targetItem.DefIndex, targetSch.ItemClass, actionType,
					)
				}
			}
		}
	}

	switch actionType {
	case "paint":
		paintName := ""
		if s != nil && value != "" {
			if color, parseErr := strconv.ParseUint(value, 16, 32); parseErr == nil {
				paintName = schema.GetPaintName(uint32(color))
			}
		}

		if err := tf2Mod.ApplyPaint(ctx, toolID, itemID); err != nil {
			return "", err
		}

		if paintName != "" {
			return fmt.Sprintf("Successfully applied paint %s (0x%s) to item %d.", paintName, value, itemID), nil
		}

		return fmt.Sprintf("Successfully applied paint %d to item %d.", toolID, itemID), nil

	case "nametag":
		if value == "" {
			return "", errors.New("nametag requires value parameter for the custom name")
		}

		if err := tf2Mod.NameItem(ctx, toolID, itemID, value); err != nil {
			return "", err
		}

		return fmt.Sprintf("Successfully renamed item %d using tool %d to %q.", itemID, toolID, value), nil

	case "desctag":
		if value == "" {
			return "", errors.New("desctag requires value parameter for the custom description")
		}

		if err := tf2Mod.DescribeItem(ctx, toolID, itemID, value); err != nil {
			return "", err
		}

		return fmt.Sprintf(
			"Successfully added description to item %d using tool %d: %q.",
			itemID,
			toolID,
			value,
		), nil

	case "strangifier":
		if err := tf2Mod.ApplyStrangifier(ctx, itemID, toolID); err != nil {
			return "", err
		}

		return fmt.Sprintf("Successfully strangified item %d using tool %d.", itemID, toolID), nil

	case "strange-part":
		if err := tf2Mod.ApplyStrangePart(ctx, itemID, toolID); err != nil {
			return "", err
		}

		return fmt.Sprintf("Successfully applied strange part %d to item %d.", toolID, itemID), nil

	case "unlock-crate":
		if err := tf2Mod.UnlockCrate(ctx, toolID, itemID); err != nil {
			return "", err
		}

		return fmt.Sprintf("Successfully unlocked crate %d using key %d.", itemID, toolID), nil

	case "wrap-item":
		if err := tf2Mod.WrapItem(ctx, toolID, itemID); err != nil {
			return "", err
		}

		return fmt.Sprintf("Successfully wrapped item %d using gift wrap %d.", itemID, toolID), nil

	case "warpaint":
		if s != nil {
			targetItem := findItemInCache(tf2Mod, itemID)
			if targetItem != nil {
				targetSch := s.ItemByDef(int(targetItem.DefIndex))
				if targetSch != nil && !targetSch.IsPaintKitWeapon() {
					return "", fmt.Errorf(
						"item %d (defindex %d) is not eligible for war paint application",
						itemID, targetItem.DefIndex,
					)
				}
			}
		}

		weaponDefIndex := uint32(itemID) //#nosec G115
		if err := tf2Mod.ConsumePaintkit(ctx, toolID, weaponDefIndex); err != nil {
			return "", err
		}

		return fmt.Sprintf(
			"Successfully applied warpaint %d to create weapon defindex %d.",
			toolID,
			weaponDefIndex,
		), nil

	default:
		return "", fmt.Errorf("unsupported apply-tool action type: %s", actionType)
	}
}

func (d *Driver) actionSetItemStyle(ctx context.Context, params map[string]string) (string, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	itemIDStr, exists := params["item_id"]
	if !exists {
		return "", errors.New("set-item-style requires item_id parameter")
	}

	itemID, err := strconv.ParseUint(itemIDStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid item_id: %w", err)
	}

	styleStr, exists := params["style"]
	if !exists {
		return "", errors.New("set-item-style requires style parameter")
	}

	style, err := strconv.ParseUint(styleStr, 10, 8)
	if err != nil {
		return "", fmt.Errorf("invalid style: %w", err)
	}

	if err := tf2Mod.SetItemStyle(ctx, itemID, uint8(style)); err != nil {
		return "", err
	}

	return fmt.Sprintf("Successfully set style of item %d to style %d.", itemID, style), nil
}

func (d *Driver) actionUnwrapGift(ctx context.Context, params map[string]string) (string, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	itemIDStr, exists := params["item_id"]
	if !exists {
		return "", errors.New("unwrap-gift requires item_id parameter")
	}

	itemID, err := strconv.ParseUint(itemIDStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid item_id: %w", err)
	}

	if err := tf2Mod.UnwrapGiftRequest(ctx, itemID); err != nil {
		return "", err
	}

	return fmt.Sprintf("Successfully sent unwrap gift request for item %d.", itemID), nil
}

func (d *Driver) actionDeliverGift(ctx context.Context, params map[string]string) (string, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	giftIDStr, exists := params["gift_id"]
	if !exists {
		return "", errors.New("deliver-gift requires gift_id parameter")
	}

	giftID, err := strconv.ParseUint(giftIDStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid gift_id: %w", err)
	}

	targetStr, exists := params["target_steam_id"]
	if !exists {
		return "", errors.New("deliver-gift requires target_steam_id parameter")
	}

	targetSteamID, err := strconv.ParseUint(targetStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid target_steam_id: %w", err)
	}

	if err := tf2Mod.DeliverGift(ctx, giftID, targetSteamID); err != nil {
		return "", err
	}

	return fmt.Sprintf("Successfully delivered gift %d to target %d.", giftID, targetSteamID), nil
}

func (d *Driver) actionFulfillDynamicRecipe(ctx context.Context, params map[string]string) (string, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	toolIDStr, exists := params["tool_id"]
	if !exists {
		return "", errors.New("fulfill-dynamic-recipe-component requires tool_id parameter")
	}

	toolID, err := strconv.ParseUint(toolIDStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid tool_id: %w", err)
	}

	subjectIDStr, exists := params["subject_id"]
	if !exists {
		return "", errors.New("fulfill-dynamic-recipe-component requires subject_id parameter")
	}

	subjectID, err := strconv.ParseUint(subjectIDStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid subject_id: %w", err)
	}

	attrIndexStr, exists := params["attribute_index"]
	if !exists {
		return "", errors.New("fulfill-dynamic-recipe-component requires attribute_index parameter")
	}

	attrIndex, err := strconv.ParseUint(attrIndexStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid attribute_index: %w", err)
	}

	if err := tf2Mod.FulfillDynamicRecipeComponent(ctx, toolID, subjectID, attrIndex); err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"Successfully fulfilled dynamic recipe component: tool %d, subject %d, attribute index %d.",
		toolID,
		subjectID,
		attrIndex,
	), nil
}

func (d *Driver) actionRemoveMakersMark(ctx context.Context, params map[string]string) (string, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	itemIDStr, exists := params["item_id"]
	if !exists {
		return "", errors.New("remove-makers-mark requires item_id parameter")
	}

	itemID, err := strconv.ParseUint(itemIDStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid item_id: %w", err)
	}

	if err := tf2Mod.RemoveMakersMark(ctx, itemID); err != nil {
		return "", err
	}

	return fmt.Sprintf("Successfully removed maker's mark signature from item %d.", itemID), nil
}

func (d *Driver) actionRemoveGiftedBy(ctx context.Context, params map[string]string) (string, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	itemIDStr, exists := params["item_id"]
	if !exists {
		return "", errors.New("remove-gifted-by requires item_id parameter")
	}

	itemID, err := strconv.ParseUint(itemIDStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid item_id: %w", err)
	}

	if err := tf2Mod.RemoveGiftedBy(ctx, itemID); err != nil {
		return "", err
	}

	return fmt.Sprintf("Successfully removed gifted-by signature from item %d.", itemID), nil
}

func (d *Driver) actionAcknowledgeAll(ctx context.Context, params map[string]string) (string, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	if err := tf2Mod.AcknowledgeAll(ctx); err != nil {
		return "", err
	}

	return "Successfully acknowledged all new items.", nil
}

func (d *Driver) actionBatchDelete(ctx context.Context, params map[string]string) (string, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	idsStr, exists := params["item_ids"]
	if !exists {
		return "", errors.New("batch-delete requires item_ids parameter (comma-separated)")
	}

	ids := strings.Split(idsStr, ",")

	var (
		deleted, failed int
		errs            []string
	)

	for _, idStr := range ids {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}

		itemID, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			failed++

			errs = append(errs, fmt.Sprintf("invalid id %q: %v", idStr, err))

			continue
		}

		if err := tf2Mod.DeleteItem(ctx, itemID); err != nil {
			failed++

			errs = append(errs, fmt.Sprintf("delete %d: %v", itemID, err))

			continue
		}

		deleted++
	}

	result := fmt.Sprintf("Batch delete: %d deleted, %d failed", deleted, failed)
	if len(errs) > 0 {
		result += "\nErrors:\n" + strings.Join(errs, "\n")
	}

	return result, nil
}

func (d *Driver) actionBatchApplyTool(ctx context.Context, params map[string]string) (string, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	bpMod, err := d.getBackpackModule()
	if err != nil {
		return "", err
	}

	s := bpMod.Schema().Get()

	toolIDStr, exists := params["tool_id"]
	if !exists {
		return "", errors.New("batch-apply-tool requires tool_id parameter")
	}

	toolID, err := strconv.ParseUint(toolIDStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid tool_id: %w", err)
	}

	idsStr, exists := params["item_ids"]
	if !exists {
		return "", errors.New("batch-apply-tool requires item_ids parameter (comma-separated)")
	}

	toolType := params["type"]

	ids := strings.Split(idsStr, ",")

	var applied, failed int

	var errs []string

	for _, idStr := range ids {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}

		itemID, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			failed++

			errs = append(errs, fmt.Sprintf("invalid id %q: %v", idStr, err))

			continue
		}

		// Pre-validate tool compatibility
		if s != nil {
			targetItem := findItemInCache(tf2Mod, itemID)
			if targetItem != nil {
				targetSch := s.ItemByDef(int(targetItem.DefIndex))
				if targetSch != nil && targetSch.Capabilities != nil && !targetSch.Capabilities.CanApplyTool(toolType) {
					failed++

					errs = append(errs, fmt.Sprintf("item %d does not support %q", itemID, toolType))

					continue
				}
			}
		}

		var applyErr error

		switch toolType {
		case "paint":
			applyErr = tf2Mod.ApplyPaint(ctx, toolID, itemID)
		case "strangifier":
			applyErr = tf2Mod.ApplyStrangifier(ctx, itemID, toolID)
		case "strange-part":
			applyErr = tf2Mod.ApplyStrangePart(ctx, itemID, toolID)
		case "autograph":
			applyErr = tf2Mod.ApplyAutograph(ctx, toolID, itemID)
		default:
			applyErr = fmt.Errorf("unsupported tool type for batch: %s", toolType)
		}

		if applyErr != nil {
			failed++

			errs = append(errs, fmt.Sprintf("apply to %d: %v", itemID, applyErr))

			continue
		}

		applied++
	}

	result := fmt.Sprintf("Batch apply: %d applied, %d failed", applied, failed)
	if len(errs) > 0 {
		result += "\nErrors:\n" + strings.Join(errs, "\n")
	}

	return result, nil
}
