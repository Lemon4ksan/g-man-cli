// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package game defines the unified contracts, data structures, and registry for integrating
// game-specific modules and Game Coordinator (GC) drivers into the daemon.
//
// Key Concepts:
//   - [Driver]: Interface that game modules implement to control Game Coordinator sessions and execute inventory actions.
//   - [Registry]: Thread-safe catalog mapping Steam AppIDs (e.g., 440 for TF2) to registered drivers.
//   - [InventoryProvider]: High-level interface for fetching backpack items across different game drivers.
//
// Thread Safety:
//   - The [Registry] is safe for concurrent read and write operations.
package game
