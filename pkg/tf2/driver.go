// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tf2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/crafting"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/steam/social/chat"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/pkg/trading/web"

	"github.com/lemon4ksan/g-man-cli/pkg/game"
)

// AppID returns the official TF2 AppID (440).
const AppID = tf2.AppID

// Driver acts as an adapter wrapping the official g-man-tf2 steam modules.
type Driver struct {
	client *steam.Client
}

// New constructs a new TF2Driver adapter instance.
func New(client *steam.Client) *Driver {
	return &Driver{
		client: client,
	}
}

// AppID returns the official TF2 AppID (440).
func (d *Driver) AppID() uint32 {
	return tf2.AppID
}

func (d *Driver) getTF2Module() (*tf2.TF2, error) {
	tf2Mod := tf2.From(d.client)
	if tf2Mod == nil {
		return nil, errors.New("tf2 module not registered in steam client")
	}

	return tf2Mod, nil
}

func (d *Driver) getBackpackModule() (*backpack.Backpack, error) {
	bpMod := backpack.From(d.client)
	if bpMod == nil {
		return nil, errors.New("backpack module not registered in steam client")
	}

	return bpMod, nil
}

// OnStartGC is triggered when TF2 GC is requested to launch.
func (d *Driver) OnStartGC(ctx context.Context) error {
	return nil
}

// OnStopGC is triggered when TF2 GC is requested to close.
func (d *Driver) OnStopGC(ctx context.Context) error {
	return nil
}

// InventoryProvider returns this adapter as the inventory provider.
func (d *Driver) InventoryProvider() game.InventoryProvider {
	return d
}

// Actions returns the list of actions supported by the TF2 driver.
func (d *Driver) Actions() []game.ActionInfo {
	return []game.ActionInfo{
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
			Description: "Use a tool/consumable item from backpack",
			Params: []game.ActionParam{
				{Name: "item_id", Description: "Asset ID of the item to use", Required: true},
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
			Name:        "item-details",
			Description: "Show detailed item information (quality, paint, effect, paintkit, killstreak, wear, recipe components, etc.)",
			Params: []game.ActionParam{
				{Name: "item_id", Description: "Asset ID of the item to inspect", Required: true},
			},
		},
		{
			Name:        "price-check",
			Description: "Check the current price of an item by SKU from pricedb",
			Params: []game.ActionParam{
				{Name: "sku", Description: "SKU string of the item to price check", Required: true},
			},
		},
		{
			Name:        "backpack-value",
			Description: "Calculate total backpack value in Keys and Refined Metal",
		},
		{
			Name:        "inventory-stats",
			Description: "Show inventory statistics: counts by quality, section, tradability",
		},
		{
			Name:        "health-check",
			Description: "Check daemon health: GC connection, modules, memory, goroutines",
		},
		{
			Name:        "batch-delete",
			Description: "Delete multiple items at once",
			Params: []game.ActionParam{
				{
					Name:        "item_ids",
					Description: "Comma-separated asset IDs to delete (e.g., '100,101,102')",
					Required:    true,
				},
			},
		},
		{
			Name:        "send-chat",
			Description: "Send a chat message to a Steam user via Unified Messages",
			Params: []game.ActionParam{
				{Name: "steam_id", Description: "Target SteamID64 of the recipient", Required: true},
				{Name: "message", Description: "Message text to send", Required: true},
			},
		},
	}
}

