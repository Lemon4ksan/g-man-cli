// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package driver

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lemon4ksan/foundation/encoding/json"
	"github.com/lemon4ksan/foundation/generic"
	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	"github.com/lemon4ksan/g-man/pkg/steam/community"
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

	var offerID uint64
	if err := retryOnRetriable(ctx, func() error {
		var err error

		offerID, err = webMod.SendOffer(ctx, offerParams)

		return err
	}); err != nil {
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

	var partnerID id.ID
	if partnerIDStr, ok := params["partner_id"]; ok && partnerIDStr != "" {
		partnerID = id.Parse(partnerIDStr)
	}

	if err := retryOnRetriable(ctx, func() error {
		if partnerID != 0 {
			return webMod.AcceptOfferWithPartner(ctx, offerID, partnerID)
		}

		return webMod.AcceptOffer(ctx, offerID)
	}); err != nil {
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

	if err := retryOnRetriable(ctx, func() error {
		return webMod.DeclineOffer(ctx, offerID)
	}); err != nil {
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

	var cancelErr error
	if err := retryOnRetriable(ctx, func() error {
		return webMod.CancelOffer(ctx, offerID)
	}); err != nil {
		cancelErr = err
	}

	// Also perform cancellation via Steam Community Web endpoint if logged in
	if comm := webMod.Community(); comm != nil {
		sessionID := ""
		if sp, ok := comm.(community.SessionProvider); ok {
			sessionID = sp.SessionID(community.BaseURL)
		}

		formReq := map[string]string{
			"sessionid": sessionID,
		}

		path := fmt.Sprintf("tradeoffer/%d/cancel", offerID)
		if _, commErr := community.PostFormTo[any](ctx, comm, path, formReq); commErr == nil {
			return fmt.Sprintf("Successfully cancelled offer %d on Steam Community.", offerID), nil
		}
	}

	if cancelErr != nil {
		errStr := strings.ToLower(cancelErr.Error())
		if strings.Contains(errStr, "cannot cancel") ||
			strings.Contains(errStr, "already") ||
			strings.Contains(errStr, "invalid") ||
			strings.Contains(errStr, "not active") ||
			strings.Contains(errStr, "state") {
			return fmt.Sprintf("Offer %d is no longer active on Steam.", offerID), nil
		}

		return "", fmt.Errorf("failed to cancel offer: %w", cancelErr)
	}

	return fmt.Sprintf("Successfully cancelled offer %d.", offerID), nil
}

func (d *Driver) actionCheckEscrow(ctx context.Context, params map[string]string) (string, error) {
	webMod := web.From(d.client)
	if webMod == nil {
		return "", errors.New("web module not registered or loaded")
	}

	var targetOffer *trading.TradeOffer

	if offerIDStr, ok := params["offer_id"]; ok && offerIDStr != "" {
		offerID, err := strconv.ParseUint(offerIDStr, 10, 64)
		if err == nil && offerID != 0 {
			fetched, err := webMod.GetOffer(ctx, offerID)
			if err == nil && fetched != nil {
				targetOffer = fetched
			}
		}
	}

	if targetOffer == nil {
		offerJSON, exists := params["offer"]
		if !exists {
			return "", errors.New("check-escrow requires offer or offer_id parameter")
		}

		var offer trading.TradeOffer
		if err := json.Unmarshal([]byte(offerJSON), &offer); err != nil {
			return "", fmt.Errorf("failed to unmarshal offer: %w", err)
		}

		targetOffer = &offer

		if targetOffer.ID != 0 {
			fetched, err := webMod.GetOffer(ctx, targetOffer.ID)
			if err == nil && fetched != nil {
				targetOffer = fetched
			}
		}
	}

	hasEscrow, err := webMod.CheckEscrow(ctx, targetOffer)
	if err != nil {
		if targetOffer != nil && targetOffer.IsOurOffer {
			// Sent (outgoing) trade offer pages on Steam Community do not embed recipient escrow variables in HTML.
			return "false", nil
		}

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
				if offer.OtherSteamID != 0 && offer.OtherSteamID < id.FromAccountID(0) {
					offer.OtherSteamID = id.FromAccountID(uint32(offer.OtherSteamID)) //nolint:gosec
				}

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

func (d *Driver) actionActiveSentOffers(ctx context.Context, _ map[string]string) (string, error) {
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

func (d *Driver) actionActiveOffersRich(ctx context.Context, _ map[string]string) (string, error) {
	return d.collectOffersRich(ctx, false, true)
}

func (d *Driver) actionActiveSentOffersRich(ctx context.Context, _ map[string]string) (string, error) {
	return d.collectOffersRich(ctx, true, true)
}

func (d *Driver) actionAllOffersRich(ctx context.Context, _ map[string]string) (string, error) {
	return d.collectOffersRich(ctx, false, false)
}

func (d *Driver) actionAllSentOffersRich(ctx context.Context, _ map[string]string) (string, error) {
	return d.collectOffersRich(ctx, true, false)
}

// collectOffersRich gathers trade offers from poll data, optionally filtering
// by active state. sent=true uses the Sent map, false uses Received.
func (d *Driver) collectOffersRich(ctx context.Context, sent, activeOnly bool) (string, error) {
	webMod := web.From(d.client)
	if webMod == nil {
		return "", errors.New("web module not registered or loaded")
	}

	pollData := webMod.GetPollData()

	offerMap := pollData.Received
	if sent {
		offerMap = pollData.Sent
	}

	var offers []*trading.TradeOffer

	for offerID, state := range offerMap {
		if activeOnly && state != trading.OfferStateActive && state != trading.OfferStateCreatedNeedsConfirmation {
			continue
		}

		offer, err := webMod.GetOffer(ctx, offerID)
		if err == nil && offer != nil {
			offers = append(offers, offer)
		}
	}

	data, err := d.resolveTradeOffers(offers)
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
		partnerID.Uint64(),
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

		skuStr = stripEffectFromCrateSku(it.ToSKU(), s)
		if skuStr != "" && skuStr != "N/A" {
			skuItem, _ = sku.FromString(skuStr)
		}

		if s != nil {
			if schItem := s.ItemByDef(it.Defindex); schItem != nil {
				imgURL = schItem.ImageURL
				imgURLLarge = schItem.ImageURLLarge
			}
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
					"sku":        generic.Coalesce(skuStr, "N/A"),
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
					switch uint32(attr.Defindex) { //nolint:gosec
					case tf2.AttrCustomTextureLow:
						attrs["152"] = fmt.Sprintf("%v", attr.Value)
					case tf2.AttrUnusualEffect:
						attrs["134"] = fmt.Sprintf("%v", attr.Value)
					case tf2.AttrPaintPrimary:
						attrs["142"] = fmt.Sprintf("%v", attr.Value)
					case tf2.AttrGifterAccountID:
						attrs["186"] = fmt.Sprintf("%v", attr.Value)
					case tf2.AttrPaintSecondary:
						attrs["261"] = fmt.Sprintf("%v", attr.Value)
					case tf2.AttrCrateSeries:
						attrs["187"] = fmt.Sprintf("%v", attr.Value)
					case tf2.AttrKillEater:
						attrs["214"] = fmt.Sprintf("%v", attr.Value)
					case tf2.AttrCustomTextureHigh:
						attrs["227"] = fmt.Sprintf("%v", attr.Value)
					case tf2.AttrCrafterAccountID:
						attrs["228"] = fmt.Sprintf("%v", attr.Value)
					case tf2.AttrCraftNumber:
						attrs["229"] = fmt.Sprintf("%v", attr.Value)
					case tf2.AttrWear:
						attrs["725"] = fmt.Sprintf("%v", attr.Value)
					case tf2.AttrPaintkit:
						attrs["834"] = fmt.Sprintf("%v", attr.Value)
					case tf2.AttrPaintkitSeedLo:
						attrs["866"] = fmt.Sprintf("%v", attr.Value)
					case tf2.AttrPaintkitSeedHi:
						attrs["867"] = fmt.Sprintf("%v", attr.Value)
					case tf2.AttrTarget:
						attrs["2012"] = fmt.Sprintf("%v", attr.Value)
					case tf2.AttrKillstreaker:
						attrs["2013"] = fmt.Sprintf("%v", attr.Value)
					case tf2.AttrSheen:
						attrs["2014"] = fmt.Sprintf("%v", attr.Value)
					case tf2.AttrKillstreakTier:
						attrs["2025"] = fmt.Sprintf("%v", attr.Value)
					case tf2.AttrAustralium:
						attrs["2027"] = fmt.Sprintf("%v", attr.Value)
					case tf2.AttrQuestLoanerIDLow:
						attrs["2051"] = fmt.Sprintf("%v", attr.Value)
					case tf2.AttrQuestLoanerIDHigh:
						attrs["2052"] = fmt.Sprintf("%v", attr.Value)
					case tf2.AttrFestivized:
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

func stripEffectFromCrateSku(skuStr string, s *schema.Schema) string {
	if skuStr == "" || skuStr == "N/A" {
		return skuStr
	}

	skuItem, err := sku.FromString(skuStr)
	if err != nil || skuItem == nil || skuItem.Effect == 0 {
		return skuStr
	}

	const qualityUnusual = 5

	if s != nil {
		if schItem := s.ItemByDef(skuItem.Defindex); schItem != nil {
			if schItem.ItemClass != "supply_crate" {
				return skuStr
			}

			skuItem.Effect = 0

			return sku.FromObject(skuItem)
		}
	}

	if skuItem.Quality != qualityUnusual {
		skuItem.Effect = 0
		return sku.FromObject(skuItem)
	}

	return skuStr
}

func (d *Driver) resolveTradeOffers(offers []*trading.TradeOffer) ([]byte, error) {
	schemaMod := schema.From(d.client)
	if schemaMod == nil {
		return nil, errors.New("schema module not registered in steam client")
	}

	sch := schemaMod.Get()
	if sch == nil {
		return nil, errors.New("schema not loaded yet")
	}

	type richItem struct {
		AssetID        uint64 `json:"assetid"`
		SKU            string `json:"sku"`
		MarketHashName string `json:"market_hash_name"`
	}

	type richOffer struct {
		ID              uint64             `json:"id"`
		State           trading.OfferState `json:"state"`
		TradeOfferState trading.OfferState `json:"trade_offer_state"`
		OtherSteamID    string             `json:"other_steam_id"`
		ItemsGive       []richItem         `json:"items_to_give"`
		ItemsReceive    []richItem         `json:"items_to_receive"`
	}

	richOffers := make([]richOffer, len(offers))
	for i, o := range offers {
		partnerSteamID := o.OtherSteamID
		if partnerSteamID != 0 && partnerSteamID < id.FromAccountID(0) {
			partnerSteamID = id.FromAccountID(uint32(partnerSteamID)) //nolint:gosec
		}

		rich := richOffer{
			ID:              o.ID,
			State:           o.State,
			TradeOfferState: o.State,
			OtherSteamID:    partnerSteamID.String(),
			ItemsGive:       make([]richItem, len(o.ItemsToGive)),
			ItemsReceive:    make([]richItem, len(o.ItemsToReceive)),
		}

		for idx, it := range o.ItemsToGive {
			rich.ItemsGive[idx] = richItem{
				AssetID:        it.AssetID,
				SKU:            it.SKU,
				MarketHashName: it.MarketHashName,
			}
		}

		for idx, it := range o.ItemsToReceive {
			rich.ItemsReceive[idx] = richItem{
				AssetID:        it.AssetID,
				SKU:            it.SKU,
				MarketHashName: it.MarketHashName,
			}
		}

		richOffers[i] = rich
	}

	data, err := json.Marshal(richOffers)
	if err != nil {
		return nil, err
	}

	return data, nil
}
