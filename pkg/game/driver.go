// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package game

import (
	"context"
)

// Driver defines the unified contract for game-specific modules.
type Driver interface {
	// AppID returns the Steam AppID of the game.
	AppID() uint32

	// OnStartGC is called when the game starts playing and GC needs to be initialized.
	OnStartGC(ctx context.Context) error

	// OnStopGC is called when the game session is terminated.
	OnStopGC(ctx context.Context) error

	// ExecuteAction runs a game-specific inventory manipulation command.
	// For example: "sort-backpack", "craft-metal", "delete-item".
	ExecuteAction(ctx context.Context, action string, params map[string]string) (string, error)

	// Actions returns a list of supported action descriptions for the driver.
	Actions() []ActionInfo
}

// Item represents a generic item in the game's inventory.
type Item struct {
	AssetID       uint64            `json:"asset_id"`
	DefIndex      uint32            `json:"def_index"`
	Quality       uint32            `json:"quality"`
	Quantity      uint32            `json:"quantity"`
	IsTradable    bool              `json:"is_tradable"`
	IsCraftable   bool              `json:"is_craftable"`
	Attributes    map[string]string `json:"attributes,omitempty"`
	ImageURL      string            `json:"image_url,omitempty"`
	ImageURLLarge string            `json:"image_url_large,omitempty"`
}

// InventoryProvider provides high-level operations for inventory interaction.
type InventoryProvider interface {
	// GetInventory returns the list of all items currently in the user's backpack.
	GetInventory(ctx context.Context) ([]Item, error)
}

// ActionParam describes a parameter for an inventory action.
type ActionParam struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// ActionInfo describes an executable inventory action.
type ActionInfo struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Params      []ActionParam `json:"params,omitempty"`
}
