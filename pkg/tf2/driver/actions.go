// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package driver

import "github.com/lemon4ksan/g-man-cli/pkg/game"

// Actions returns the list of actions supported by the TF2 driver.
func (d *Driver) Actions() []game.ActionInfo {
	return []game.ActionInfo{
		{
			Name:        "memprofile",
			Description: "Get memory profile of the driver",
		},
		{
			Name:        "backpack-value",
			Description: "Calculate total backpack value in Keys and Refined Metal using pricedb",
		},
		{
			Name:        "targeted-smelt",
			Description: "Smelt two specific weapons into scrap metal",
			Params: []game.ActionParam{
				{Name: "item_id1", Description: "Asset ID of the first weapon to smelt", Required: true},
				{Name: "item_id2", Description: "Asset ID of the second weapon to smelt", Required: true},
			},
		},
		{
			Name:        "inventory",
			Description: "Fetch and list backpack items in a clean table representation",
		},
		{
			Name:        "sort-backpack",
			Description: "Sort the TF2 backpack using a specific layout or GC auto-sort",
			Params: []game.ActionParam{
				{
					Name:        "type",
					Description: "Set to 'gc' for GC auto-sort, otherwise uses G-MAN hierarchical sort",
					Required:    false,
				},
				{
					Name:        "sort_type",
					Description: "GC sort type parameter (e.g., '3'). Only used if type=gc",
					Required:    false,
				},
			},
		},
		{
			Name:        "maintenance",
			Description: "Perform automated backpack maintenance (smelt duplicate weapons, condense metals, and sort backpack)",
		},
		{
			Name:        "craft-metal",
			Description: "Combine weapons or smaller metals into refined/reclaimed metal",
			Params: []game.ActionParam{
				{
					Name:        "type",
					Description: "Target metal type. 'scrap' (weapons to scrap) or 'reclaimed' (scrap to reclaimed)",
					Required:    false,
				},
			},
		},
		{
			Name:        "delete-item",
			Description: "Delete an item from backpack",
			Params: []game.ActionParam{
				{Name: "item_id", Description: "Asset ID of the item to delete", Required: true},
			},
		},
		{
			Name:        "use-item",
			Description: "Use a consumable item from backpack (schema-validated: checks can_consume)",
			Params: []game.ActionParam{
				{Name: "item_id", Description: "Asset ID of the item to use", Required: true},
			},
		},
		{
			Name:        "apply-tool",
			Description: "Apply a tool to a target item (schema-validated: checks capabilities before GC call)",
			Params: []game.ActionParam{
				{Name: "tool_id", Description: "Asset ID of the tool (e.g. Paint, Tag, Key)", Required: true},
				{
					Name:        "item_id",
					Description: "Asset ID of the target item (e.g. Cosmetic, Weapon, Crate)",
					Required:    true,
				},
				{
					Name:        "type",
					Description: "Type: 'paint', 'nametag', 'desctag', 'strangifier', 'strange-part', 'unlock-crate', 'wrap-item', 'warpaint'",
					Required:    true,
				},
				{
					Name:        "value",
					Description: "Custom string value (only required for 'nametag' and 'desctag')",
					Required:    false,
				},
			},
		},
		{
			Name:        "acknowledge-all",
			Description: "Acknowledge all new items in the backpack",
		},
		{
			Name:        "schema",
			Description: "Dump the current TF2 items schema raw JSON payload",
		},
		{
			Name:        "condense-metal",
			Description: "Condense weapons and metal to clean up backpack space",
		},
		{
			Name:        "make-change",
			Description: "Break down higher tier metal into smaller metal",
			Params: []game.ActionParam{
				{
					Name:        "target_defindex",
					Description: "Defindex of the metal to break down (5000 for scrap, 5001 for reclaimed)",
					Required:    true,
				},
				{Name: "target_count", Description: "Number of items to produce", Required: true},
			},
		},
		{
			Name:        "smelt-weapons",
			Description: "Smelt duplicate weapons for a specific class",
			Params: []game.ActionParam{
				{
					Name:        "class",
					Description: "Class name (e.g., 'Scout', 'Soldier', 'Pyro', 'Demoman', 'Heavy', 'Engineer', 'Medic', 'Sniper', 'Spy')",
					Required:    true,
				},
			},
		},
		{
			Name:        "send-offer",
			Description: "Send a trade offer to a partner",
			Params: []game.ActionParam{
				{Name: "offer_params", Description: "JSON string representing OfferParams", Required: true},
			},
		},
		{
			Name:        "accept-offer",
			Description: "Accept an incoming trade offer",
			Params: []game.ActionParam{
				{Name: "offer_id", Description: "Trade offer ID to accept", Required: true},
			},
		},
		{
			Name:        "decline-offer",
			Description: "Decline an incoming trade offer",
			Params: []game.ActionParam{
				{Name: "offer_id", Description: "Trade offer ID to decline", Required: true},
			},
		},
		{
			Name:        "cancel-offer",
			Description: "Cancel an outgoing trade offer",
			Params: []game.ActionParam{
				{Name: "offer_id", Description: "Trade offer ID to cancel", Required: true},
			},
		},
		{
			Name:        "check-escrow",
			Description: "Check if an offer is subject to escrow",
			Params: []game.ActionParam{
				{Name: "offer", Description: "JSON string representing TradeOffer", Required: true},
			},
		},
		{
			Name:        "craft",
			Description: "Execute a specific TF2 crafting recipe",
			Params: []game.ActionParam{
				{Name: "recipe", Description: "Recipe ID (e.g., '3' for smelt class weapons)", Required: true},
				{Name: "items", Description: "JSON array of item asset IDs to use (e.g., '[101,102]')", Required: true},
			},
		},
		{
			Name:        "resolve-vanity-url",
			Description: "Resolve a Steam vanity URL to a Steam64 ID",
			Params: []game.ActionParam{
				{Name: "url", Description: "Vanity URL to resolve", Required: true},
			},
		},
		{
			Name:        "get-partner-inventory",
			Description: "Fetch the public Steam inventory of a trade partner",
			Params: []game.ActionParam{
				{Name: "partner_id", Description: "64-bit SteamID of the partner", Required: true},
			},
		},
		{
			Name:        "active-offers",
			Description: "Get all active incoming trade offers",
		},
		{
			Name:        "active-sent-offers",
			Description: "Get all active outgoing trade offers",
		},
		{
			Name:        "active-offers-rich",
			Description: "Get all active incoming trade offers with resolved item names (for TUI display)",
		},
		{
			Name:        "active-sent-offers-rich",
			Description: "Get all active outgoing trade offers with resolved item names (for TUI display)",
		},
		{
			Name:        "all-offers-rich",
			Description: "Get all incoming trade offers with resolved item names (active and inactive)",
		},
		{
			Name:        "all-sent-offers-rich",
			Description: "Get all outgoing trade offers with resolved item names (active and inactive)",
		},
		{
			Name:        "set-item-style",
			Description: "Set the style index of an item",
			Params: []game.ActionParam{
				{Name: "item_id", Description: "Asset ID of the item to style", Required: true},
				{Name: "style", Description: "Style index to set (e.g. '1')", Required: true},
			},
		},
		{
			Name:        "unwrap-gift",
			Description: "Unwrap a gift item package",
			Params: []game.ActionParam{
				{Name: "item_id", Description: "Asset ID of the gift package to unwrap", Required: true},
			},
		},
		{
			Name:        "deliver-gift",
			Description: "Deliver a wrapped gift package to a target player",
			Params: []game.ActionParam{
				{Name: "gift_id", Description: "Asset ID of the gift package to deliver", Required: true},
				{Name: "target_steam_id", Description: "64-bit SteamID of the target receiver", Required: true},
			},
		},
		{
			Name:        "fulfill-dynamic-recipe-component",
			Description: "Fulfill a component/ingredient of a dynamic recipe (e.g. chemistry set, fabricator)",
			Params: []game.ActionParam{
				{
					Name:        "tool_id",
					Description: "Asset ID of the recipe tool (fabricator/chemistry set)",
					Required:    true,
				},
				{Name: "subject_id", Description: "Asset ID of the ingredient item to consume", Required: true},
				{
					Name:        "attribute_index",
					Description: "Target ingredient attribute index inside the recipe",
					Required:    true,
				},
			},
		},
		{
			Name:        "remove-makers-mark",
			Description: "Remove the crafted-by signature from a crafted item",
			Params: []game.ActionParam{
				{
					Name:        "item_id",
					Description: "Asset ID of the item to remove the crafted-by signature from",
					Required:    true,
				},
			},
		},
		{
			Name:        "remove-gifted-by",
			Description: "Remove the gifted-by signature from an item",
			Params: []game.ActionParam{
				{
					Name:        "item_id",
					Description: "Asset ID of the item to remove the gifted-by signature from",
					Required:    true,
				},
			},
		},
		{
			Name:        "batch-delete",
			Description: "Delete multiple items from backpack at once",
			Params: []game.ActionParam{
				{Name: "item_ids", Description: "Comma-separated list of asset IDs to delete", Required: true},
			},
		},
		{
			Name:        "batch-apply-tool",
			Description: "Apply a tool to multiple items at once (schema-validated per item)",
			Params: []game.ActionParam{
				{Name: "tool_id", Description: "Asset ID of the tool to apply", Required: true},
				{Name: "item_ids", Description: "Comma-separated list of target asset IDs", Required: true},
				{
					Name:        "type",
					Description: "Tool type: 'paint', 'strangifier', 'strange-part', 'autograph'",
					Required:    true,
				},
			},
		},
		{
			Name:        "item-details",
			Description: "Show detailed information about a specific item",
			Params: []game.ActionParam{
				{Name: "item_id", Description: "Asset ID of the item to inspect", Required: true},
			},
		},
		{
			Name:        "price-check",
			Description: "Check the current price of an item by SKU from pricedb",
			Params: []game.ActionParam{
				{Name: "sku", Description: "Item SKU to check (e.g. '5021;6' for keys)", Required: true},
			},
		},
		{
			Name:        "inventory-stats",
			Description: "Show inventory statistics: counts by quality, section, tradability",
		},
		{
			Name:        "craft-recipe-list",
			Description: "List available crafting recipes and operations",
		},
		{
			Name:        "health-check",
			Description: "Check daemon health: GC connection, modules, memory, goroutines",
		},
		{
			Name:        "profit-report",
			Description: "Show profit report from stats database",
			Params: []game.ActionParam{
				{Name: "stats_path", Description: "Path to the stats JSON file", Required: true},
				{Name: "days", Description: "Number of days to include (default: 7)", Required: false},
			},
		},
		{
			Name:        "activity-log",
			Description: "Show recent bot activity from cache",
		},
	}
}
