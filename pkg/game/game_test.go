// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by aBSD-style
// license that can be found in the LICENSE file.

package game

import (
	"context"
	"testing"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/apps"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/gc"
	"github.com/lemon4ksan/g-man/pkg/trading/web"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGameRegistry(t *testing.T) {
	cfg := steam.DefaultConfig()
	client, err := steam.NewClient(cfg)
	require.NoError(t, err)

	r := NewRegistry()
	driver := NewTF2Driver(client)

	// Initial registry is empty
	_, found := r.Get(440)
	assert.False(t, found)

	// Register driver
	err = r.Register(driver)
	require.NoError(t, err)

	// Retrieve driver
	d, found := r.Get(440)
	assert.True(t, found)
	assert.Equal(t, uint32(440), d.AppID())

	// Registering same driver again must fail
	err = r.Register(driver)
	assert.Error(t, err)

	// List drivers
	list := r.List()
	assert.Len(t, list, 1)
	assert.Equal(t, uint32(440), list[0].AppID())
}

func TestTF2DriverLifecycleAndQueries(t *testing.T) {
	cfg := steam.DefaultConfig()
	// Register all dependencies topologically
	client, err := steam.NewClient(
		cfg,
		apps.WithModule(),
		gc.WithModule(),
		web.WithModule(web.DefaultConfig()), // trading
		schema.WithModule(schema.DefaultConfig()), // schema
		tf2.WithModule(),      // tf2
		backpack.WithModule(), // tf2_backpack
	)
	require.NoError(t, err)

	// Run the client, which executes InitAll and StartAll on all modules topologically
	err = client.Run()
	require.NoError(t, err)

	defer client.Close()

	d := NewTF2Driver(client)
	ctx := context.Background()

	// Test basic hooks
	assert.Equal(t, uint32(440), d.AppID())
	assert.NoError(t, d.OnStartGC(ctx))
	assert.NoError(t, d.OnStopGC(ctx))

	// Get inventory (should return empty list successfully as SOCache is empty initially)
	items, err := d.GetInventory(ctx)
	require.NoError(t, err)
	assert.Empty(t, items)

	// Verify inventory provider method returns the driver adapter
	assert.Equal(t, d, d.InventoryProvider())
}

func TestTF2DriverActionErrors(t *testing.T) {
	cfg := steam.DefaultConfig()
	client, err := steam.NewClient(cfg) // Modules not registered
	require.NoError(t, err)

	d := NewTF2Driver(client)
	ctx := context.Background()

	// Operations should fail since the tf2 module is missing in the steam client
	_, err = d.GetInventory(ctx)
	assert.Error(t, err)

	_, err = d.ExecuteAction(ctx, "sort-backpack", nil)
	assert.Error(t, err)
}
