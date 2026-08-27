// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/steam/social/chat"
	"github.com/lemon4ksan/g-man/pkg/steam/social/chat/commands"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/pkg/trading/web"

	gd "github.com/lemon4ksan/g-man-cli/pkg/guard/driver"
	ecp "github.com/lemon4ksan/g-man-cli/pkg/tf2/ecp"
	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
)

// Sentinel errors for trade selection.
var (
	ErrWebModuleNotLoaded   = errors.New("web module not loaded")
	ErrRefinedPriceMissing  = errors.New("refined metal is not priced and key price is missing/invalid")
	ErrItemNotPriced        = errors.New("item is not in the enabled pricelist")
	ErrInsufficientStockSKU = errors.New("insufficient stock for requested SKU")
)

var (
	prefixQtyRegex = regexp.MustCompile(`^(buy|sell)_(\d+)_(.+)$`)
	suffixQtyRegex = regexp.MustCompile(`_(?i:x)?(\d+)$`)
)

func (d *Daemon) setupChatCommands() {
	cmdMgr := commands.From(d.client)
	if cmdMgr == nil {
		d.logger.Error("Chat commands module not found in client")
		return
	}

	cmdMgr.SetTrustedSteamIDs(d.cfg.TrustedIDs)

	cmdMgr.Register("status", d.handleChatStatus,
		commands.WithDescription("Show current status, active game, uptime, and memory usage"),
	)
	cmdMgr.Register("play", d.handleChatPlay,
		commands.WithDescription("Play a Steam game by AppID (e.g. play 440)"),
		commands.WithAdmin(),
	)
	cmdMgr.Register("exit", d.handleChatExit,
		commands.WithDescription("Exit the current active Steam game"),
		commands.WithAdmin(),
	)
	cmdMgr.Register("gc", d.handleChatGC,
		commands.WithDescription("Free memory and run manual Garbage Collection"),
		commands.WithAdmin(),
	)
	cmdMgr.Register("2fa", d.handleChat2FA,
		commands.WithDescription("Generate the current Steam Guard 2FA auth code"),
		commands.WithAdmin(),
	)
	cmdMgr.Register("confs", d.handleChatConfs,
		commands.WithDescription("List all pending Steam Guard confirmations"),
		commands.WithAdmin(),
	)
	cmdMgr.Register("respond", d.handleChatRespond,
		commands.WithDescription("Respond to a confirmation (e.g. respond <id> accept|decline)"),
		commands.WithAdmin(),
	)

	go d.listenForEcpMessages(d.shutdownCtx)

	d.logger.Info("Steam Chat Command router initialized successfully")
}

func (d *Daemon) handleChatStatus(ctx context.Context, senderID uint64, args []string) (string, error) {
	status, err := d.GetStatus(ctx, &pb.GetStatusRequest{})
	if err != nil {
		return "", err
	}

	connectedStr := "Offline"
	if status.GetConnected() {
		connectedStr = "Online"
	}

	return fmt.Sprintf("🤖 Daemon Status: %s\nSteamID: %s\nUptime: %s\nMemory: %.2f MB\nActive Game: %s (ID: %d)",
		connectedStr,
		status.GetSteamId(),
		status.GetUptime(),
		float64(status.GetMemoryBytes())/1024.0/1024.0,
		status.GetCurrentAppName(),
		status.GetCurrentAppid(),
	), nil
}

func (d *Daemon) handleChatPlay(ctx context.Context, senderID uint64, args []string) (string, error) {
	if len(args) < 1 {
		return "Error: Usage: !play <appid>", nil
	}

	appID, err := strconv.ParseUint(args[0], 10, 32)
	if err != nil {
		return "Error: Invalid AppID format", nil //nolint:nilerr
	}

	resp, err := d.PlayGame(ctx, &pb.PlayGameRequest{Appid: uint32(appID)})
	if err != nil {
		return "", err
	}

	return resp.GetMessage(), nil
}

func (d *Daemon) handleChatExit(ctx context.Context, senderID uint64, args []string) (string, error) {
	resp, err := d.ExitGame(ctx, &pb.ExitGameRequest{})
	if err != nil {
		return "", err
	}

	return resp.GetMessage(), nil
}