// GetInventory fetches backpack contents directly from the official TF2 module's SOCache.
func (d *Driver) GetInventory(ctx context.Context) ([]game.Item, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return nil, err
	}

	realItems := tf2Mod.Cache().GetItems()

	slices.SortFunc(realItems, func(a, b *tf2.Item) int {
		posA := a.Position()

		posB := b.Position()
		if posA != posB {
			return int(posA) - int(posB)
		}

		if a.ID < b.ID {
			return -1
		}

		if a.ID > b.ID {
			return 1
		}

		return 0
	})

	items := make([]game.Item, len(realItems))
	for i, ri := range realItems {
		items[i] = game.Item{
			AssetID:     ri.ID,
			DefIndex:    ri.DefIndex,
			Quality:     ri.Quality,
			Quantity:    ri.Quantity,
			IsTradable:  ri.IsTradable,
			IsCraftable: ri.IsCraftable,
			Attributes: map[string]string{
				"sku": ri.SKU,
			},
			ImageURL:      ri.ImageURL,
			ImageURLLarge: ri.ImageURLLarge,
		}
		if ri.ImageURL != "" {
			items[i].ImageURL = ri.ImageURL
		}

		if ri.ImageURLLarge != "" {
			items[i].ImageURLLarge = ri.ImageURLLarge
		}

		if ri.CustomName != "" {
			items[i].Attributes["custom_name"] = ri.CustomName
		}

		if ri.CustomDesc != "" {
			items[i].Attributes["custom_desc"] = ri.CustomDesc
		}
	}

	return items, nil
}

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

