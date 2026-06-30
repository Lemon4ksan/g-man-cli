// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package driver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"

	"github.com/lemon4ksan/g-man-cli/pkg/game"
)

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
		attrs := map[string]string{
			"sku": ri.SKU,
		}

		if ri.CustomName != "" {
			attrs["custom_name"] = ri.CustomName
		}

		if ri.CustomDesc != "" {
			attrs["custom_desc"] = ri.CustomDesc
		}

		if ri.DecalUGCID != 0 {
			attrs["152"] = strconv.FormatUint(ri.DecalUGCID, 10)
			attrs["227"] = strconv.FormatUint(ri.DecalUGCID>>32, 10)
		}

		// Attribute ID keys matching schema attribute definitions
		if ri.Effect != 0 {
			attrs["134"] = strconv.FormatUint(uint64(ri.Effect), 10)
		}

		if ri.PaintPrimary != 0 {
			attrs["142"] = strconv.FormatUint(uint64(ri.PaintPrimary), 10)
		}

		if ri.PaintSecondary != 0 {
			attrs["261"] = strconv.FormatUint(uint64(ri.PaintSecondary), 10)
		}

		if ri.CrateSeries != 0 {
			attrs["187"] = strconv.FormatUint(uint64(ri.CrateSeries), 10)
		}

		if ri.KillstreakTier != 0 {
			attrs["2025"] = strconv.FormatUint(uint64(ri.KillstreakTier), 10)
		}

		if ri.Australium {
			attrs["2027"] = "1"
		}

		if ri.Festivized {
			attrs["2053"] = "1"
		}

		if ri.Wear != 0 {
			attrs["725"] = fmt.Sprintf("%f", ri.Wear)
		}

		if ri.Paintkit != 0 {
			attrs["834"] = strconv.FormatUint(uint64(ri.Paintkit), 10)
		}

		if ri.PaintkitSeed != 0 {
			attrs["866"] = strconv.FormatUint(ri.PaintkitSeed, 10)
			attrs["867"] = strconv.FormatUint(ri.PaintkitSeed>>32, 10)
		}

		if ri.Sheen != 0 {
			attrs["2014"] = strconv.FormatUint(uint64(ri.Sheen), 10)
		}

		if ri.Killstreaker != 0 {
			attrs["2013"] = strconv.FormatUint(uint64(ri.Killstreaker), 10)
		}

		if ri.CrafterAccountID != 0 {
			attrs["228"] = strconv.FormatUint(uint64(ri.CrafterAccountID), 10)
		}

		if ri.CraftNumber != 0 {
			attrs["229"] = strconv.FormatUint(uint64(ri.CraftNumber), 10)
		}

		if ri.Series != 0 {
			attrs["187"] = strconv.FormatUint(uint64(ri.Series), 10)
		}

		if ri.Target != 0 {
			attrs["2012"] = strconv.FormatUint(uint64(ri.Target), 10)
		}

		if ri.QuestID != 0 {
			attrs["2051"] = strconv.FormatUint(ri.QuestID, 10)
			attrs["2052"] = strconv.FormatUint(ri.QuestID>>32, 10)
		}

		if ri.GifterAccountID != 0 {
			attrs["186"] = strconv.FormatUint(uint64(ri.GifterAccountID), 10)
		}

		if ri.IsElevated {
			attrs["214"] = "1"
		}

		// Serialize spells as comma-separated "attr:value" pairs
		if len(ri.Spells) > 0 {
			var spellParts []string
			for _, sp := range ri.Spells {
				spellParts = append(spellParts, fmt.Sprintf("%d:%d", sp.Attribute, sp.Value))
			}

			attrs["spells"] = strings.Join(spellParts, ",")
		}

		// Serialize parts as comma-separated IDs
		if len(ri.Parts) > 0 {
			var partStrs []string
			for _, p := range ri.Parts {
				partStrs = append(partStrs, strconv.FormatUint(uint64(p), 10))
			}

			attrs["parts"] = strings.Join(partStrs, ",")
		}

		// Part values as "id:value" pairs
		if len(ri.PartValues) > 0 {
			var pvParts []string
			for k, v := range ri.PartValues {
				pvParts = append(pvParts, fmt.Sprintf("%d:%d", k, v))
			}

			attrs["part_values"] = strings.Join(pvParts, ",")
		}

		// Recipe components (fabricator/chemistry set) as JSON
		if len(ri.RecipeComponents) > 0 {
			if compJSON, err := json.Marshal(ri.RecipeComponents); err == nil {
				attrs["recipe_components"] = string(compJSON)
			}
		}

		items[i] = game.Item{
			AssetID:       ri.ID,
			DefIndex:      ri.DefIndex,
			Quality:       ri.Quality,
			Quantity:      ri.Quantity,
			IsTradable:    ri.IsTradable,
			IsCraftable:   ri.IsCraftable,
			Attributes:    attrs,
			ImageURL:      ri.ImageURL,
			ImageURLLarge: ri.ImageURLLarge,
		}
	}

	return items, nil
}