func (d *Daemon) handleChatGC(ctx context.Context, senderID uint64, args []string) (string, error) {
	var mBefore runtime.MemStats
	runtime.ReadMemStats(&mBefore)

	runtime.GC()
	debug.FreeOSMemory()

	var mAfter runtime.MemStats
	runtime.ReadMemStats(&mAfter)

	diffMB := float64(mBefore.Alloc-mAfter.Alloc) / 1024.0 / 1024.0

	return fmt.Sprintf(
		"✅ GC Executed. Freed %.2f MB. Current Heap: %.2f MB",
		diffMB,
		float64(mAfter.Alloc)/1024.0/1024.0,
	), nil
}

func (d *Daemon) handleChat2FA(ctx context.Context, senderID uint64, args []string) (string, error) {
	resp, err := d.GuardCode(ctx, &pb.GuardCodeRequest{})
	if err != nil {
		return "", err
	}

	return "🔐 Steam Guard Code: " + resp.GetCode(), nil
}

func (d *Daemon) handleChatConfs(ctx context.Context, senderID uint64, args []string) (string, error) {
	resp, err := d.GuardList(ctx, &pb.GuardListRequest{})
	if err != nil {
		return "", err
	}

	if len(resp.GetConfirmations()) == 0 {
		return "No pending confirmations found.", nil
	}

	var sb strings.Builder
	sb.WriteString("📋 Pending Confirmations:\n")

	for _, c := range resp.GetConfirmations() {
		fmt.Fprintf(&sb, "- [%d] %s (%s): %s\n", c.GetId(), c.GetTitle(), c.GetTypeName(), c.GetDescription())
	}

	return sb.String(), nil
}

func (d *Daemon) handleChatRespond(ctx context.Context, senderID uint64, args []string) (string, error) {
	if len(args) < 2 {
		return "Error: Usage: !respond <confirmation_id> <accept|decline>", nil
	}

	confID, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return "Error: Invalid confirmation ID format", nil //nolint:nilerr
	}

	action := strings.ToLower(args[1])

	var accept bool

	switch action {
	case "accept", "y", "yes":
		accept = true
	case "decline", "cancel", "n", "no":
		accept = false
	default:
		return "Error: Action must be 'accept' or 'decline'", nil
	}

	driver := gd.New(d.client)

	var respondErr error
	if accept {
		respondErr = driver.AcceptConfirmation(ctx, confID)
	} else {
		respondErr = driver.CancelConfirmation(ctx, confID)
	}

	if respondErr != nil {
		return fmt.Sprintf("❌ Failed to respond to confirmation %d: %v", confID, respondErr), nil
	}

	actionWord := "declined"
	if accept {
		actionWord = "accepted"
	}

	return fmt.Sprintf("✅ Confirmation %d successfully %s.", confID, actionWord), nil
}

func (d *Daemon) listenForEcpMessages(ctx context.Context) {
	sub := d.client.Bus().Subscribe(&chat.MessageEvent{})
	defer sub.Unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-sub.C():
			if !ok {
				return
			}

			if mev, ok := ev.(*chat.MessageEvent); ok {
				d.handleEcpMessage(ctx, mev)
			}
		}
	}
}

