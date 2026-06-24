// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
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
func (d *Daemon) StreamEvents(req *pb.StreamEventsRequest, stream pb.DaemonService_StreamEventsServer) error {
	sub := d.client.Bus().Subscribe(
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

	d.logger.Info("Client connected to daemon event stream")

	for {
		select {
		case <-stream.Context().Done():
			d.logger.Info("Client disconnected from event stream")
			return stream.Context().Err()
		case <-d.shutdownCtx.Done():
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
				d.logger.Error("Failed to marshal event for stream", log.Err(err))
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
				d.logger.Error("Failed to send event to stream", log.Err(err))
				return err
			}
		}
	}
}

func (d *Daemon) handleEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-d.sub.C():
			if !ok {
				return
			}

			switch ev := event.(type) {
			case *auth.LoggedOnEvent:
				d.logger.Info("Login successful", log.Uint64("steam_id", ev.SteamID))
				d.mu.RLock()
				desiredAppID := d.desiredAppID
				d.mu.RUnlock()

				if desiredAppID != 0 {
					d.logger.Info("Resuming game play after reconnection", log.Uint32("appid", desiredAppID))

					go func(aid uint32) {
						playCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
						defer cancel()

						d.mu.Lock()
						d.currentAppID = aid
						d.mu.Unlock()

						if err := d.apps.PlayGames(playCtx, []uint32{aid}, true); err != nil {
							d.logger.Error(
								"Failed to resume playing game after reconnection",
								log.Uint32("appid", aid),
								log.Err(err),
							)
							d.mu.Lock()
							d.currentAppID = 0
							d.mu.Unlock()

							return
						}

						if driver, ok := d.registry.Get(aid); ok {
							d.logger.Info(
								"Resuming game coordinator session after reconnection",
								log.Uint32("appid", aid),
							)

							if err := driver.OnStartGC(playCtx); err != nil {
								d.logger.Error(
									"GC startup failed on driver after reconnection",
									log.Uint32("appid", aid),
									log.Err(err),
								)
							}
						}

						if aid == tf2.AppID {
							d.StartStartupMaintenance(d.shutdownCtx)
						}
					}(desiredAppID)
				}

			case *auth.LoggedOffEvent:
				d.logger.Warn("Logged off from Steam", log.Uint32("result", uint32(ev.Result))) // #nosec G115
				d.mu.RLock()
				activeApp := d.currentAppID
				d.mu.RUnlock()

				if activeApp != 0 {
					if driver, ok := d.registry.Get(activeApp); ok {
						d.logger.Info("Stopping GC session due to disconnect", log.Uint32("appid", activeApp))

						_ = driver.OnStopGC(ctx)
					}

					_ = d.apps.StopPlaying(ctx)
				}

			case *apps.AppLaunchedEvent:
				d.onAppLaunched(ctx, ev.AppID)
			case *apps.AppQuitEvent:
				d.onAppQuit(ctx, ev.AppID)
			case *auth.SteamGuardRequiredEvent:
				d.activeAuthCallbackMu.Lock()
				d.activeAuthCallback = ev.Callback
				d.activeAuthCallbackMu.Unlock()
				d.logger.Info(
					"Steam Guard code required",
					log.Bool("is_2fa", ev.Is2FA),
					log.String("domain", ev.EmailDomain),
				)

			case *web.NewOfferEvent:
				partnerID := strconv.FormatUint(ev.Offer.OtherSteamID.Uint64(), 10)

				if slices.Contains(d.cfg.ExcludedIDs, partnerID) {
					d.logger.Info(
						"Auto-declining trade offer from excluded SteamID",
						log.Uint64("offer_id", ev.Offer.ID),
						log.String("partner_steam_id", partnerID),
					)

					webMod := web.From(d.client)
					if webMod != nil {
						if err := webMod.DeclineOffer(ctx, ev.Offer.ID); err != nil {
							d.logger.Error(
								"Failed to auto-decline trade offer from excluded SteamID",
								log.Uint64("offer_id", ev.Offer.ID),
								log.Err(err),
							)
						}
					}

					continue
				}

				if slices.Contains(d.cfg.TrustedIDs, partnerID) {
					d.logger.Info(
						"Auto-accepting trade offer from trusted SteamID",
						log.Uint64("offer_id", ev.Offer.ID),
						log.String("partner_steam_id", partnerID),
					)

					webMod := web.From(d.client)
					if webMod == nil {
						d.logger.Error(
							"Web module not registered, cannot auto-accept offer",
							log.Uint64("offer_id", ev.Offer.ID),
						)

						continue
					}

					if err := webMod.AcceptOffer(ctx, ev.Offer.ID); err != nil {
						d.logger.Error(
							"Failed to auto-accept trade offer from trusted SteamID",
							log.Uint64("offer_id", ev.Offer.ID),
							log.Err(err),
						)
					}
				}
			}
		}
	}
}

func (d *Daemon) onAppLaunched(ctx context.Context, appID uint32) {
	d.mu.Lock()
	d.currentAppID = appID
	d.mu.Unlock()

	d.logger.Info("Detected game launched", log.Uint32("appid", appID))

	if driver, ok := d.registry.Get(appID); ok {
		d.logger.Info("Initializing game coordinator session for auto-launched game", log.Uint32("appid", appID))

		if err := driver.OnStartGC(ctx); err != nil {
			d.logger.Error("GC startup failed on driver", log.Uint32("appid", appID), log.Err(err))
		}
	}
}

func (d *Daemon) onAppQuit(ctx context.Context, appID uint32) {
	d.mu.Lock()
	if d.currentAppID == appID {
		d.currentAppID = 0
	}

	d.mu.Unlock()

	d.logger.Info("Detected game quit", log.Uint32("appid", appID))

	if driver, ok := d.registry.Get(appID); ok {
		d.logger.Info("Stopping game coordinator session for quit game", log.Uint32("appid", appID))

		_ = driver.OnStopGC(ctx)
	}
}

// watchConnection periodically monitors the Steam connection health.
// The library (g-man) handles reconnection automatically; this goroutine only
// performs GC/game cleanup when a persistent drop is detected so that
// LoggedOnEvent can resume everything cleanly after the library reconnects.
func (d *Daemon) watchConnection(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.mu.RLock()
			locked := d.isLocked
			d.mu.RUnlock()

			if locked {
				continue
			}

			if d.isConnectionHealthy() {
				continue
			}

			// Connection looks unhealthy — wait to rule out a transient network flap.
			// The library is already reconnecting; we only need to clean up GC state.
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}

			if d.isConnectionHealthy() {
				continue
			}

			d.logger.Warn("Watchdog: Steam connection is unhealthy, cleaning up GC state")

			d.mu.RLock()
			activeApp := d.currentAppID
			d.mu.RUnlock()

			if activeApp != 0 {
				if driver, ok := d.registry.Get(activeApp); ok {
					d.logger.Info(
						"Stopping GC session due to watchdog detected drop",
						log.Uint32("appid", activeApp),
					)

					_ = driver.OnStopGC(ctx)
				}

				_ = d.apps.StopPlaying(ctx)

				d.mu.Lock()
				d.currentAppID = 0
				d.mu.Unlock()
			}
		}
	}
}

// isConnectionHealthy reports whether the Steam socket is connected and the session is authenticated.
func (d *Daemon) isConnectionHealthy() bool {
	sock := d.client.Socket()
	if sock == nil {
		return false
	}

	if !sock.IsConnected() {
		return false
	}

	session := sock.Session()
	if session == nil {
		return false
	}

	return session.IsAuthenticated()
}