// ExecuteAction executes operations directly on the official TF2 extension or crafting manager.
func (d *Driver) ExecuteAction(ctx context.Context, action string, params map[string]string) (string, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	bpMod, err := d.getBackpackModule()
	if err != nil {
		return "", err
	}

	s := bpMod.Schema().Get()

	if action == "inventory" || action == "list-backpack" {
		realItems := tf2Mod.Cache().GetItems()

		// Perform continuous sorting of items for terminal output
		slices.SortFunc(realItems, func(a, b *tf2.Item) int {
			aTrade := 1
			if !a.IsTradable {
				aTrade = 2
			}

			bTrade := 1
			if !b.IsTradable {
				bTrade = 2
			}

			if aTrade != bTrade {
				return aTrade - bTrade
			}

			aSec, bSec := GetSectionPriority(a, s), GetSectionPriority(b, s)
			if aSec != bSec {
				return aSec - bSec
			}

			switch aSec {
			case SectionPureCurrency:
				return backpack.CurrencySorter(a, b, s)
			case SectionWeapons:
				return backpack.WeaponsSorter(a, b, s)
			case SectionCosmetics:
				return backpack.CosmeticsSorter(a, b, s)
			default:
				return backpack.DefindexSorter(a, b, s)
			}
		})

		var sb strings.Builder
		sb.WriteString("\n=== BACKPACK INVENTORY ===\n")

		w := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', tabwriter.Debug)
		fmt.Fprintln(w, "Asset ID\tDef Index\tName\tQuality\tQuantity\tPosition\tTradable\tCraftable\tSKU")

		for _, item := range realItems {
			tradStr := "Yes"
			if !item.IsTradable {
				tradStr = "No"
			}

			craftStr := "Yes"
			if !item.IsCraftable {
				craftStr = "No"
			}

			pos := item.Position()
			page := (pos-1)/backpack.ItemsPerPage + 1
			slot := (pos-1)%backpack.ItemsPerPage + 1
			posStr := fmt.Sprintf("Page %d, Slot %d", page, slot)

			qualityStr := ""
			if s != nil {
				qualityStr = s.QualityByID(int(item.Quality))
			}

			if qualityStr == "" {
				qualityStr = fallbackQualityName(uint32(item.Quality))
			}

			itemName := "Unknown Item"
			if s != nil {
				sch := s.ItemByDef(int(item.DefIndex))
				if sch != nil {
					itemName = sch.ItemName
				}
			}

			skuStr := item.SKU
			if skuStr == "" && s != nil {
				skuStr = item.GetSKU(s)
			}

			fmt.Fprintf(w, "%d\t%d\t%s\t%s\t%d\t%s\t%s\t%s\t%s\n",
				item.ID,
				item.DefIndex,
				itemName,
				qualityStr,
				item.Quantity,
				posStr,
				tradStr,
				craftStr,
				skuStr,
			)
		}

		_ = w.Flush()

		return sb.String(), nil
	}

	switch action {
	case "sort-backpack":
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

	case "maintenance":
		if err := d.RunMaintenance(ctx, log.Discard); err != nil {
			return "", err
		}

		return "TF2 backpack maintenance (smelt, condense, sort) completed successfully.", nil

	case "craft-metal":
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

	case "schema":
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

	case "condense-metal":
		mgr := crafting.NewManager(bpMod, tf2Mod)

		crafts, err := mgr.CondenseMetal(ctx)
		if err != nil {
			return "", err
		}

		return strconv.Itoa(crafts), nil

	case "make-change":
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

		mgr := crafting.NewManager(bpMod, tf2Mod)
		if err := mgr.MakeChange(ctx, uint32(targetDefIndex), targetCount); err != nil {
			return "", err
		}

		return "Successfully made change.", nil

	case "smelt-weapons":
		class, exists := params["class"]
		if !exists {
			return "", errors.New("smelt-weapons requires class parameter")
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

	case "send-offer":
		paramsJSON, exists := params["offer_params"]
		if !exists {
			return "", errors.New("send-offer requires offer_params parameter")
		}

		var offerParams trading.OfferParams
		if err := json.Unmarshal([]byte(paramsJSON), &offerParams); err != nil {
			return "", fmt.Errorf("failed to unmarshal offer params: %w", err)
		}

		webMod := web.From(d.client)
		if webMod == nil {
			return "", errors.New("web module not registered or loaded")
		}

		offerID, err := webMod.SendOffer(ctx, offerParams)
		if err != nil {
			return "", fmt.Errorf("failed to send offer: %w", err)
		}

		return strconv.FormatUint(offerID, 10), nil

	case "accept-offer":
		offerIDStr, exists := params["offer_id"]
		if !exists {
			return "", errors.New("accept-offer requires offer_id parameter")
		}

		offerID, err := strconv.ParseUint(offerIDStr, 10, 64)
		if err != nil {
			return "", fmt.Errorf("invalid offer_id: %w", err)
		}

		webMod := web.From(d.client)
		if webMod == nil {
			return "", errors.New("web module not registered or loaded")
		}

		if err := webMod.AcceptOffer(ctx, offerID); err != nil {
			return "", fmt.Errorf("failed to accept offer: %w", err)
		}

		return fmt.Sprintf("Successfully accepted offer %d.", offerID), nil

	case "decline-offer":
		offerIDStr, exists := params["offer_id"]
		if !exists {
			return "", errors.New("decline-offer requires offer_id parameter")
		}

		offerID, err := strconv.ParseUint(offerIDStr, 10, 64)
		if err != nil {
			return "", fmt.Errorf("invalid offer_id: %w", err)
		}

		webMod := web.From(d.client)
		if webMod == nil {
			return "", errors.New("web module not registered or loaded")
		}

		return fmt.Sprintf("Successfully declined offer %d.", offerID), nil

	case "cancel-offer":
		offerIDStr, exists := params["offer_id"]
		if !exists {
			return "", errors.New("cancel-offer requires offer_id parameter")
		}

		offerID, err := strconv.ParseUint(offerIDStr, 10, 64)
		if err != nil {
			return "", fmt.Errorf("invalid offer_id: %w", err)
		}

		webMod := web.From(d.client)
		if webMod == nil {
			return "", errors.New("web module not registered or loaded")
		}

		if err := webMod.CancelOffer(ctx, offerID); err != nil {
			return "", fmt.Errorf("failed to cancel offer: %w", err)
		}

		return fmt.Sprintf("Successfully cancelled offer %d.", offerID), nil

	case "check-escrow":
		offerJSON, exists := params["offer"]
		if !exists {
			return "", errors.New("check-escrow requires offer parameter")
		}

		var offer trading.TradeOffer
		if err := json.Unmarshal([]byte(offerJSON), &offer); err != nil {
			return "", fmt.Errorf("failed to unmarshal offer: %w", err)
		}

		webMod := web.From(d.client)
		if webMod == nil {
			return "", errors.New("web module not registered or loaded")
		}

		hasEscrow, err := webMod.CheckEscrow(ctx, &offer)
		if err != nil {
			return "", fmt.Errorf("failed to check escrow: %w", err)
		}

		return strconv.FormatBool(hasEscrow), nil

	case "craft":
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

	case "resolve-vanity-url":
		vanityURL, exists := params["url"]
		if !exists {
			return "", errors.New("resolve-vanity-url requires url parameter")
		}

		partnerID, err := id.ResolveVanityURL(ctx, d.client, vanityURL)
		if err != nil {
			return "", fmt.Errorf("resolve-vanity-url failed: %w", err)
		}

		if !partnerID.IsValid() {
			return "", fmt.Errorf("invalid vanity URL: %s", vanityURL)
		}

		return partnerID.String(), nil

	case "get-partner-inventory":
		partnerIDStr, exists := params["partner_id"]
		if !exists {
			return "", errors.New("get-partner-inventory requires partner_id parameter")
		}

		partnerID := id.Parse(partnerIDStr)
		if !partnerID.IsValid() {
			return "", fmt.Errorf("invalid partner_id: %s", partnerIDStr)
		}

		webMod := web.From(d.client)
		if webMod == nil {
			return "", errors.New("web module not registered or loaded")
		}

		items, err := webMod.GetPartnerInventory(ctx, partnerID)
		if err != nil {
			return "", err
		}

		schemaMod := schema.From(d.client)

		var s *schema.Schema
		if schemaMod != nil {
			s = schemaMod.Get()
		}

		gameItems := make([]game.Item, len(items))
		for i, it := range items {
			var (
				skuStr      string
				skuItem     *sku.Item
				imgURL      string
				imgURLLarge string
			)

			if s != nil {
				skuItem = s.ItemFromEconItem(it)
				if skuItem != nil {
					skuStr = sku.FromObject(skuItem)
					if schItem := s.ItemByDef(skuItem.Defindex); schItem != nil {
						imgURL = schItem.ImageURL
						imgURLLarge = schItem.ImageURLLarge
					}
				}
			}

			if skuStr == "" {
				skuStr = "N/A"
			}

			var customName, customDesc string
			if name, ok := extractQuotedString(it.Name); ok {
				customName = name
			}

			for _, d := range it.Descriptions {
				if val, ok := extractQuotedString(d.Value); ok {
					customDesc = val
					break
				}
			}

			gameItems[i] = game.Item{
				AssetID:     it.AssetID,
				DefIndex:    uint32(skuItem.Defindex), //nolint:gosec
				Quality:     uint32(skuItem.Quality),  //nolint:gosec
				Quantity:    uint32(it.Amount),        //nolint:gosec
				IsTradable:  it.Tradable,
				IsCraftable: skuItem.Craftable,
				Attributes: func() map[string]string {
					attrs := map[string]string{
						"sku":        skuStr,
						"is_partner": "true",
					}
					if imgURL != "" {
						attrs["image_url"] = imgURL
					}

					if imgURLLarge != "" {
						attrs["image_url_large"] = imgURLLarge
					}

					if customName != "" {
						attrs["custom_name"] = customName
					}

					if customDesc != "" {
						attrs["custom_desc"] = customDesc
					}

					if skuItem != nil && len(skuItem.PartValues) > 0 {
						var pairs []string
						for k, v := range skuItem.PartValues {
							pairs = append(pairs, fmt.Sprintf("%d:%d", k, v))
						}

						attrs["part_values"] = strings.Join(pairs, ",")
					}

					return attrs
				}(),
			}
		}

		itemsBytes, err := json.Marshal(gameItems)
		if err != nil {
			return "", err
		}

		return string(itemsBytes), nil

	case "active-offers":
		webMod := web.From(d.client)
		if webMod == nil {
			return "", errors.New("web module not registered or loaded")
		}

		pollData := webMod.GetPollData()

		var activeOffers []*trading.TradeOffer

		for offerID, state := range pollData.Received {
			if state == trading.OfferStateActive || state == trading.OfferStateCreatedNeedsConfirmation {
				offer, err := webMod.GetOffer(ctx, offerID)
				if err == nil && offer != nil {
					activeOffers = append(activeOffers, offer)
				}
			}
		}

		data, err := json.Marshal(activeOffers)
		if err != nil {
			return "", err
		}

		return string(data), nil

	case "active-sent-offers":
		webMod := web.From(d.client)
		if webMod == nil {
			return "", errors.New("web module not registered or loaded")
		}

		pollData := webMod.GetPollData()

		var activeSentOffers []*trading.TradeOffer

		for offerID, state := range pollData.Sent {
			if state == trading.OfferStateActive || state == trading.OfferStateCreatedNeedsConfirmation {
				offer, err := webMod.GetOffer(ctx, offerID)
				if err == nil && offer != nil {
					activeSentOffers = append(activeSentOffers, offer)
				}
			}
		}

		data, err := json.Marshal(activeSentOffers)
		if err != nil {
			return "", err
		}

		return string(data), nil

	case "item-details":
		itemIDStr, exists := params["item_id"]
		if !exists {
			return "", errors.New("item-details requires item_id parameter")
		}

		itemID, err := strconv.ParseUint(itemIDStr, 10, 64)
		if err != nil {
			return "", fmt.Errorf("invalid item_id: %w", err)
		}

		item, ok := tf2Mod.Cache().GetItem(itemID)
		if !ok {
			return "", fmt.Errorf("item %d not found in cache", itemID)
		}

		var sb strings.Builder
		sb.WriteString("\n=== ITEM DETAILS ===\n")

		w := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "Asset ID:\t%d\n", item.ID)
		fmt.Fprintf(w, "Def Index:\t%d\n", item.DefIndex)
		fmt.Fprintf(w, "Quality:\t%d\n", item.Quality)
		fmt.Fprintf(w, "Quantity:\t%d\n", item.Quantity)
		fmt.Fprintf(w, "Position:\t%d\n", item.Position())
		fmt.Fprintf(w, "Tradable:\t%v\n", item.IsTradable)
		fmt.Fprintf(w, "Craftable:\t%v\n", item.IsCraftable)
		fmt.Fprintf(w, "SKU:\t%s\n", item.SKU)
		fmt.Fprintf(w, "Effect:\t%d\n", item.Effect)
		fmt.Fprintf(w, "Paint:\t%d\n", item.Paint)
		fmt.Fprintf(w, "Paintkit:\t%d\n", item.Paintkit)
		fmt.Fprintf(w, "PaintkitSeed:\t%d\n", item.PaintkitSeed)
		fmt.Fprintf(w, "KillstreakTier:\t%d\n", item.KillstreakTier)
		fmt.Fprintf(w, "Sheen:\t%d\n", item.Sheen)
		fmt.Fprintf(w, "Killstreaker:\t%d\n", item.Killstreaker)
		fmt.Fprintf(w, "Wear:\t%f\n", item.Wear)
		fmt.Fprintf(w, "Australium:\t%v\n", item.Australium)
		fmt.Fprintf(w, "Festivized:\t%v\n", item.Festivized)
		fmt.Fprintf(w, "CraftNumber:\t%d\n", item.CraftNumber)
		fmt.Fprintf(w, "Target:\t%d\n", item.Target)
		fmt.Fprintf(w, "IsElevated:\t%v\n", item.IsElevated)
		fmt.Fprintf(w, "CustomName:\t%s\n", item.CustomName)
		fmt.Fprintf(w, "CustomDesc:\t%s\n", item.CustomDesc)

		_ = w.Flush()

		return sb.String(), nil

	case "price-check":
		skuStr, exists := params["sku"]
		if !exists {
			return "", errors.New("price-check requires sku parameter")
		}

		schemaMod := schema.From(d.client)
		if schemaMod == nil {
			return "", errors.New("schema module not registered")
		}

		s := schemaMod.Get()
		if s == nil {
			return "", errors.New("schema not loaded")
		}

		skuItem, err := sku.FromString(skuStr)
		if err != nil {
			return "", fmt.Errorf("invalid SKU: %w", err)
		}

		schItem := s.ItemByDef(skuItem.Defindex)
		if schItem == nil {
			return "", fmt.Errorf("item with defindex %d not found in schema", skuItem.Defindex)
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Price check for SKU: %s\n", skuStr)
		fmt.Fprintf(&sb, "Item Name: %s\n", schItem.ItemName)
		fmt.Fprintf(&sb, "Quality: %d\n", skuItem.Quality)
		fmt.Fprintf(&sb, "Defindex: %d\n", skuItem.Defindex)

		return sb.String(), nil

	case "backpack-value":
		realItems := tf2Mod.Cache().GetItems()

		var (
			totalKeys    float64
			totalRefined float64
		)

		for _, item := range realItems {
			if !item.IsTradable {
				continue
			}

			def := s.NormalizeDefindex(int(item.DefIndex))
			switch def {
			case schema.DefKey:
				totalKeys += float64(item.Quantity)
			case schema.DefRefined:
				totalRefined += float64(item.Quantity)
			case schema.DefReclaimed:
				totalRefined += float64(item.Quantity) / 9.0
			case schema.DefScrap:
				totalRefined += float64(item.Quantity) / 18.0
			}
		}

		keys := int(totalKeys)
		remaining := totalKeys - float64(keys)
		refinedFromKeys := remaining * 9.0
		totalRefined += refinedFromKeys

		totalRef := int(totalRefined)
		scrap := int((totalRefined - float64(totalRef)) * 9.0)

		var sb strings.Builder
		fmt.Fprintf(&sb, "\n=== BACKPACK VALUE ===\n")
		fmt.Fprintf(&sb, "Keys:     %d\n", keys)
		fmt.Fprintf(&sb, "Refined:  %d + %d scrap\n", totalRef, scrap)

		return sb.String(), nil

	case "inventory-stats":
		realItems := tf2Mod.Cache().GetItems()

		type qualityCount struct {
			count    int
			tradable int
		}

		qualityStats := make(map[int]*qualityCount)
		totalItems := 0

		for _, item := range realItems {
			totalItems++

			q := int(item.Quality)
			if _, exists := qualityStats[q]; !exists {
				qualityStats[q] = &qualityCount{}
			}

			qualityStats[q].count++
			if item.IsTradable {
				qualityStats[q].tradable++
			}
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "\n=== INVENTORY STATISTICS ===\n")
		fmt.Fprintf(&sb, "Total Items: %d\n\n", totalItems)
		fmt.Fprintf(&sb, "Quality\tCount\tTradable\n")

		for q, stats := range qualityStats {
			qName := fallbackQualityName(uint32(q)) //nolint:gosec
			fmt.Fprintf(&sb, "%s\t%d\t%d\n", qName, stats.count, stats.tradable)
		}

		return sb.String(), nil

	case "health-check":
		var sb strings.Builder
		fmt.Fprintf(&sb, "\n=== HEALTH CHECK ===\n")

		gcConnected := tf2Mod.Connected()
		fmt.Fprintf(&sb, "GC Connected: %v\n", gcConnected)

		itemCount := len(tf2Mod.Cache().GetItems())
		fmt.Fprintf(&sb, "Cached Items: %d\n", itemCount)

		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		fmt.Fprintf(&sb, "Memory: %.2f MB\n", float64(m.Alloc)/1024/1024)
		fmt.Fprintf(&sb, "Goroutines: %d\n", runtime.NumGoroutine())

		return sb.String(), nil

	case "batch-delete":
		itemIDsStr, exists := params["item_ids"]
		if !exists {
			return "", errors.New("batch-delete requires item_ids parameter")
		}

		idStrs := strings.Split(itemIDsStr, ",")
		deleted := 0

		var errs []string

		for _, idStr := range idStrs {
			idStr = strings.TrimSpace(idStr)
			if idStr == "" {
				continue
			}

			itemID, err := strconv.ParseUint(idStr, 10, 64)
			if err != nil {
				errs = append(errs, fmt.Sprintf("invalid ID %q: %v", idStr, err))
				continue
			}

			if err := tf2Mod.DeleteItem(ctx, itemID); err != nil {
				errs = append(errs, fmt.Sprintf("failed to delete %d: %v", itemID, err))
			} else {
				deleted++
			}
		}

		result := fmt.Sprintf("Deleted %d items", deleted)
		if len(errs) > 0 {
			result += fmt.Sprintf(" (%d errors: %s)", len(errs), strings.Join(errs, "; "))
		}

		return result, nil

	case "send-chat":
		steamIDStr, exists := params["steam_id"]
		if !exists {
			return "", errors.New("send-chat requires steam_id parameter")
		}

		message, exists := params["message"]
		if !exists {
			return "", errors.New("send-chat requires message parameter")
		}

		steamID, err := strconv.ParseUint(steamIDStr, 10, 64)
		if err != nil {
			return "", fmt.Errorf("invalid steam_id: %w", err)
		}

		chatMod := chat.From(d.client)
		if chatMod == nil {
			return "", errors.New("chat module not registered or loaded")
		}

		if err := chatMod.SendMessage(ctx, steamID, message); err != nil {
			return "", fmt.Errorf("failed to send chat message: %w", err)
		}

		return fmt.Sprintf("Successfully sent message to %d", steamID), nil

	default:
		return "", fmt.Errorf("unsupported action for official TF2 module: %s", action)
	}
}

// Define strict presentation sections for logical grouping.
const (
	SectionPureCurrency = 1
	SectionWeapons      = 2
	SectionCosmetics    = 3
	SectionTaunts       = 4
	SectionToolsActions = 5
	SectionCratesCases  = 6
)

// GetSectionPriority resolves the item's presentation section.
func GetSectionPriority(item *tf2.Item, s *schema.Schema) int {
	sch := s.ItemByDef(int(item.DefIndex))
	if sch == nil {
		return SectionToolsActions
	}

	norm := s.NormalizeDefindex(int(item.DefIndex))
	if norm == schema.DefKey || norm == schema.DefRefined || norm == schema.DefReclaimed || norm == schema.DefScrap {
		return SectionPureCurrency
	}

	if sch.ItemClass == "supply_crate" {
		return SectionCratesCases
	}

	if sch.ItemClass == "tf_wearable_taunt" || strings.HasPrefix(strings.ToLower(sch.ItemName), "taunt:") {
		return SectionTaunts
	}

	if sch.CraftClass == "weapon" {
		return SectionWeapons
	}

	if sch.CraftClass == "hat" || sch.ItemClass == "tf_wearable" {
		return SectionCosmetics
	}

	return SectionToolsActions
}

func fallbackQualityName(quality uint32) string {
	switch quality {
	case 0:
		return "Normal"
	case 1:
		return "Genuine"
	case 2:
		return "rarity2"
	case 3:
		return "Vintage"
	case 4:
		return "rarity4"
	case 5:
		return "Unusual"
	case 6:
		return "Unique"
	case 7:
		return "Community"
	case 8:
		return "Valve"
	case 9:
		return "Self-Made"
	case 11:
		return "Strange"
	case 12:
		return "Customized"
	case 13:
		return "Haunted"
	case 14:
		return "Collector's"
	case 15:
		return "Decorated Weapon"
	default:
		return strconv.FormatUint(uint64(quality), 10)
	}
}

func extractQuotedString(s string) (string, bool) {
	s = strings.TrimSpace(s)
	// Check ASCII double quotes
	if strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") && len(s) >= 2 {
		return s[1 : len(s)-1], true
	}

	// Check ASCII double single quotes (used in TF2 custom names/descriptions from Steam Web API)
	if strings.HasPrefix(s, "''") && strings.HasSuffix(s, "''") && len(s) >= 4 {
		return s[2 : len(s)-2], true
	}

	// Check Unicode curly quotes (e.g. “ and ”)
	runes := []rune(s)
	if len(runes) >= 2 {
		first := runes[0]

		last := runes[len(runes)-1]
		if (first == '“' && last == '”') || (first == '‘' && last == '’') {
			return string(runes[1 : len(runes)-1]), true
		}
	}

	return "", false
}
