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
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/apps"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/pkg/trading/web"

	tf2driver "github.com/lemon4ksan/g-man-cli/pkg/tf2/driver"
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

			if newOfferEv, ok := ev.(*web.NewOfferEvent); ok && newOfferEv.Offer != nil {
				if newOfferEv.Offer.OtherSteamID != 0 && newOfferEv.Offer.OtherSteamID < id.FromAccountID(0) {
					newOfferEv.Offer.OtherSteamID = id.FromAccountID(
						uint32(newOfferEv.Offer.OtherSteamID), //nolint:gosec
					)
				}
			} else if offerChangedEv, ok := ev.(*web.OfferChangedEvent); ok && offerChangedEv.Offer != nil {
				if offerChangedEv.Offer.OtherSteamID != 0 && offerChangedEv.Offer.OtherSteamID < id.FromAccountID(0) {
					offerChangedEv.Offer.OtherSteamID = id.FromAccountID(
						uint32(offerChangedEv.Offer.OtherSteamID), //nolint:gosec
					)
				}
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
				d.handleTrustedOrExcludedOffer(ctx, ev.Offer)

			case *web.OfferChangedEvent:
				if ev.Offer.State == trading.OfferStateActive {
					d.handleTrustedOrExcludedOffer(ctx, ev.Offer)
				}

			case *web.PollSuccessEvent:
				d.checkAndProcessActiveOffers(ctx)

			case *tf2.BackpackLoadedEvent:
				d.logger.Info("Backpack loaded, acknowledging items", log.Int("items", ev.Count))

				// Acknowledge items immediately - OnStartGC's goroutine only acks
				// on FUTURE events, not the current one that triggered it.
				if tf2Mod := tf2.From(d.client); tf2Mod != nil {
					if err := tf2Mod.AcknowledgeAll(ctx); err != nil {
						d.logger.Error("Failed to acknowledge items after backpack load", log.Err(err))
					}
				}

				// Start auto-ack goroutine for future item acquisitions
				if driver, ok := d.registry.Get(tf2.AppID); ok {
					if err := driver.OnStartGC(ctx); err != nil {
						d.logger.Error("Failed to start auto-ack after backpack load", log.Err(err))
					}

					// Run maintenance (smelt, condense, sort) now that items are loaded
					go func() {
						if err := driver.(*tf2driver.Driver).RunMaintenance(ctx, d.logger); err != nil {
							d.logger.Error("Post-load maintenance failed", log.Err(err))
						}
					}()
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

func (d *Daemon) handleTrustedOrExcludedOffer(ctx context.Context, offer *trading.TradeOffer) {
	if offer != nil {
		if offer.OtherSteamID != 0 && offer.OtherSteamID < id.FromAccountID(0) {
			offer.OtherSteamID = id.FromAccountID(uint32(offer.OtherSteamID)) //nolint:gosec
		}
	}

	partnerID := strconv.FormatUint(offer.OtherSteamID.Uint64(), 10)

	if slices.Contains(d.cfg.ExcludedIDs, partnerID) {
		d.logger.Info(
			"Auto-declining trade offer from excluded SteamID",
			log.Uint64("offer_id", offer.ID),
			log.String("partner_steam_id", partnerID),
		)

		webMod := web.From(d.client)
		if webMod != nil {
			if err := webMod.DeclineOffer(ctx, offer.ID); err != nil {
				d.logger.Error(
					"Failed to auto-decline trade offer from excluded SteamID",
					log.Uint64("offer_id", offer.ID),
					log.Err(err),
				)
			}
		}

		return
	}

	if slices.Contains(d.cfg.TrustedIDs, partnerID) {
		d.logger.Info(
			"Auto-accepting trade offer from trusted SteamID",
			log.Uint64("offer_id", offer.ID),
			log.String("partner_steam_id", partnerID),
		)

		webMod := web.From(d.client)
		if webMod == nil {
			d.logger.Error(
				"Web module not registered, cannot auto-accept offer",
				log.Uint64("offer_id", offer.ID),
			)

			return
		}

		if err := webMod.AcceptOffer(ctx, offer.ID); err != nil {
			d.logger.Error(
				"Failed to auto-accept trade offer from trusted SteamID",
				log.Uint64("offer_id", offer.ID),
				log.Err(err),
			)
		}
	}
}

func (d *Daemon) checkAndProcessActiveOffers(ctx context.Context) {
	webMod := web.From(d.client)
	if webMod == nil {
		return
	}

	pollData := webMod.GetPollData()
	for offerID, state := range pollData.Received {
		if state == trading.OfferStateActive {
			offer, err := webMod.GetOffer(ctx, offerID)
			if err == nil && offer != nil {
				d.handleTrustedOrExcludedOffer(ctx, offer)
			}
		}
	}
}
