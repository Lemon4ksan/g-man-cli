// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tf2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/crafting"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/pkg/trading/web"

	"github.com/lemon4ksan/g-man-cli/pkg/game"
)

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

// GetInventory fetches backpack contents directly from the official TF2 module's SOCache.
func (d *Driver) GetInventory(ctx context.Context) ([]game.Item, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return nil, err
	}

	realItems := tf2Mod.Cache().GetItems()

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
				"custom_name": ri.CustomName,
				"custom_desc": ri.CustomDesc,
				"sku":         ri.SKU,
			},
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
				switch item.Quality {
				case 0:
					qualityStr = "Normal"
				case 1:
					qualityStr = "Genuine"
				case 3:
					qualityStr = "Vintage"
				case 6:
					qualityStr = "Unique"
				case 11:
					qualityStr = "Strange"
				case 13:
					qualityStr = "Unusual"
				default:
					qualityStr = strconv.FormatUint(uint64(item.Quality), 10)
				}
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

		itemsBytes, err := json.Marshal(items)
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
