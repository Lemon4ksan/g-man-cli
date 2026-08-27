// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package driver

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/lemon4ksan/aoni"
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
	router        *game.ActionRouter
	mu            sync.Mutex
	cancelAutoAck context.CancelFunc
	priceDBDoer   aoni.RequestDoer // optional; nil means use default
}

// New constructs a new TF2Driver adapter instance.
func New(client *steam.Client) *Driver {
	d := &Driver{
		client: client,
		router: game.NewActionRouter(),
	}
	d.initRouter()

	return d
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

		// Subscribe to both BackpackLoadedEvent (initial bulk load from SOCacheSubscribed)
		// and ItemAcquiredEvent (individual items from trades/drops). The bulk load
		// does NOT publish ItemAcquiredEvent, so we must listen for both.
		sub := d.client.Bus().Subscribe(&tf2.BackpackLoadedEvent{}, &tf2.ItemAcquiredEvent{})
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
				// to avoid hammering the GC if multiple items arrive at once.
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
	return d.router.Execute(ctx, action, params)
}

// Actions returns the list of actions supported by the TF2 driver.
func (d *Driver) Actions() []game.ActionInfo {
	return d.router.Actions()
}
