// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lemon4ksan/miyako/log"

	"github.com/lemon4ksan/g-man-cli/pkg/game"
	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
)

var ErrDriverNotInventoryProvider = errors.New("driver does not implement inventory provider")

// PlayGame launches playing status for a game AppID on Steam and initializes the Game Coordinator.
func (d *Daemon) PlayGame(ctx context.Context, req *pb.PlayGameRequest) (*pb.PlayGameResponse, error) {
	d.logger.Info("Play game request", log.Uint32("appid", req.GetAppid()))

	d.mu.Lock()
	oldApp := d.currentAppID
	d.currentAppID = req.GetAppid()
	d.desiredAppID = req.GetAppid()
	d.mu.Unlock()

	if oldApp != 0 && oldApp != req.GetAppid() {
		if oldDriver, ok := d.registry.Get(oldApp); ok {
			_ = oldDriver.OnStopGC(ctx)
		}
	}

	if err := d.apps.PlayGames(ctx, []uint32{req.GetAppid()}, true); err != nil {
		d.mu.Lock()
		d.currentAppID = oldApp
		d.desiredAppID = oldApp
		d.mu.Unlock()

		return nil, fmt.Errorf("failed to play game: %w", err)
	}

	if driver, ok := d.registry.Get(req.GetAppid()); ok {
		d.logger.Info("Initializing game coordinator session", log.Uint32("appid", req.GetAppid()))

		if err := driver.OnStartGC(ctx); err != nil {
			d.logger.Error("GC startup failed on driver", log.Uint32("appid", req.GetAppid()), log.Err(err))
		}
	}

	return &pb.PlayGameResponse{
		Message: fmt.Sprintf("Daemon is now playing game %d.", req.GetAppid()),
	}, nil
}

// ExitGame terminates current playing session and shuts down Game Coordinator streams.
func (d *Daemon) ExitGame(ctx context.Context, req *pb.ExitGameRequest) (*pb.ExitGameResponse, error) {
	d.mu.Lock()
	currentApp := d.currentAppID
	d.currentAppID = 0
	d.desiredAppID = 0
	d.mu.Unlock()

	if currentApp == 0 {
		return &pb.ExitGameResponse{Message: "No game is currently active."}, nil
	}

	d.logger.Info("Exit game request", log.Uint32("appid", currentApp))

	if driver, ok := d.registry.Get(currentApp); ok {
		d.logger.Info("Stopping game coordinator session", log.Uint32("appid", currentApp))

		if err := driver.OnStopGC(ctx); err != nil {
			d.logger.Error("GC shutdown failed on driver", log.Uint32("appid", currentApp), log.Err(err))
		}
	}

	if err := d.apps.StopPlaying(ctx); err != nil {
		return nil, fmt.Errorf("failed to stop playing game: %w", err)
	}

	return &pb.ExitGameResponse{
		Message: fmt.Sprintf("Successfully exited game %d.", currentApp),
	}, nil
}

// ExecAction routes dynamic game actions to the registered game driver (e.g. inventory, craft-metal, delete-item).
func (d *Daemon) ExecAction(ctx context.Context, req *pb.ExecActionRequest) (*pb.ExecActionResponse, error) {
	action := req.GetAction()
	appID := req.GetAppid()

	d.logger.Info("Exec action request", log.Uint32("appid", appID), log.String("action", action))

	if action == "memprofile" {
		return &pb.ExecActionResponse{
			Message: "Memory profile generated successfully.",
			Details: d.generateMemoryProfile(),
		}, nil
	}

	driver, ok := d.registry.Get(appID)
	if !ok {
		return nil, fmt.Errorf("no game driver registered for appid %d", appID)
	}

	if action == "list-actions" {
		data, err := json.Marshal(driver.Actions())
		if err != nil {
			return nil, fmt.Errorf("failed to marshal actions: %w", err)
		}

		return &pb.ExecActionResponse{
			Message: "Actions list retrieved successfully",
			Details: string(data),
		}, nil
	}

	d.mu.RLock()
	currentApp := d.currentAppID
	d.mu.RUnlock()

	if currentApp != appID {
		return nil, fmt.Errorf("appid %d is not currently active (active app: %d). Play it first", appID, currentApp)
	}

	if action == "inventory" {
		provider, ok := driver.(game.InventoryProvider)
		if !ok {
			return nil, ErrDriverNotInventoryProvider
		}

		items, err := provider.GetInventory(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get inventory from driver: %w", err)
		}

		return &pb.ExecActionResponse{
			Message: "Inventory fetched successfully",
			Items:   d.toProtoItems(items),
		}, nil
	}

	details, err := driver.ExecuteAction(ctx, action, req.GetParams())
	if err != nil {
		return nil, fmt.Errorf("action execution failed: %w", err)
	}

	var pbItems []*pb.Item
	if action == "get-partner-inventory" {
		var gItems []game.Item
		if json.Unmarshal([]byte(details), &gItems) == nil {
			pbItems = d.toProtoItems(gItems)
		}
	}

	return &pb.ExecActionResponse{
		Message: "Operation completed successfully.",
		Details: details,
		Items:   pbItems,
	}, nil
}

func (d *Daemon) toProtoItems(items []game.Item) []*pb.Item {
	pbItems := make([]*pb.Item, len(items))
	for i, gi := range items {
		pbItems[i] = &pb.Item{
			AssetId:     gi.AssetID,
			DefIndex:    gi.DefIndex,
			Quality:     gi.Quality,
			Quantity:    gi.Quantity,
			IsTradable:  gi.IsTradable,
			IsCraftable: gi.IsCraftable,
			Attributes:  gi.Attributes,
		}
	}

	return pbItems
}
