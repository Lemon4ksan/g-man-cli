// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package driver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/pkg/trading/web"

	"github.com/lemon4ksan/g-man-cli/pkg/game"
)

func (d *Driver) actionSendOffer(ctx context.Context, params map[string]string) (string, error) {
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
}

func (d *Driver) actionAcceptOffer(ctx context.Context, params map[string]string) (string, error) {
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
}

func (d *Driver) actionDeclineOffer(ctx context.Context, params map[string]string) (string, error) {
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

	if err := webMod.DeclineOffer(ctx, offerID); err != nil {
		return "", fmt.Errorf("failed to decline offer: %w", err)
	}

	return fmt.Sprintf("Successfully declined offer %d.", offerID), nil
}

func (d *Driver) actionCancelOffer(ctx context.Context, params map[string]string) (string, error) {
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
}

func (d *Driver) actionCheckEscrow(ctx context.Context, params map[string]string) (string, error) {
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
}

func (d *Driver) actionActiveOffers(ctx context.Context, params map[string]string) (string, error) {
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
}

func (d *Driver) actionActiveSentOffers(ctx context.Context, params map[string]string) (string, error) {
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
}

func (d *Driver) actionActiveOffersRich(ctx context.Context, params map[string]string) (string, error) {
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

	data, err := d.resolveTradeOffers(activeOffers)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (d *Driver) actionActiveSentOffersRich(ctx context.Context, params map[string]string) (string, error) {
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

	data, err := d.resolveTradeOffers(activeSentOffers)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (d *Driver) actionAllOffersRich(ctx context.Context, params map[string]string) (string, error) {
	webMod := web.From(d.client)
	if webMod == nil {
		return "", errors.New("web module not registered or loaded")
	}

	pollData := webMod.GetPollData()

	var allOffers []*trading.TradeOffer

	for offerID := range pollData.Received {
		offer, err := webMod.GetOffer(ctx, offerID)
		if err == nil && offer != nil {
			allOffers = append(allOffers, offer)
		}
	}

	data, err := d.resolveTradeOffers(allOffers)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (d *Driver) actionAllSentOffersRich(ctx context.Context, params map[string]string) (string, error) {
	webMod := web.From(d.client)
	if webMod == nil {
		return "", errors.New("web module not registered or loaded")
	}

	pollData := webMod.GetPollData()

	var allSentOffers []*trading.TradeOffer

	for offerID := range pollData.Sent {
		offer, err := webMod.GetOffer(ctx, offerID)
		if err == nil && offer != nil {
			allSentOffers = append(allSentOffers, offer)
		}
	}

	data, err := d.resolveTradeOffers(allSentOffers)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (d *Driver) actionGetPartnerInventory(ctx context.Context, params map[string]string) (string, error) {
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

	schemaMod := schema.From(d.client)

	var s *schema.Schema
	if schemaMod != nil {
		s = schemaMod.Get()
	}

	remoteInv := backpack.NewRemote(
		uint64(partnerID),
		webMod.Web(),
		webMod.Community(),
		s,
	)

	items, err := remoteInv.GetItems(ctx)
	if err != nil {
		return "", err
	}

	gameItems := make([]game.Item, len(items))
	for i, it := range items {
		var (
			skuStr      string
			skuItem     *sku.Item
			imgURL      string
			imgURLLarge string
		)

		skuStr = it.ToSKU()
		if skuStr != "" && skuStr != "N/A" {
			skuItem, _ = sku.FromString(skuStr)
		}

		if s != nil {
			if schItem := s.ItemByDef(it.Defindex); schItem != nil {
				imgURL = schItem.ImageURL
				imgURLLarge = schItem.ImageURLLarge
			}
		}

		if skuStr == "" {
			skuStr = "N/A"
		}

		gameItems[i] = game.Item{
			AssetID:     it.ID,
			DefIndex:    uint32(it.Defindex), //nolint:gosec
			Quality:     uint32(it.Quality),  //nolint:gosec
			Quantity:    uint32(it.Quantity), //nolint:gosec
			IsTradable:  !it.FlagCannotTrade,
			IsCraftable: !it.FlagCannotCraft,
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

				if it.CustomName != "" {
					attrs["custom_name"] = it.CustomName
				}

				if it.CustomDesc != "" {
					attrs["custom_desc"] = it.CustomDesc
				}

				// Serialize all TF2Item attributes to the map
				for _, attr := range it.Attributes {
					switch attr.Defindex {
					case 134: // Unusual effect
						attrs["134"] = fmt.Sprintf("%v", attr.Value)
					case 142: // Paint
						attrs["142"] = fmt.Sprintf("%v", attr.Value)
					case 187: // Crate series
						attrs["187"] = fmt.Sprintf("%v", attr.Value)
					case 214: // Is elevated
						attrs["214"] = fmt.Sprintf("%v", attr.Value)
					case 229: // Craft number
						attrs["229"] = fmt.Sprintf("%v", attr.Value)
					case 725: // Wear
						attrs["725"] = fmt.Sprintf("%v", attr.Value)
					case 834: // Paintkit
						attrs["834"] = fmt.Sprintf("%v", attr.Value)
					case 866: // Paintkit seed lo
						attrs["866"] = fmt.Sprintf("%v", attr.Value)
					case 867: // Paintkit seed hi
						attrs["867"] = fmt.Sprintf("%v", attr.Value)
					case 2012: // Target
						attrs["2012"] = fmt.Sprintf("%v", attr.Value)
					case 2013: // Eye effect / killstreaker
						attrs["2013"] = fmt.Sprintf("%v", attr.Value)
					case 2014: // Sheen
						attrs["2014"] = fmt.Sprintf("%v", attr.Value)
					case 2025: // Killstreak tier
						attrs["2025"] = fmt.Sprintf("%v", attr.Value)
					case 2027: // Australium
						attrs["2027"] = fmt.Sprintf("%v", attr.Value)
					case 2053: // Festivized
						attrs["2053"] = fmt.Sprintf("%v", attr.Value)
					}
				}

				// Serialize spells and parts from TF2Item attributes
				var (
					spellParts []string
					partIDs    []string
				)

				for _, attr := range it.Attributes {
					if attr.Defindex >= 11000 && attr.Defindex < 12000 {
						// Spell proxy: defindex encodes spell attribute, value encodes spell value
						spellAttr := attr.Defindex - 11000
						spellParts = append(spellParts, fmt.Sprintf("%d:%v", spellAttr, attr.Value))
					} else if attr.Defindex >= 10000 && attr.Defindex < 11000 {
						// Parts proxy: value is the part defindex
						partIDs = append(partIDs, fmt.Sprintf("%v", attr.Value))
					}
				}

				if len(spellParts) > 0 {
					attrs["spells"] = strings.Join(spellParts, ",")
				}

				if len(partIDs) > 0 {
					attrs["parts"] = strings.Join(partIDs, ",")
				}

				// Serialize part values from SKU if available
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
}

func (d *Driver) actionResolveVanityURL(ctx context.Context, params map[string]string) (string, error) {
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
}

func (d *Driver) buildBackpackIndex() map[uint64]struct {
	sku      string
	defIndex string
} {
	tf2Mod, err := d.getTF2Module()
	if err != nil {
		return nil
	}

	idx := make(map[uint64]struct {
		sku      string
		defIndex string
	})

	for _, ri := range tf2Mod.Cache().GetItems() {
		idx[ri.ID] = struct {
			sku      string
			defIndex string
		}{
			sku:      ri.SKU,
			defIndex: strconv.FormatUint(uint64(ri.DefIndex), 10),
		}
	}

	return idx
}

func (d *Driver) resolveTradeOffers(offers []*trading.TradeOffer) ([]byte, error) {
	schemaMod := schema.From(d.client)

	var s *schema.Schema
	if schemaMod != nil {
		s = schemaMod.Get()
	}

	bpIndex := d.buildBackpackIndex()

	resolvedList := make([]ResolvedOffer, len(offers))
	for i, offer := range offers {
		ro := ResolvedOffer{
			ID:             strconv.FormatUint(offer.ID, 10),
			OtherSteamID:   offer.OtherSteamID.String(),
			Message:        offer.Message,
			State:          int(offer.State),
			IsOurOffer:     offer.IsOurOffer,
			ItemsToGive:    make([]ResolvedItem, len(offer.ItemsToGive)),
			ItemsToReceive: make([]ResolvedItem, len(offer.ItemsToReceive)),
			TimeCreated:    offer.TimeCreated,
			TimeUpdated:    offer.TimeUpdated,
		}

		for j, it := range offer.ItemsToGive {
			skuStr := ""
			defIndex := "0"

			if s != nil {
				skuItem := s.ItemFromEconItem(it)
				if skuItem != nil && skuItem.Defindex > 0 && skuItem.Defindex != 25000 {
					skuStr = sku.FromObject(skuItem)
					defIndex = strconv.Itoa(skuItem.Defindex)
				}
			}

			if skuStr == "" {
				if entry, ok := bpIndex[it.AssetID]; ok {
					skuStr = entry.sku
					defIndex = entry.defIndex
				}
			}

			if skuStr == "" {
				skuStr = it.MarketHashName
			}

			ro.ItemsToGive[j] = ResolvedItem{
				AppID:          it.AppID,
				ContextID:      strconv.FormatInt(it.ContextID, 10),
				AssetID:        strconv.FormatUint(it.AssetID, 10),
				ClassID:        strconv.FormatUint(it.ClassID, 10),
				DefIndex:       defIndex,
				InstanceID:     strconv.FormatUint(it.InstanceID, 10),
				Amount:         strconv.FormatInt(it.Amount, 10),
				MarketHashName: skuStr,
			}
		}

		for j, it := range offer.ItemsToReceive {
			skuStr := ""
			defIndex := "0"

			if s != nil {
				skuItem := s.ItemFromEconItem(it)
				if skuItem != nil && skuItem.Defindex > 0 && skuItem.Defindex != 25000 {
					skuStr = sku.FromObject(skuItem)
					defIndex = strconv.Itoa(skuItem.Defindex)
				}
			}

			if skuStr == "" {
				if entry, ok := bpIndex[it.AssetID]; ok {
					skuStr = entry.sku
					defIndex = entry.defIndex
				}
			}

			if skuStr == "" {
				skuStr = it.MarketHashName
			}

			ro.ItemsToReceive[j] = ResolvedItem{
				AppID:          it.AppID,
				ContextID:      strconv.FormatInt(it.ContextID, 10),
				AssetID:        strconv.FormatUint(it.AssetID, 10),
				ClassID:        strconv.FormatUint(it.ClassID, 10),
				DefIndex:       defIndex,
				InstanceID:     strconv.FormatUint(it.InstanceID, 10),
				Amount:         strconv.FormatInt(it.Amount, 10),
				MarketHashName: skuStr,
			}
		}

		resolvedList[i] = ro
	}

	return json.Marshal(resolvedList)
}