func (d *Driver) actionInventory(_ context.Context, _ map[string]string) (string, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	bpMod, err := d.getBackpackModule()
	if err != nil {
		return "", err
	}

	s := bpMod.Schema().Get()

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

func (d *Driver) actionInventoryStats(_ context.Context, _ map[string]string) (string, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	bpMod, err := d.getBackpackModule()
	if err != nil {
		return "", err
	}

	s := bpMod.Schema().Get()

	realItems := tf2Mod.Cache().GetItems()

	totalCount := len(realItems)
	byQuality := make(map[string]int)
	bySection := make(map[string]int)
	tradableCount := 0
	craftableCount := 0
	unusualCount := 0

	sectionNames := map[int]string{
		SectionPureCurrency: "Currency",
		SectionWeapons:      "Weapons",
		SectionCosmetics:    "Cosmetics",
		SectionTaunts:       "Taunts",
		SectionToolsActions: "Tools/Actions",
		SectionCratesCases:  "Crates/Cases",
	}

	for _, item := range realItems {
		qName := fallbackQualityName(item.Quality)
		if s != nil {
			if q := s.QualityByID(int(item.Quality)); q != "" {
				qName = q
			}
		}

		byQuality[qName]++

		sec := GetSectionPriority(item, s)
		secName := sectionNames[sec]

		if secName == "" {
			secName = "Other"
		}

		bySection[secName]++

		if item.IsTradable {
			tradableCount++
		}

		if item.IsCraftable {
			craftableCount++
		}

		if item.Quality == schema.QualityUnusual {
			unusualCount++
		}
	}

	var sb strings.Builder
	sb.WriteString("\n=== INVENTORY STATS ===\n")
	fmt.Fprintf(&sb, "Total Items:    %d\n", totalCount)
	fmt.Fprintf(&sb, "Tradable:       %d\n", tradableCount)
	fmt.Fprintf(&sb, "Non-Tradable:   %d\n", totalCount-tradableCount)
	fmt.Fprintf(&sb, "Craftable:      %d\n", craftableCount)
	fmt.Fprintf(&sb, "Unusuals:       %d\n", unusualCount)
	sb.WriteString("\nBy Quality:\n")

	qNames := make([]string, 0, len(byQuality))
	for q := range byQuality {
		qNames = append(qNames, q)
	}

	slices.Sort(qNames)

	for _, q := range qNames {
		fmt.Fprintf(&sb, "  %-20s %d\n", q, byQuality[q])
	}

	sb.WriteString("\nBy Section:\n")

	sNames := make([]string, 0, len(bySection))
	for sn := range bySection {
		sNames = append(sNames, sn)
	}

	slices.Sort(sNames)

	for _, sn := range sNames {
		fmt.Fprintf(&sb, "  %-20s %d\n", sn, bySection[sn])
	}

	sb.WriteString("=======================\n")

	return sb.String(), nil
}

func (d *Driver) actionItemDetails(ctx context.Context, params map[string]string) (string, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	bpMod, err := d.getBackpackModule()
	if err != nil {
		return "", err
	}

	s := bpMod.Schema().Get()

	idStr, exists := params["item_id"]
	if !exists {
		return "", errors.New("item-details requires item_id parameter")
	}

	itemID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid item_id: %w", err)
	}

	var found *tf2.Item
	for _, it := range tf2Mod.Cache().GetItems() {
		if it.ID == itemID {
			found = it
			break
		}
	}

	if found == nil {
		return "", fmt.Errorf("item %d not found in backpack", itemID)
	}

	var sb strings.Builder
	sb.WriteString("\n=== ITEM DETAILS ===\n")

	var sch *schema.Item

	itemName := "Unknown"
	if s != nil {
		sch = s.ItemByDef(int(found.DefIndex))
		if sch != nil {
			itemName = sch.ItemName
		}
	}

	qualityStr := fallbackQualityName(found.Quality)
	if s != nil {
		if qName := s.QualityByID(int(found.Quality)); qName != "" {
			qualityStr = qName
		}
	}

	skuStr := found.GetSKU(s)

	fmt.Fprintf(&sb, "Name:        %s\n", itemName)
	fmt.Fprintf(&sb, "Asset ID:    %d\n", found.ID)
	fmt.Fprintf(&sb, "Def Index:   %d\n", found.DefIndex)
	fmt.Fprintf(&sb, "SKU:         %s\n", skuStr)
	fmt.Fprintf(&sb, "Quality:     %s (%d)\n", qualityStr, found.Quality)
	fmt.Fprintf(&sb, "Quantity:    %d\n", found.Quantity)
	fmt.Fprintf(&sb, "Tradable:    %v\n", found.IsTradable)
	fmt.Fprintf(&sb, "Craftable:   %v\n", found.IsCraftable)

	if found.PaintPrimary > 0 {
		paintName := schema.GetPaintName(found.PaintPrimary)
		if paintName != "" {
			fmt.Fprintf(&sb, "Paint:       %s (0x%06X)\n", paintName, found.PaintPrimary)
		} else {
			fmt.Fprintf(&sb, "Paint:       0x%06X\n", found.PaintPrimary)
		}
	}

	if found.PaintSecondary > 0 {
		paintName := schema.GetPaintName(found.PaintSecondary)
		if paintName != "" {
			fmt.Fprintf(&sb, "Paint Sec:   %s (0x%06X)\n", paintName, found.PaintSecondary)
		} else {
			fmt.Fprintf(&sb, "Paint Sec:   0x%06X\n", found.PaintSecondary)
		}
	}

	if found.Australium {
		fmt.Fprintf(&sb, "Australium:  Yes\n")
	}

	if found.Quality == schema.QualityStrange {
		fmt.Fprintf(&sb, "Strange:     Yes\n")
	}

	if found.Wear > 0 {
		fmt.Fprintf(&sb, "Wear:        %.4f\n", found.Wear)
	}

	if found.Paintkit > 0 {
		paintkitName := ""
		if s != nil {
			paintkitName = s.SkinByID(int(found.Paintkit))
		}

		if paintkitName != "" {
			fmt.Fprintf(&sb, "Paintkit:    %s (%d)\n", paintkitName, found.Paintkit)
		} else {
			fmt.Fprintf(&sb, "Paintkit:    %d\n", found.Paintkit)
		}
	}

	if found.PaintkitSeed > 0 {
		fmt.Fprintf(&sb, "Seed:        %d\n", found.PaintkitSeed)
	}

	if found.KillstreakTier > 0 {
		fmt.Fprintf(&sb, "Killstreak:  Tier %d\n", found.KillstreakTier)
	}

	if found.Effect > 0 {
		effectName := ""
		if s != nil {
			effectName = s.EffectByID(int(found.Effect))
		}

		if effectName != "" {
			fmt.Fprintf(&sb, "Effect:      %s (%d)\n", effectName, found.Effect)
		} else {
			fmt.Fprintf(&sb, "Effect:      %d\n", found.Effect)
		}
	}

	if found.Festivized {
		fmt.Fprintf(&sb, "Festivized:  Yes\n")
	}

	if found.CustomName != "" {
		fmt.Fprintf(&sb, "Custom Name: %s\n", found.CustomName)
	}

	if found.CustomDesc != "" {
		fmt.Fprintf(&sb, "Custom Desc: %s\n", found.CustomDesc)
	}

	if found.CrafterAccountID != 0 {
		fmt.Fprintf(&sb, "Crafter ID:  %d\n", found.CrafterAccountID)
	}

	if found.GifterAccountID != 0 {
		fmt.Fprintf(&sb, "Gifter ID:   %d\n", found.GifterAccountID)
	}

	if found.QuestID != 0 {
		fmt.Fprintf(&sb, "Quest ID:    %d\n", found.QuestID)
	}

	if found.DecalUGCID != 0 {
		fmt.Fprintf(&sb, "Decal UGC ID:%d\n", found.DecalUGCID)
	}

	// Schema-derived information
	if sch != nil {
		if sch.Origin != 0 {
			fmt.Fprintf(&sb, "Origin:      %d\n", sch.Origin)
		}

		if slot := sch.GetLoadoutSlot(); slot != schema.LoadoutInvalid {
			fmt.Fprintf(&sb, "Loadout Slot:%d\n", slot)
		}

		if sch.ItemSlot != "" {
			fmt.Fprintf(&sb, "Weapon Slot: %s\n", sch.ItemSlot)
		}

		if sch.HasFlag(schema.FlagNonEconomy) {
			fmt.Fprintf(&sb, "Non-Economy: Yes\n")
		}

		if sch.HasFlag(schema.FlagCanBeTradedByFreeAccounts) {
			fmt.Fprintf(&sb, "Free-Tradable: Yes\n")
		}

		if sch.Capabilities != nil {
			var caps []string
			for _, toolType := range []string{"paint", "nametag", "desctag", "strangifier", "strange-part", "killstreak", "gift-wrap"} {
				if sch.Capabilities.CanApplyTool(toolType) {
					caps = append(caps, toolType)
				}
			}

			if len(caps) > 0 {
				fmt.Fprintf(&sb, "Applicable Tools: %s\n", strings.Join(caps, ", "))
			}

			if sch.Capabilities.CanConsume {
				fmt.Fprintf(&sb, "Consumable:  Yes\n")
			}
		}
	}

	// Recipe components (fabricator/chemistry set)
	if len(found.RecipeComponents) > 0 {
		sb.WriteString("\n--- Recipe Components ---\n")

		for _, comp := range found.RecipeComponents {
			role := "INPUT"
			if comp.IsOutput() {
				role = "OUTPUT"
			}

			status := fmt.Sprintf("%d/%d", comp.NumFulfilled, comp.NumRequired)
			if comp.IsComplete() {
				status += " DONE"
			}

			reqStr := "any item"
			if comp.HasDefIndex() {
				itemName := "Unknown"
				if s != nil {
					if sch := s.ItemByDef(int(comp.DefIndex)); sch != nil {
						itemName = sch.ItemName
					}
				}

				reqStr = fmt.Sprintf("%s (defindex %d)", itemName, comp.DefIndex)
			}

			qualStr := ""
			if comp.HasQuality() {
				if s != nil {
					qualStr = s.QualityByID(int(comp.Quality))
				}

				if qualStr == "" {
					qualStr = fmt.Sprintf("quality %d", comp.Quality)
				}

				reqStr += fmt.Sprintf(" [%s]", qualStr)
			}

			fmt.Fprintf(&sb, "  Slot %d: %s %s (%s)\n", comp.SlotIndex, role, reqStr, status)
		}
	}

	sb.WriteString("=====================\n")

	return sb.String(), nil
}

