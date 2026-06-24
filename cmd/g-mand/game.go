// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lemon4ksan/g-man/pkg/log"

	"github.com/lemon4ksan/g-man-cli/pkg/game"
	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
)

// PlayGame launches a game session on Steam.
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

// ExitGame stops playing the current game and returns the bot to simple online mode.
func (d *Daemon) ExitGame(ctx context.Context, req *pb.ExitGameRequest) (*pb.ExitGameResponse, error) {
	d.mu.Lock()
	currentApp := d.currentAppID
	d.currentAppID = 0
	d.desiredAppID = 0
	d.mu.Unlock()

	if currentApp == 0 {
		return &pb.ExitGameResponse{
			Message: "No game is currently active.",
		}, nil
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

// ExecAction routes dynamic commands to the active game driver.
func (d *Daemon) ExecAction(ctx context.Context, req *pb.ExecActionRequest) (*pb.ExecActionResponse, error) {
	d.logger.Info("Exec action request",
		log.Uint32("appid", req.GetAppid()),
		log.String("action", req.GetAction()),
	)

	if req.GetAction() == "memprofile" {
		profile := d.generateMemoryProfile()

		return &pb.ExecActionResponse{
			Message: "Memory profile generated successfully.",
			Details: profile,
		}, nil
	}

	driver, ok := d.registry.Get(req.GetAppid())
	if !ok {
		return nil, fmt.Errorf("no game driver registered for appid %d", req.GetAppid())
	}

	if req.GetAction() == "list-actions" {
		actions := driver.Actions()

		data, err := json.Marshal(actions)
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

	if currentApp != req.GetAppid() {
		return nil, fmt.Errorf(
			"appid %d is not currently active (active app: %d). Play it first",
			req.GetAppid(),
			currentApp,
		)
	}

	if req.GetAction() == "inventory" {
		driver, ok := d.registry.Get(req.GetAppid())
		if !ok {
			return nil, fmt.Errorf("no game driver registered for appid %d", req.GetAppid())
		}

		provider, ok := driver.(game.InventoryProvider)
		if !ok {
			return nil, errors.New("driver does not implement inventory provider")
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

	details, err := driver.ExecuteAction(ctx, req.GetAction(), req.GetParams())
	if err != nil {
		return nil, fmt.Errorf("action execution failed: %w", err)
	}

	var pbItems []*pb.Item
	if req.GetAction() == "get-partner-inventory" {
		var gItems []game.Item
		if json.Unmarshal([]byte(details), &gItems) == nil {
			pbItems = make([]*pb.Item, len(gItems))
			for i, gi := range gItems {
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
