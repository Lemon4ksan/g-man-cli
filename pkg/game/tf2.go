// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package game

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/crafting"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	"github.com/lemon4ksan/g-man/pkg/steam"
)

// TF2Driver acts as an adapter wrapping the official g-man-tf2 steam modules.
type TF2Driver struct {
	client *steam.Client
}

// NewTF2Driver constructs a new TF2Driver adapter instance.
func NewTF2Driver(client *steam.Client) *TF2Driver {
	return &TF2Driver{
		client: client,
	}
}

// AppID returns the official TF2 AppID (440).
func (d *TF2Driver) AppID() uint32 {
	return tf2.AppID
}

func (d *TF2Driver) getTF2Module() (*tf2.TF2, error) {
	tf2Mod := tf2.From(d.client)
	if tf2Mod == nil {
		return nil, errors.New("tf2 module not registered in steam client")
	}

	return tf2Mod, nil
}

func (d *TF2Driver) getBackpackModule() (*backpack.Backpack, error) {
	bpMod := backpack.From(d.client)
	if bpMod == nil {
		return nil, errors.New("backpack module not registered in steam client")
	}

	return bpMod, nil
}

// OnStartGC is triggered when TF2 GC is requested to launch.
func (d *TF2Driver) OnStartGC(ctx context.Context) error {
	// The official TF2 module automatically starts the hello handshake when authenticated.
	return nil
}

// OnStopGC is triggered when TF2 GC is requested to close.
func (d *TF2Driver) OnStopGC(ctx context.Context) error {
	return nil
}

// InventoryProvider returns this adapter as the inventory provider.
func (d *TF2Driver) InventoryProvider() InventoryProvider {
	return d
}

// GetInventory fetches backpack contents directly from the official TF2 module's SOCache.
func (d *TF2Driver) GetInventory(ctx context.Context) ([]Item, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return nil, err
	}

	realItems := tf2Mod.Cache().GetItems()

	items := make([]Item, len(realItems))
	for i, ri := range realItems {
		items[i] = Item{
			AssetID:     ri.ID,
			DefIndex:    ri.DefIndex,
			Quality:     ri.Quality,
			Quantity:    ri.Quantity,
			IsTradable:  ri.IsTradable,
			IsCraftable: ri.IsCraftable,
			Attributes: map[string]string{
				"custom_name": ri.CustomName,
				"custom_desc": ri.CustomDesc,
				"sku":         ri.SKU,
			},
		}
	}

	return items, nil
}

// ExecuteAction executes operations directly on the official TF2 extension or crafting manager.
func (d *TF2Driver) ExecuteAction(ctx context.Context, action string, params map[string]string) (string, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	switch action {
	case "sort-backpack":
		sortType := uint32(1)
		if val, exists := params["sort_type"]; exists {
			if parsed, err := strconv.ParseUint(val, 10, 32); err == nil {
				sortType = uint32(parsed)
			}
		}

		if err := tf2Mod.SortBackpack(ctx, sortType); err != nil {
			return "", err
		}

		return "TF2 backpack sorted successfully via official module.", nil

	case "craft-metal":
		bpMod, err := d.getBackpackModule()
		if err != nil {
			return "", err
		}

		// Initialize the official crafting manager using bpMod (InventoryProvider) and tf2Mod (GCProvider)
		mgr := crafting.NewManager(bpMod, tf2Mod)

		targetType := "reclaimed" // Default to combining reclaimed into refined
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

	case "delete-item":
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

	case "use-item":
		itemIDStr, exists := params["item_id"]
		if !exists {
			return "", errors.New("use-item requires item_id parameter")
		}

		itemID, err := strconv.ParseUint(itemIDStr, 10, 64)
		if err != nil {
			return "", fmt.Errorf("invalid item_id: %w", err)
		}

		if err := tf2Mod.UseItem(ctx, itemID); err != nil {
			return "", err
		}

		return fmt.Sprintf("Successfully used item %d via official module.", itemID), nil

	case "acknowledge-all":
		if err := tf2Mod.AcknowledgeAll(ctx); err != nil {
			return "", err
		}

		return "Successfully acknowledged all new items.", nil

	default:
		return "", fmt.Errorf("unsupported action for official TF2 module: %s", action)
	}
}
