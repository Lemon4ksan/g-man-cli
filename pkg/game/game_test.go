// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by aBSD-style
// license that can be found in the LICENSE file.

package game_test

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

	"github.com/lemon4ksan/g-man-cli/pkg/game"
	tf2driver "github.com/lemon4ksan/g-man-cli/pkg/tf2/driver"
)

func TestGameRegistry(t *testing.T) {
	cfg := steam.DefaultConfig()
	client, err := steam.NewClient(cfg)
	require.NoError(t, err)

	r := game.NewRegistry()
	driver := tf2driver.New(client)

	_, found := r.Get(440)
	assert.False(t, found)

	err = r.Register(driver)
	require.NoError(t, err)

	d, found := r.Get(440)
	assert.True(t, found)
	assert.Equal(t, uint32(440), d.AppID())

	err = r.Register(driver)
	assert.Error(t, err)

	list := r.List()
	assert.Len(t, list, 1)
	assert.Equal(t, uint32(440), list[0].AppID())
}

func TestTF2DriverLifecycleAndQueries(t *testing.T) {
	cfg := steam.DefaultConfig()
	client, err := steam.NewClient(
		cfg,
		apps.WithModule(),
		gc.WithModule(),
		web.WithModule(web.DefaultConfig()),
		schema.WithModule(schema.DefaultConfig()),
		tf2.WithModule(),
		backpack.WithModule(),
	)
	require.NoError(t, err)

	err = client.Run()
	require.NoError(t, err)

	defer client.Close()

	d := tf2driver.New(client)
	ctx := context.Background()

	assert.Equal(t, uint32(440), d.AppID())
	assert.NoError(t, d.OnStartGC(ctx))
	assert.NoError(t, d.OnStopGC(ctx))

	items, err := d.GetInventory(ctx)
	require.NoError(t, err)
	assert.Empty(t, items)

	assert.Equal(t, d, d.InventoryProvider())
}

func TestTF2DriverActionErrors(t *testing.T) {
	cfg := steam.DefaultConfig()
	client, err := steam.NewClient(cfg)
	require.NoError(t, err)

	d := tf2driver.New(client)
	ctx := context.Background()

	_, err = d.GetInventory(ctx)
	assert.Error(t, err)

	_, err = d.ExecuteAction(ctx, "sort-backpack", nil)
	assert.Error(t, err)
}