func (d *Driver) actionBackpackValue(ctx context.Context, params map[string]string) (string, error) {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return "", err
	}

	bpMod, err := d.getBackpackModule()
	if err != nil {
		return "", err
	}

	s := bpMod.Schema().Get()

	realItems := tf2Mod.Cache().GetItems()
	if len(realItems) == 0 {
		return "Backpack is empty.", nil
	}

	skuMap := make(map[string]bool)
	for _, item := range realItems {
		skuStr := item.SKU
		if skuStr == "" && s != nil {
			skuStr = item.GetSKU(s)
		}

		if skuStr != "" {
			skuMap[sku.ToPricingSKU(skuStr)] = true
		}
	}

	skuMap["5021;6"] = true

	skus := make([]string, 0, len(skuMap))
	for k := range skuMap {
		skus = append(skus, k)
	}

	pdbClient := d.getPDBClient()

	pricesSlice, err := pdbClient.GetItemsBulk(ctx, skus)
	if err != nil {
		return "", fmt.Errorf("failed to fetch prices from pricedb: %w", err)
	}

	prices := make(map[string]*pricedb.Price)
	for _, p := range pricesSlice {
		if p != nil {
			prices[p.SKU] = p
		}
	}

	var (
		totalKeysBuy, totalKeysSell int
		totalRefBuy, totalRefSell   float64
		itemCount                   int
		unpricedCount               int
	)

	for _, item := range realItems {
		itemCount++

		skuStr := item.SKU
		if skuStr == "" && s != nil {
			skuStr = item.GetSKU(s)
		}

		if skuStr == "5021;6" { // Key
			totalKeysBuy += int(item.Quantity)
			totalKeysSell += int(item.Quantity)
			continue
		}

		if skuStr == "5002;6" { // Refined
			totalRefBuy += float64(item.Quantity) * 1.0
			totalRefSell += float64(item.Quantity) * 1.0
			continue
		}

		if skuStr == "5001;6" { // Reclaimed
			totalRefBuy += float64(item.Quantity) * (1.0 / 3.0)
			totalRefSell += float64(item.Quantity) * (1.0 / 3.0)
			continue
		}

		if skuStr == "5000;6" { // Scrap
			totalRefBuy += float64(item.Quantity) * (1.0 / 9.0)
			totalRefSell += float64(item.Quantity) * (1.0 / 9.0)
			continue
		}

		price, exists := prices[sku.ToPricingSKU(skuStr)]
		if exists && price != nil {
			totalKeysBuy += price.Buy.Keys * int(item.Quantity)
			totalRefBuy += price.Buy.Metal * float64(item.Quantity)

			totalKeysSell += price.Sell.Keys * int(item.Quantity)
			totalRefSell += price.Sell.Metal * float64(item.Quantity)
		} else {
			// Fallback for weapons
			if s != nil {
				sch := s.ItemByDef(int(item.DefIndex))
				if sch != nil && sch.CraftClass == "weapon" {
					totalRefBuy += 0.05 * float64(item.Quantity)
					totalRefSell += 0.05 * float64(item.Quantity)
					continue
				}
			}

			unpricedCount++
		}
	}

	keyPriceBuy, keyPriceSell := 70.0, 70.0 // conservative fallback
	if keyPriceEntry, exists := prices["5021;6"]; exists && keyPriceEntry != nil {
		if keyPriceEntry.Buy.Metal > 0 {
			keyPriceBuy = keyPriceEntry.Buy.Metal
		}

		if keyPriceEntry.Sell.Metal > 0 {
			keyPriceSell = keyPriceEntry.Sell.Metal
		}
	}

	combinedBuy := float64(totalKeysBuy) + (totalRefBuy / keyPriceBuy)
	combinedSell := float64(totalKeysSell) + (totalRefSell / keyPriceSell)

	var sb strings.Builder
	sb.WriteString("\n=== BACKPACK VALUATION ===\n")
	fmt.Fprintf(&sb, "Total Items:        %d\n", itemCount)
	fmt.Fprintf(&sb, "Unpriced Items:     %d\n", unpricedCount)
	fmt.Fprintf(&sb, "Key Rate (Buy/Sell): %.2f ref / %.2f ref\n", keyPriceBuy, keyPriceSell)
	sb.WriteString("---------------------------\n")
	fmt.Fprintf(&sb, "Buy Valuation:\n")
	fmt.Fprintf(&sb, "  Pure Metal:       %.2f ref\n", totalRefBuy)
	fmt.Fprintf(&sb, "  Pure Keys:        %d keys\n", totalKeysBuy)
	fmt.Fprintf(&sb, "  Combined Value:   %.2f keys\n", combinedBuy)
	sb.WriteString("---------------------------\n")
	fmt.Fprintf(&sb, "Sell Valuation:\n")
	fmt.Fprintf(&sb, "  Pure Metal:       %.2f ref\n", totalRefSell)
	fmt.Fprintf(&sb, "  Pure Keys:        %d keys\n", totalKeysSell)
	fmt.Fprintf(&sb, "  Combined Value:   %.2f keys\n", combinedSell)
	sb.WriteString("===========================\n")

	return sb.String(), nil
}
