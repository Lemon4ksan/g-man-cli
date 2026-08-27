// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package driver

import (
	"context"

	"github.com/lemon4ksan/g-man-cli/pkg/game"
)

// initRouter registers all declarative TF2 Game Coordinator and crafting actions.
func (d *Driver) initRouter() {
	r := d.router

	r.Define("inventory").
		Desc("Fetch and list backpack items in a clean table representation").
		Aliases("list-backpack").
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionInventory(ctx, p)
		})

	r.Define("memprofile").
		Desc("Get memory profile of the driver").
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionMemprofile(ctx, p)
		})

	r.Define("backpack-value").
		Desc("Calculate total backpack value in Keys and Refined Metal using pricedb").
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionBackpackValue(ctx, p)
		})

	r.Define("targeted-smelt").
		Desc("Smelt two specific weapons into scrap metal").
		Param("item_id1", "Asset ID of the first weapon to smelt", true).
		Param("item_id2", "Asset ID of the second weapon to smelt", true).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionTargetedSmelt(ctx, p)
		})

	r.Define("sort-backpack").
		Desc("Sort the TF2 backpack using a specific layout or GC auto-sort").
		Param("type", "Set to 'gc' for GC auto-sort, otherwise uses G-MAN hierarchical sort", false).
		Param("sort_type", "GC sort type parameter (e.g., '3'). Only used if type=gc", false).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionSortBackpack(ctx, p)
		})

	r.Define("maintenance").
		Desc("Perform automated backpack maintenance (smelt duplicate weapons, condense metals, and sort backpack)").
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionMaintenance(ctx, p)
		})

	r.Define("craft-metal").
		Desc("Combine weapons or smaller metals into refined/reclaimed metal").
		Param("type", "Target metal type. 'scrap' (weapons to scrap) or 'reclaimed' (scrap to reclaimed)", false).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionCraftMetal(ctx, p)
		})

	r.Define("delete-item").
		Desc("Delete an item from backpack").
		Param("item_id", "Asset ID of the item to delete", true).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionDeleteItem(ctx, p)
		})

	r.Define("use-item").
		Desc("Use a consumable item from backpack (schema-validated: checks can_consume)").
		Param("item_id", "Asset ID of the item to use", true).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionUseItem(ctx, p)
		})

	r.Define("apply-tool").
		Desc("Apply a tool to a target item (schema-validated: checks capabilities before GC call)").
		Param("tool_id", "Asset ID of the tool (e.g. Paint, Tag, Key)", true).
		Param("item_id", "Asset ID of the target item (e.g. Cosmetic, Weapon, Crate)", true).
		Param("type", "Type: 'paint', 'nametag', 'desctag', 'strangifier', 'strange-part', 'unlock-crate', 'wrap-item', 'warpaint'", true).
		Param("value", "Custom string value (only required for 'nametag' and 'desctag')", false).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionApplyTool(ctx, p)
		})

	r.Define("set-item-style").
		Desc("Set the style index of an item").
		Param("item_id", "Asset ID of the item to style", true).
		Param("style", "Style index to set (e.g. '1')", true).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionSetItemStyle(ctx, p)
		})

	r.Define("unwrap-gift").
		Desc("Unwrap a gift item package").
		Param("item_id", "Asset ID of the gift package to unwrap", true).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionUnwrapGift(ctx, p)
		})

	r.Define("deliver-gift").
		Desc("Deliver a wrapped gift package to a target player").
		Param("gift_id", "Asset ID of the gift package to deliver", true).
		Param("target_steam_id", "64-bit SteamID of the target receiver", true).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionDeliverGift(ctx, p)
		})

	r.Define("fulfill-dynamic-recipe-component").
		Desc("Fulfill a component/ingredient of a dynamic recipe (e.g. chemistry set, fabricator)").
		Param("tool_id", "Asset ID of the recipe tool (fabricator/chemistry set)", true).
		Param("subject_id", "Asset ID of the ingredient item to consume", true).
		Param("attribute_index", "Target ingredient attribute index inside the recipe", true).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionFulfillDynamicRecipe(ctx, p)
		})

	r.Define("remove-makers-mark").
		Desc("Remove the crafted-by signature from a crafted item").
		Param("item_id", "Asset ID of the item to remove the crafted-by signature from", true).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionRemoveMakersMark(ctx, p)
		})

	r.Define("remove-gifted-by").
		Desc("Remove the gifted-by signature from an item").
		Param("item_id", "Asset ID of the item to remove the gifted-by signature from", true).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionRemoveGiftedBy(ctx, p)
		})

	r.Define("acknowledge-all").
		Desc("Acknowledge all new items in the backpack").
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionAcknowledgeAll(ctx, p)
		})

	r.Define("schema").
		Desc("Dump the current TF2 items schema raw JSON payload").
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionSchema(ctx, p)
		})

	r.Define("condense-metal").
		Desc("Condense weapons and metal to clean up backpack space").
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionCondenseMetal(ctx, p)
		})

	r.Define("make-change").
		Desc("Break down higher tier metal into smaller metal").
		Param("target_defindex", "Defindex of the metal to break down (5000 for scrap, 5001 for reclaimed)", true).
		Param("target_count", "Number of items to produce", true).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionMakeChange(ctx, p)
		})

	r.Define("smelt-weapons").
		Desc("Smelt duplicate weapons for a specific class").
		Param("class", "Class name (e.g., 'Scout', 'Soldier', 'Pyro', 'Demoman', 'Heavy', 'Engineer', 'Medic', 'Sniper', 'Spy')", true).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionSmeltWeapons(ctx, p)
		})

	r.Define("send-offer").
		Desc("Send a trade offer to a partner").
		Param("offer_params", "JSON string representing OfferParams", true).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionSendOffer(ctx, p)
		})

	r.Define("accept-offer").
		Desc("Accept an incoming trade offer").
		Param("offer_id", "Trade offer ID to accept", true).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionAcceptOffer(ctx, p)
		})

	r.Define("decline-offer").
		Desc("Decline an incoming trade offer").
		Param("offer_id", "Trade offer ID to decline", true).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionDeclineOffer(ctx, p)
		})

	r.Define("cancel-offer").
		Desc("Cancel an outgoing trade offer").
		Param("offer_id", "Trade offer ID to cancel", true).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionCancelOffer(ctx, p)
		})

	r.Define("check-escrow").
		Desc("Check if an offer is subject to escrow").
		Param("offer", "JSON string representing TradeOffer", true).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionCheckEscrow(ctx, p)
		})

	r.Define("craft").
		Desc("Execute a specific TF2 crafting recipe").
		Param("recipe", "Recipe ID (e.g., '3' for smelt class weapons)", true).
		Param("items", "JSON array of item asset IDs to use (e.g., '[101,102]')", true).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionCraft(ctx, p)
		})

	r.Define("resolve-vanity-url").
		Desc("Resolve a Steam vanity URL to a Steam64 ID").
		Param("url", "Vanity URL to resolve", true).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionResolveVanityURL(ctx, p)
		})

	r.Define("get-partner-inventory").
		Desc("Fetch the public Steam inventory of a trade partner").
		Param("partner_id", "64-bit SteamID of the partner", true).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionGetPartnerInventory(ctx, p)
		})

	r.Define("active-offers").
		Desc("Get all active incoming trade offers").
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionActiveOffers(ctx, p)
		})

	r.Define("active-sent-offers").
		Desc("Get all active outgoing trade offers").
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionActiveSentOffers(ctx, p)
		})

	r.Define("active-offers-rich").
		Desc("Get all active incoming trade offers with resolved item names (for TUI display)").
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionActiveOffersRich(ctx, p)
		})

	r.Define("active-sent-offers-rich").
		Desc("Get all active outgoing trade offers with resolved item names (for TUI display)").
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionActiveSentOffersRich(ctx, p)
		})

	r.Define("all-offers-rich").
		Desc("Get all incoming trade offers with resolved item names (active and inactive)").
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionAllOffersRich(ctx, p)
		})

	r.Define("all-sent-offers-rich").
		Desc("Get all outgoing trade offers with resolved item names (active and inactive)").
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionAllSentOffersRich(ctx, p)
		})

	r.Define("batch-delete").
		Desc("Delete multiple items from backpack at once").
		Param("item_ids", "Comma-separated list of asset IDs to delete", true).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionBatchDelete(ctx, p)
		})

	r.Define("batch-apply-tool").
		Desc("Apply a tool to multiple items at once (schema-validated per item)").
		Param("tool_id", "Asset ID of the tool to apply", true).
		Param("item_ids", "Comma-separated list of target asset IDs", true).
		Param("type", "Tool type: 'paint', 'strangifier', 'strange-part', 'autograph'", true).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionBatchApplyTool(ctx, p)
		})

	r.Define("item-details").
		Desc("Show detailed information about a specific item").
		Param("item_id", "Asset ID of the item to inspect", true).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionItemDetails(ctx, p)
		})

	r.Define("price-check").
		Desc("Check the current price of an item by SKU from pricedb").
		Param("sku", "Item SKU to check (e.g. '5021;6' for keys)", true).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionPriceCheck(ctx, p)
		})

	r.Define("inventory-stats").
		Desc("Show inventory statistics: counts by quality, section, tradability").
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionInventoryStats(ctx, p)
		})

	r.Define("craft-recipe-list").
		Desc("List available crafting recipes and operations").
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionCraftRecipeList(ctx, p)
		})

	r.Define("health-check").
		Desc("Check daemon health: GC connection, modules, memory, goroutines").
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionHealthCheck(ctx, p)
		})

	r.Define("profit-report").
		Desc("Show profit report from stats database").
		Param("stats_path", "Path to the stats JSON file", true).
		Param("days", "Number of days to include (default: 7)", false).
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionProfitReport(ctx, p)
		})

	r.Define("activity-log").
		Desc("Show recent bot activity from cache").
		Handle(func(ctx context.Context, p game.Params) (string, error) {
			return d.actionActivityLog(ctx, p)
		})
}