func (d *Daemon) handleEcpMessage(ctx context.Context, mev *chat.MessageEvent) {
	cleanedMsg := mev.Message
	if len(cleanedMsg) > 0 && (cleanedMsg[0] == '!' || cleanedMsg[0] == '/') {
		cleanedMsg = cleanedMsg[1:]
	}

	cleanedMsg = strings.TrimSpace(cleanedMsg)

	parts := strings.Fields(cleanedMsg)
	if len(parts) == 0 {
		return
	}

	ecpStr := parts[0]
	quantity := 1

	if len(parts) > 1 {
		if q, err := strconv.Atoi(parts[1]); err == nil && q > 0 {
			quantity = q
		}
	}

	nativeEcp := d.ecpEngine.SwapToNativeChars(ecpStr)

	if match := prefixQtyRegex.FindStringSubmatch(nativeEcp); match != nil {
		if q, err := strconv.Atoi(match[2]); err == nil && q > 0 {
			quantity = q
			nativeEcp = match[1] + "_" + match[3]
		}
	} else if match := suffixQtyRegex.FindStringSubmatch(nativeEcp); match != nil {
		if q, err := strconv.Atoi(match[1]); err == nil && q > 0 {
			quantity = q
			nativeEcp = nativeEcp[:len(nativeEcp)-len(match[0])]
		}
	}

	isBuy := strings.HasPrefix(nativeEcp, "buy_")

	isSell := strings.HasPrefix(nativeEcp, "sell_")
	if !isBuy && !isSell {
		return
	}

	tf2Schema := d.schemaMgr.Get()
	if tf2Schema == nil {
		d.replyToChat(ctx, mev.SenderID, "❌ TF2 Schema is not loaded yet. Please try again in a few seconds.")
		return
	}

	d.populateEcpEngine()

	decoded, err := d.ecpEngine.ReverseEcpString(nativeEcp)
	if err != nil {
		d.replyToChat(ctx, mev.SenderID, fmt.Sprintf("❌ Error decoding ECP command: %v", err))
		return
	}

	targetSKU := tf2Schema.SkuFromName(decoded.OriginalItemName)
	if targetSKU == "" {
		d.replyToChat(
			ctx,
			mev.SenderID,
			fmt.Sprintf("❌ Could not resolve item name '%s' to SKU.", decoded.OriginalItemName),
		)

		return
	}

	d.replyToChat(
		ctx,
		mev.SenderID,
		fmt.Sprintf("⏳ Processing trade request for %d x %s...", quantity, decoded.OriginalItemName),
	)

	itemsToGive, itemsToReceive, err := d.selectTradeItems(ctx, id.ID(mev.SenderID), decoded, targetSKU, quantity)
	if err != nil {
		d.replyToChat(ctx, mev.SenderID, fmt.Sprintf("❌ Failed to match items: %v", err))
		return
	}

	webMod := web.From(d.client)
	if webMod == nil {
		d.replyToChat(ctx, mev.SenderID, "❌ Web module not loaded.")
		return
	}

	bpMod := backpack.From(d.client)

	lockIDs := make([]uint64, 0, len(itemsToGive))
	for _, it := range itemsToGive {
		lockIDs = append(lockIDs, it.AssetID)
	}

	if bpMod != nil && len(lockIDs) > 0 {
		bpMod.LockItems(lockIDs)
	}

	offerParams := trading.OfferParams{
		PartnerID:      id.ID(mev.SenderID),
		Message:        fmt.Sprintf("ECP Trade: %d x %s", quantity, decoded.OriginalItemName),
		ItemsToGive:    itemsToGive,
		ItemsToReceive: itemsToReceive,
	}

	offerID, err := webMod.SendOffer(ctx, offerParams)
	if err != nil {
		if bpMod != nil && len(lockIDs) > 0 {
			bpMod.UnlockItems(lockIDs)
		}

		d.replyToChat(ctx, mev.SenderID, fmt.Sprintf("❌ Failed to send trade offer: %v", err))

		return
	}

	d.replyToChat(ctx, mev.SenderID, fmt.Sprintf("✅ Trade offer successfully sent! Offer ID: %d.", offerID))
}

func (d *Daemon) replyToChat(ctx context.Context, steamID uint64, text string) {
	chatMgr := chat.From(d.client)
	if chatMgr != nil {
		_ = chatMgr.SendMessage(ctx, steamID, text)
	}
}

type selectableItem struct {
	ID       uint64
	SKU      string
	econItem *trading.Item
}

