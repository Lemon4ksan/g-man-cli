// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package game

import "context"

// Driver defines the contract for game-specific modules integrating with Steam Game Coordinators.
type Driver interface {
	// AppID returns the official Steam AppID of the target game (e.g. 440 for TF2).
	AppID() uint32

	// OnStartGC is called when a game session starts playing and GC streams require initialization.
	OnStartGC(ctx context.Context) error

	// OnStopGC is called when a game session ends to close GC network channels cleanly.
	OnStopGC(ctx context.Context) error

	// ExecuteAction runs a game-specific command (e.g., "sort-backpack", "craft-metal", "delete-item").
	ExecuteAction(ctx context.Context, action string, params map[string]string) (string, error)

	// Actions returns descriptions for all executable commands supported by the driver.
	Actions() []ActionInfo
}

// Item represents a generic item structure in a game's inventory.
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

// InventoryProvider provides high-level inventory retrieval operations.
type InventoryProvider interface {
	// GetInventory fetches and returns all items currently present in the user's backpack.
	GetInventory(ctx context.Context) ([]Item, error)
}

// ActionParam describes an input parameter accepted by an executable driver action.
type ActionParam struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// ActionInfo metadata describing an executable driver action and its parameter schema.
type ActionInfo struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Params      []ActionParam `json:"params,omitempty"`
}
