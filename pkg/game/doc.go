// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package game defines the unified abstraction layer and contracts for integrating
game-specific features, inventories, and Game Coordinator (GC) modules.

The package decouples core steam-level daemon features from specific game details,
allowing support for multiple games (such as Team Fortress 2) by registering concrete
[Driver] adapters.

# Key Components

  - [Driver]: The primary interface that game-specific modules implement to coordinate GC state and register providers.
  - [Item]: A unified structure representing a generic item in a game's inventory.
  - [InventoryProvider]: The interface detailing actions like inventory retrieval and custom game action execution.
  - [Registry]: A thread-safe catalog used to register, retrieve, and inspect active game drivers by their AppID.

# Basic Usage Example

	package main

	import (
		"context"
		"fmt"
		"github.com/lemon4ksan/g-man-cli/pkg/game"
	)

	func printInventory(ctx context.Context, reg *game.Registry, appID uint32) {
		driver, ok := reg.Get(appID)
		if !ok {
			fmt.Printf("No driver registered for AppID %d\n", appID)
			return
		}

		provider := driver.InventoryProvider()
		items, err := provider.GetInventory(ctx)
		if err != nil {
			fmt.Println("Failed to get inventory:", err)
			return
		}

		fmt.Printf("Inventory has %d items\n", len(items))
	}
*/
package game