func (d *Daemon) selectTradeItems(
	ctx context.Context,
	partnerID id.ID,
	decoded *ecp.DecodedECP,
	targetSKU string,
	quantity int,
) (itemsToGive, itemsToReceive []*trading.Item, err error) {
	var botPool []selectableItem

	d.tf2.Cache().ForEachItem(func(it *tf2.Item) bool {
		if !it.IsTradable {
			return true
		}

		botPool = append(botPool, selectableItem{
			ID:       it.ID,
			SKU:      it.SKU,
			econItem: it.ToEconItem(),
		})

		return true
	})

	webMod := web.From(d.client)
	if webMod == nil {
		return nil, nil, ErrWebModuleNotLoaded
	}

	schemaMod := schema.From(d.client)

	var tf2Schema *schema.Schema
	if schemaMod != nil {
		tf2Schema = schemaMod.Get()
	}

	remoteInv := backpack.NewRemote(
		uint64(partnerID),
		webMod.Web(),
		webMod.Community(),
		tf2Schema,
	)

	partnerItems, err := remoteInv.GetItems(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch partner inventory: %w", err)
	}

	partnerPool := make([]selectableItem, 0, len(partnerItems))
	for _, it := range partnerItems {
		if it.FlagCannotTrade {
			continue
		}

		partnerPool = append(partnerPool, selectableItem{
			ID:       it.ID,
			SKU:      it.ToSKU(),
			econItem: it.ToEconItem(),
		})
	}

	usedBotIDs := make(map[uint64]bool)
	usedPartnerIDs := make(map[uint64]bool)

	switch decoded.DecodedIntent {
	case "buy":
		return d.selectBuyItems(botPool, partnerPool, targetSKU, quantity, usedBotIDs, usedPartnerIDs)
	case "sell":
		return d.selectSellItems(botPool, partnerPool, targetSKU, quantity, usedBotIDs, usedPartnerIDs)
	default:
		return nil, nil, fmt.Errorf("unknown decoded intent: %s", decoded.DecodedIntent)
	}
}

func (d *Daemon) selectBuyItems(
	botPool, partnerPool []selectableItem,
	targetSKU string,
	quantity int,
	usedBotIDs, usedPartnerIDs map[uint64]bool,
) (itemsToGive, itemsToReceive []*trading.Item, err error) {
	var (
		sellKeys                  int
		sellMetal, giveExtraMetal float64
	)

	if entry, ok := d.GetManualPrice(targetSKU); ok {
		sellKeys = entry.SellKeys
		sellMetal = entry.SellMetal
	} else if targetSKU == currency.SKURefined {
		keyEntry, ok := d.GetManualPrice(currency.SKUKey)
		if !ok || keyEntry.BuyMetal <= 0 {
			return nil, nil, ErrRefinedPriceMissing
		}

		sellKeys = int(math.Ceil(float64(quantity) / keyEntry.BuyMetal))
		giveExtraMetal = float64(sellKeys)*keyEntry.BuyMetal - float64(quantity)
	} else {
		return nil, nil, ErrItemNotPriced
	}

	givenItems, err := selectInventoryItems(botPool, targetSKU, quantity, usedBotIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("bot inventory: %w", err)
	}

	itemsToGive = append(itemsToGive, givenItems...)

	if giveExtraMetal > 0 {
		ref, rec, scr := metalToItems(giveExtraMetal)

		givenMetal, err := selectMetal(botPool, ref, rec, scr, usedBotIDs)
		if err != nil {
			return nil, nil, fmt.Errorf("bot inventory (change): %w", err)
		}

		itemsToGive = append(itemsToGive, givenMetal...)
	}

	if sellKeys > 0 {
		receivedKeys, err := selectInventoryItems(partnerPool, currency.SKUKey, sellKeys, usedPartnerIDs)
		if err != nil {
			return nil, nil, fmt.Errorf("partner inventory (keys): %w", err)
		}

		itemsToReceive = append(itemsToReceive, receivedKeys...)
	}

	if sellMetal > 0 {
		ref, rec, scr := metalToItems(sellMetal * float64(quantity))

		receivedMetal, err := selectMetal(partnerPool, ref, rec, scr, usedPartnerIDs)
		if err != nil {
			return nil, nil, fmt.Errorf("partner inventory (metal): %w", err)
		}

		itemsToReceive = append(itemsToReceive, receivedMetal...)
	}

	return itemsToGive, itemsToReceive, nil
}

