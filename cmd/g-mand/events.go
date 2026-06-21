// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam/auth"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/apps"
	"github.com/lemon4ksan/g-man/pkg/trading/web"

	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
)

// StreamEvents broadcasts the event bus notifications as a gRPC stream.
func (s *Daemon) StreamEvents(req *pb.StreamEventsRequest, stream pb.DaemonService_StreamEventsServer) error {
	sub := s.client.Bus().Subscribe(
		&tf2.BackpackLoadedEvent{},
		&tf2.ItemUpdatedEvent{},
		&tf2.ItemAcquiredEvent{},
		&tf2.ItemRemovedEvent{},
		&tf2.ConnectedEvent{},
		&tf2.DisconnectedEvent{},
		&web.NewOfferEvent{},
		&web.OfferChangedEvent{},
		&auth.SteamGuardRequiredEvent{},
	)
	defer sub.Unsubscribe()

	s.logger.Info("Client connected to daemon event stream")

	for {
		select {
		case <-stream.Context().Done():
			s.logger.Info("Client disconnected from event stream")
			return stream.Context().Err()
		case <-s.shutdownCtx.Done():
			return errors.New("daemon shutting down")
		case ev, ok := <-sub.C():
			if !ok {
				return nil
			}

			var (
				payloadBytes []byte
				err          error
			)

			if guardEv, ok := ev.(*auth.SteamGuardRequiredEvent); ok {
				payloadBytes, err = json.Marshal(struct {
					IsAppConfirm bool   `json:"is_app_confirm"`
					Is2FA        bool   `json:"is_2fa"`
					EmailDomain  string `json:"email_domain"`
				}{
					IsAppConfirm: guardEv.IsAppConfirm,
					Is2FA:        guardEv.Is2FA,
					EmailDomain:  guardEv.EmailDomain,
				})
			} else {
				payloadBytes, err = json.Marshal(ev)
			}

			if err != nil {
				s.logger.Error("Failed to marshal event for stream", log.Err(err))
				continue
			}

			eventType := fmt.Sprintf("%T", ev)
			resp := &pb.StreamEventsResponse{
				EventId:     strconv.FormatInt(time.Now().UnixNano(), 10),
				EventType:   eventType,
				PayloadJson: string(payloadBytes),
				Timestamp:   time.Now().Unix(),
			}

			if err := stream.Send(resp); err != nil {
				s.logger.Error("Failed to send event to stream", log.Err(err))
				return err
			}
		}
	}
}

func (s *Daemon) handleEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-s.sub.C():
			if !ok {
				return
			}

			switch ev := event.(type) {
			case *auth.LoggedOnEvent:
				s.logger.Info("Login successful", log.Uint64("steam_id", ev.SteamID))
			case *auth.LoggedOffEvent:
				s.logger.Info("Logged off")
			case *apps.AppLaunchedEvent:
				s.onAppLaunched(ctx, ev.AppID)
			case *apps.AppQuitEvent:
				s.onAppQuit(ctx, ev.AppID)
			case *auth.SteamGuardRequiredEvent:
				s.activeAuthCallbackMu.Lock()
				s.activeAuthCallback = ev.Callback
				s.activeAuthCallbackMu.Unlock()
				s.logger.Info(
					"Steam Guard code required",
					log.Bool("is_2fa", ev.Is2FA),
					log.String("domain", ev.EmailDomain),
				)
			}
		}
	}
}

func (s *Daemon) onAppLaunched(ctx context.Context, appID uint32) {
	s.mu.Lock()
	s.currentAppID = appID
	s.mu.Unlock()

	s.logger.Info("Detected game launched", log.Uint32("appid", appID))

	if driver, ok := s.registry.Get(appID); ok {
		s.logger.Info("Initializing game coordinator session for auto-launched game", log.Uint32("appid", appID))

		if err := driver.OnStartGC(ctx); err != nil {
			s.logger.Error("GC startup failed on driver", log.Uint32("appid", appID), log.Err(err))
		}
	}
}

func (s *Daemon) onAppQuit(ctx context.Context, appID uint32) {
	s.mu.Lock()
	if s.currentAppID == appID {
		s.currentAppID = 0
	}

	s.mu.Unlock()

	s.logger.Info("Detected game quit", log.Uint32("appid", appID))

	if driver, ok := s.registry.Get(appID); ok {
		s.logger.Info("Stopping game coordinator session for quit game", log.Uint32("appid", appID))

		_ = driver.OnStopGC(ctx)
	}
}
