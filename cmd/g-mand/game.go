// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lemon4ksan/g-man/pkg/log"

	"github.com/lemon4ksan/g-man-cli/pkg/game"
	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
)

// PlayGame launches a game session on Steam.
func (s *Daemon) PlayGame(ctx context.Context, req *pb.PlayGameRequest) (*pb.PlayGameResponse, error) {
	s.logger.Info("Play game request", log.Uint32("appid", req.GetAppid()))

	s.mu.Lock()
	oldApp := s.currentAppID
	s.currentAppID = req.GetAppid()
	s.mu.Unlock()

	if oldApp != 0 && oldApp != req.GetAppid() {
		if oldDriver, ok := s.registry.Get(oldApp); ok {
			_ = oldDriver.OnStopGC(ctx)
		}
	}

	if err := s.apps.PlayGames(ctx, []uint32{req.GetAppid()}, true); err != nil {
		s.mu.Lock()
		s.currentAppID = oldApp
		s.mu.Unlock()

		return nil, fmt.Errorf("failed to play game: %w", err)
	}

	if driver, ok := s.registry.Get(req.GetAppid()); ok {
		s.logger.Info("Initializing game coordinator session", log.Uint32("appid", req.GetAppid()))

		if err := driver.OnStartGC(ctx); err != nil {
			s.logger.Error("GC startup failed on driver", log.Uint32("appid", req.GetAppid()), log.Err(err))
		}
	}

	return &pb.PlayGameResponse{
		Message: fmt.Sprintf("Daemon is now playing game %d.", req.GetAppid()),
	}, nil
}

// ExitGame stops playing the current game and returns the bot to simple online mode.
func (s *Daemon) ExitGame(ctx context.Context, req *pb.ExitGameRequest) (*pb.ExitGameResponse, error) {
	s.mu.Lock()
	currentApp := s.currentAppID
	s.currentAppID = 0
	s.mu.Unlock()

	if currentApp == 0 {
		return &pb.ExitGameResponse{
			Message: "No game is currently active.",
		}, nil
	}

	s.logger.Info("Exit game request", log.Uint32("appid", currentApp))

	if driver, ok := s.registry.Get(currentApp); ok {
		s.logger.Info("Stopping game coordinator session", log.Uint32("appid", currentApp))

		if err := driver.OnStopGC(ctx); err != nil {
			s.logger.Error("GC shutdown failed on driver", log.Uint32("appid", currentApp), log.Err(err))
		}
	}

	if err := s.apps.StopPlaying(ctx); err != nil {
		return nil, fmt.Errorf("failed to stop playing game: %w", err)
	}

	return &pb.ExitGameResponse{
		Message: fmt.Sprintf("Successfully exited game %d.", currentApp),
	}, nil
}

// ExecAction routes dynamic commands to the active game driver.
func (s *Daemon) ExecAction(ctx context.Context, req *pb.ExecActionRequest) (*pb.ExecActionResponse, error) {
	s.logger.Info("Exec action request",
		log.Uint32("appid", req.GetAppid()),
		log.String("action", req.GetAction()),
	)

	if req.GetAction() == "memprofile" {
		profile := s.generateMemoryProfile()

		return &pb.ExecActionResponse{
			Message: "Memory profile generated successfully.",
			Details: profile,
		}, nil
	}

	driver, ok := s.registry.Get(req.GetAppid())
	if !ok {
		return nil, fmt.Errorf("no game driver registered for appid %d", req.GetAppid())
	}

	if req.GetAction() == "list-actions" {
		actions := driver.InventoryProvider().Actions()

		data, err := json.Marshal(actions)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal actions: %w", err)
		}

		return &pb.ExecActionResponse{
			Message: "Actions list retrieved successfully",
			Details: string(data),
		}, nil
	}

	s.mu.RLock()
	currentApp := s.currentAppID
	s.mu.RUnlock()

	if currentApp != req.GetAppid() {
		return nil, fmt.Errorf(
			"appid %d is not currently active (active app: %d). Play it first",
			req.GetAppid(),
			currentApp,
		)
	}

	if req.GetAction() == "inventory" {
		driver, ok := s.registry.Get(req.GetAppid())
		if !ok {
			return nil, fmt.Errorf("no game driver registered for appid %d", req.GetAppid())
		}

		items, err := driver.InventoryProvider().GetInventory(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get inventory from driver: %w", err)
		}

		return &pb.ExecActionResponse{
			Message: "Inventory fetched successfully",
			Items:   s.toProtoItems(items),
		}, nil
	}

	details, err := driver.InventoryProvider().ExecuteAction(ctx, req.GetAction(), req.GetParams())
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

func (s *Daemon) toProtoItems(items []game.Item) []*pb.Item {
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