func (d *Daemon) selectSellItems(
	botPool, partnerPool []selectableItem,
	targetSKU string,
	quantity int,
	usedBotIDs, usedPartnerIDs map[uint64]bool,
) (itemsToGive, itemsToReceive []*trading.Item, err error) {
	var (
		buyKeys, giveKeys      int
		buyMetal, giveMetalVal float64
	)

	if entry, ok := d.GetManualPrice(targetSKU); ok {
		buyKeys = entry.BuyKeys
		buyMetal = entry.BuyMetal
		giveKeys = buyKeys * quantity
		giveMetalVal = buyMetal * float64(quantity)
	} else if targetSKU == currency.SKURefined {
		keyEntry, ok := d.GetManualPrice(currency.SKUKey)
		if !ok || keyEntry.SellMetal <= 0 {
			return nil, nil, ErrRefinedPriceMissing
		}

		giveKeys = int(float64(quantity) / keyEntry.SellMetal)
		giveMetalVal = float64(quantity) - float64(giveKeys)*keyEntry.SellMetal
	} else {
		return nil, nil, ErrItemNotPriced
	}

	recd, err := selectInventoryItems(partnerPool, targetSKU, quantity, usedPartnerIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("partner inventory: %w", err)
	}

	itemsToReceive = append(itemsToReceive, recd...)

	if giveKeys > 0 {
		givenKeys, err := selectInventoryItems(botPool, currency.SKUKey, giveKeys, usedBotIDs)
		if err != nil {
			return nil, nil, fmt.Errorf("bot inventory (keys): %w", err)
		}

		itemsToGive = append(itemsToGive, givenKeys...)
	}

	if giveMetalVal > 0 {
		ref, rec, scr := metalToItems(giveMetalVal)

		givenMetal, err := selectMetal(botPool, ref, rec, scr, usedBotIDs)
		if err != nil {
			return nil, nil, fmt.Errorf("bot inventory (metal): %w", err)
		}

		itemsToGive = append(itemsToGive, givenMetal...)
	}

	return itemsToGive, itemsToReceive, nil
}

func selectInventoryItems(
	pool []selectableItem,
	targetSKU string,
	count int,
	usedIDs map[uint64]bool,
) ([]*trading.Item, error) {
	if count <= 0 {
		return nil, nil
	}

	selected := make([]*trading.Item, 0, count)
	for _, it := range pool {
		if it.SKU == targetSKU && !usedIDs[it.ID] {
			selected = append(selected, it.econItem)
			usedIDs[it.ID] = true

			if len(selected) == count {
				return selected, nil
			}
		}
	}

	return nil, fmt.Errorf("%w %s: needed %d", ErrInsufficientStockSKU, targetSKU, count)
}

func selectMetal(pool []selectableItem, ref, rec, scr int, usedIDs map[uint64]bool) ([]*trading.Item, error) {
	var selected []*trading.Item

	selectSKU := func(sku string, count int) error {
		if count <= 0 {
			return nil
		}

		items, err := selectInventoryItems(pool, sku, count, usedIDs)
		if err != nil {
			return err
		}

		selected = append(selected, items...)

		return nil
	}

	if err := selectSKU(currency.SKURefined, ref); err != nil {
		return nil, err
	}

	if err := selectSKU(currency.SKUReclaimed, rec); err != nil {
		return nil, err
	}

	if err := selectSKU(currency.SKUScrap, scr); err != nil {
		return nil, err
	}

	return selected, nil
}

func metalToItems(metalVal float64) (int, int, int) {
	scrapVal := currency.ToScrap(metalVal)
	refinedCount := int(scrapVal) / 9
	rem := scrapVal % 9
	reclaimedCount := int(rem) / 3
	scrapCount := int(rem) % 3

	return refinedCount, reclaimedCount, scrapCount
}

func (d *Daemon) populateEcpEngine() {
	tf2Schema := d.schemaMgr.Get()
	if tf2Schema == nil {
		return
	}

	registeredNames := make(map[string]bool)

	registerSKU := func(skuStr string) {
		skuItem, err := sku.FromString(skuStr)
		if err != nil {
			return
		}

		name := tf2Schema.ItemName(skuItem, true, true, false)
		if name != "" && !registeredNames[name] {
			_, _ = d.ecpEngine.ToEcpString(name, "buy")
			registeredNames[name] = true
		}
	}

	registerSKU(currency.SKUKey)
	registerSKU(currency.SKURefined)
	registerSKU(currency.SKUReclaimed)
	registerSKU(currency.SKUScrap)

	d.mu.RLock()

	for skuStr := range d.priceMgr.prices {
		registerSKU(skuStr)
	}

	d.mu.RUnlock()

	if d.tf2 != nil && d.tf2.Cache() != nil {
		d.tf2.Cache().ForEachItem(func(it *tf2.Item) bool {
			if it.SKU != "" {
				registerSKU(it.SKU)
			}

			return true
		})
	}
}
