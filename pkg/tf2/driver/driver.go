// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package driver

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	"github.com/lemon4ksan/g-man/pkg/steam"

	"github.com/lemon4ksan/g-man-cli/pkg/game"
)

// AppID returns the official TF2 AppID (440).
const AppID = tf2.AppID

// Driver acts as an adapter wrapping the official g-man-tf2 steam modules.
type Driver struct {
	client        *steam.Client
	mu            sync.Mutex
	cancelAutoAck context.CancelFunc
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
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.cancelAutoAck != nil {
		return nil
	}

	ackCtx, cancel := context.WithCancel(context.Background())
	d.cancelAutoAck = cancel

	go func() {
		tf2Mod, err := d.getTF2Module()
		if err != nil {
			return
		}

		_ = tf2Mod.AcknowledgeAll(ackCtx)

		sub := d.client.Bus().Subscribe(&tf2.ItemAcquiredEvent{})
		defer sub.Unsubscribe()

		for {
			select {
			case <-ackCtx.Done():
				return
			case _, ok := <-sub.C():
				if !ok {
					return
				}

				// Auto-acknowledge new items in the background after a tiny delay
				// to avoid hammering the GC if multiple items are acquired at once.
				time.Sleep(100 * time.Millisecond)

				// Drain any pending events in the channel
				for len(sub.C()) > 0 {
					<-sub.C()
				}

				_ = tf2Mod.AcknowledgeAll(ackCtx)
			}
		}
	}()

	return nil
}

// OnStopGC is triggered when TF2 GC is requested to close.
func (d *Driver) OnStopGC(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.cancelAutoAck != nil {
		d.cancelAutoAck()
		d.cancelAutoAck = nil
	}

	return nil
}

// GameProvider returns this adapter as the game provider.
func (d *Driver) GameProvider() game.InventoryProvider {
	return d
}

// ExecuteAction executes operations directly on the official TF2 extension or crafting manager.
func (d *Driver) ExecuteAction(ctx context.Context, action string, params map[string]string) (string, error) {
	if action == "inventory" || action == "list-backpack" {
		return d.actionInventory(ctx, params)
	}

	switch action {
	case "sort-backpack":
		return d.actionSortBackpack(ctx, params)
	case "maintenance":
		return d.actionMaintenance(ctx, params)
	case "craft-metal":
		return d.actionCraftMetal(ctx, params)
	case "delete-item":
		return d.actionDeleteItem(ctx, params)
	case "use-item":
		return d.actionUseItem(ctx, params)
	case "apply-tool":
		return d.actionApplyTool(ctx, params)
	case "set-item-style":
		return d.actionSetItemStyle(ctx, params)
	case "unwrap-gift":
		return d.actionUnwrapGift(ctx, params)
	case "deliver-gift":
		return d.actionDeliverGift(ctx, params)
	case "fulfill-dynamic-recipe-component":
		return d.actionFulfillDynamicRecipe(ctx, params)
	case "remove-makers-mark":
		return d.actionRemoveMakersMark(ctx, params)
	case "remove-gifted-by":
		return d.actionRemoveGiftedBy(ctx, params)
	case "acknowledge-all":
		return d.actionAcknowledgeAll(ctx, params)
	case "schema":
		return d.actionSchema(ctx, params)
	case "condense-metal":
		return d.actionCondenseMetal(ctx, params)
	case "make-change":
		return d.actionMakeChange(ctx, params)
	case "smelt-weapons":
		return d.actionSmeltWeapons(ctx, params)
	case "send-offer":
		return d.actionSendOffer(ctx, params)
	case "accept-offer":
		return d.actionAcceptOffer(ctx, params)
	case "decline-offer":
		return d.actionDeclineOffer(ctx, params)
	case "cancel-offer":
		return d.actionCancelOffer(ctx, params)
	case "check-escrow":
		return d.actionCheckEscrow(ctx, params)
	case "craft":
		return d.actionCraft(ctx, params)
	case "resolve-vanity-url":
		return d.actionResolveVanityURL(ctx, params)
	case "get-partner-inventory":
		return d.actionGetPartnerInventory(ctx, params)
	case "active-offers":
		return d.actionActiveOffers(ctx, params)
	case "active-sent-offers":
		return d.actionActiveSentOffers(ctx, params)
	case "active-offers-rich":
		return d.actionActiveOffersRich(ctx, params)
	case "active-sent-offers-rich":
		return d.actionActiveSentOffersRich(ctx, params)
	case "all-offers-rich":
		return d.actionAllOffersRich(ctx, params)
	case "all-sent-offers-rich":
		return d.actionAllSentOffersRich(ctx, params)
	case "targeted-smelt":
		return d.actionTargetedSmelt(ctx, params)
	case "backpack-value":
		return d.actionBackpackValue(ctx, params)
	case "memprofile":
		return d.actionMemprofile(ctx, params)
	case "batch-delete":
		return d.actionBatchDelete(ctx, params)
	case "batch-apply-tool":
		return d.actionBatchApplyTool(ctx, params)
	case "item-details":
		return d.actionItemDetails(ctx, params)
	case "price-check":
		return d.actionPriceCheck(ctx, params)
	case "inventory-stats":
		return d.actionInventoryStats(ctx, params)
	case "craft-recipe-list":
		return d.actionCraftRecipeList(ctx, params)
	case "health-check":
		return d.actionHealthCheck(ctx, params)
	case "profit-report":
		return d.actionProfitReport(ctx, params)
	case "activity-log":
		return d.actionActivityLog(ctx, params)
	default:
		return "", fmt.Errorf("unsupported action for official TF2 module: %s", action)
	}
}
