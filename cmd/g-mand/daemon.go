// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	"github.com/lemon4ksan/g-man/pkg/behavior"
	"github.com/lemon4ksan/g-man/pkg/behavior/guard"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/auth"
	"github.com/lemon4ksan/g-man/pkg/steam/social/chat"
	"github.com/lemon4ksan/g-man/pkg/steam/socket"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/apps"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/directory"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/gc"
	"github.com/lemon4ksan/g-man/pkg/storage"
	"github.com/lemon4ksan/g-man/pkg/trading/web"
	"github.com/lemon4ksan/miyako/bus"
	"github.com/lemon4ksan/miyako/generic"

	"github.com/lemon4ksan/g-man-cli/pkg/game"
	tf2driver "github.com/lemon4ksan/g-man-cli/pkg/tf2"
	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
)

// Daemon implements the DaemonService gRPC server and runs the bot loop.
type Daemon struct {
	pb.UnimplementedDaemonServiceServer

	cfg          Config
	store        storage.Provider
	logger       log.Logger
	client       *steam.Client
	apps         *apps.Apps
	gc           *gc.Coordinator
	tf2          *tf2.TF2
	schemaMgr    *schema.Manager
	registry     *game.Registry
	sub          *bus.Subscription
	orchestrator *behavior.Orchestrator
	wg           sync.WaitGroup

	mu           sync.RWMutex
	currentAppID uint32
	uptimeStart  time.Time
	shutdownCtx  context.Context
	shutdownFunc context.CancelFunc
}

// NewDaemon creates a new Daemon instance with the given configuration and dependencies.
func NewDaemon(
	cfg Config,
	store storage.Provider,
	logger log.Logger,
	shutdownCtx context.Context,
	shutdownFunc context.CancelFunc,
) (*Daemon, error) {
	clientCfg := steam.DefaultConfig()
	clientCfg.Storage = store

	logger = logger.With(log.Module("daemon"))

	opts := []steam.Option{
		steam.WithLogger(logger),
		apps.WithModule(),
		gc.WithModule(),
		web.WithModule(web.DefaultConfig()),
		schema.WithModule(schema.DefaultConfig()),
		tf2.WithModule(),
		backpack.WithModule(),
		guard.WithModule(guard.DefaultGuardConfig(cfg.SharedSecret, cfg.IdentitySecret, cfg.DeviceID)),
		chat.WithModule(),
	}

	client, err := steam.NewClient(clientCfg, opts...)
	if err != nil {
		return nil, fmt.Errorf("steam client initialization failed: %w", err)
	}

	appsMod := apps.From(client)
	gcMod := gc.From(client)
	tf2Mod := tf2.From(client)
	schemaMod := schema.From(client)

	registry := game.NewRegistry()
	if err := registry.Register(tf2driver.New(client)); err != nil {
		return nil, fmt.Errorf("failed to register TF2 driver: %w", err)
	}

	return &Daemon{
		cfg:          cfg,
		store:        store,
		logger:       logger,
		client:       client,
		apps:         appsMod,
		gc:           gcMod,
		tf2:          tf2Mod,
		schemaMgr:    schemaMod,
		registry:     registry,
		uptimeStart:  time.Now(),
		shutdownCtx:  shutdownCtx,
		shutdownFunc: shutdownFunc,
	}, nil
}

// Run starts the daemon and runs the core client services.
func (s *Daemon) Run(ctx context.Context) error {
	s.logger.Info("Starting core client services...")

	if err := s.client.Run(); err != nil {
		return fmt.Errorf("client run failed: %w", err)
	}

	server, err := s.discoverCMServer(ctx)
	if err != nil {
		return fmt.Errorf("cm discovery failed: %w", err)
	}

	s.logger.Info("Optimal CM server found",
		log.String("endpoint", server.Endpoint),
		log.Float64("load", server.Load),
	)

	s.setupOrchestrator()

	if err := s.orchestrator.Start(ctx); err != nil {
		return fmt.Errorf("orchestrator start failed: %w", err)
	}

	// Subscribe to auth and apps events to stay in sync with auto-play status
	s.sub = s.client.Bus().
		Subscribe(&auth.LoggedOnEvent{}, &auth.LoggedOffEvent{}, &apps.AppLaunchedEvent{}, &apps.AppQuitEvent{})

	s.wg.Go(func() {
		s.handleEvents(ctx)
	})

	s.logger.Info("Connecting and authenticating with Steam...",
		log.String("username", s.cfg.Username),
	)

	details := &auth.LogOnDetails{
		AccountName:  s.cfg.Username,
		Password:     s.cfg.Password,
		RefreshToken: s.cfg.RefreshToken,
	}
	if err := s.client.ConnectAndLogin(ctx, server, details); err != nil {
		return fmt.Errorf("connect and login failed: %w", err)
	}

	s.logger.Info("Bot logged in and fully operational")

	return nil
}

// StartStartupMaintenance launches a background process to maintain inventory
// after the Game Coordinator transitions to the Connected state.
func (s *Daemon) StartStartupMaintenance(ctx context.Context) {
	driver, ok := s.registry.Get(tf2driver.AppID)
	if !ok {
		return
	}

	tf2Driver, ok := driver.(*tf2driver.Driver)
	if !ok {
		return
	}

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		timeout := time.After(30 * time.Second)

		for {
			select {
			case <-ctx.Done():
				return
			case <-timeout:
				s.logger.Error(
					"Startup inventory maintenance cancelled: timed out waiting for TF2 Game Coordinator connection",
				)

				return

			case <-ticker.C:
				if s.tf2 != nil && s.tf2.Connected() {
					if err := tf2Driver.RunMaintenance(ctx, s.logger); err != nil {
						s.logger.Error("Startup inventory maintenance failed", log.Err(err))
					}

					return
				}
			}
		}
	}()
}

// Close stops the daemon and cleans up resources.
func (s *Daemon) Close() {
	s.logger.Info("Stopping active game sessions...")
	s.mu.Lock()
	currentApp := s.currentAppID
	s.currentAppID = 0
	s.mu.Unlock()

	if currentApp != 0 {
		if driver, ok := s.registry.Get(currentApp); ok {
			s.logger.Info("Sending goodbye to game coordinator...", log.Uint32("appid", currentApp))

			_ = driver.OnStopGC(context.Background())
		}

		_ = s.apps.StopPlaying(context.Background())
	}

	if s.orchestrator != nil {
		s.orchestrator.Stop()
		s.logger.Info("Behavior orchestrator stopped")
	}

	if s.sub != nil {
		s.sub.Unsubscribe()
	}

	s.wg.Wait()

	if err := s.client.Close(); err != nil {
		s.logger.Error("Error during client shutdown", log.Err(err))
	} else {
		s.logger.Info("Client session closed")
	}

	s.logger.Info("Bot shut down successfully")
}

func (s *Daemon) discoverCMServer(ctx context.Context) (socket.CMServer, error) {
	dirCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	s.logger.Info("Discovering optimal Steam Connection Manager server...")
	dir := directory.New(s.client.Service())

	return dir.GetOptimalCMServer(dirCtx)
}

func (s *Daemon) setupOrchestrator() {
	s.orchestrator = behavior.NewOrchestrator(s.client.Bus(), s.logger)
	guardModule := guard.From(s.client)

	guardBehaviorCfg := guard.Config{
		AutoAcceptTypes: generic.NewSet(
			guard.ConfTypeTrade,
			guard.ConfTypeMarket,
			guard.ConfTypeLogin,
		),
		PollOnStart: true,
	}

	guard.AutoAccept(s.orchestrator, guardModule, guardBehaviorCfg)
}

// GetStatus returns the daemon state, connection status, memory usage, and active game.
func (s *Daemon) GetStatus(ctx context.Context, req *pb.GetStatusRequest) (*pb.GetStatusResponse, error) {
	connected := s.client.Socket().IsConnected()
	steamID := s.client.SteamID().String()

	s.mu.RLock()
	currentApp := s.currentAppID
	s.mu.RUnlock()

	uptime := time.Since(s.uptimeStart).Truncate(time.Second).String()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	appName := "Unknown Steam Game"
	if currentApp != 0 {
		if _, ok := s.registry.Get(currentApp); ok {
			if currentApp == tf2.AppID {
				appName = "Team Fortress 2"
			}
		}
	}

	return &pb.GetStatusResponse{
		Connected:      connected,
		SteamId:        steamID,
		CurrentAppid:   currentApp,
		CurrentAppName: appName,
		Uptime:         uptime,
		MemoryBytes:    m.Alloc,
		PersonaName:    s.cfg.Username,
	}, nil
}

// FreeMemory triggers manual Garbage Collection and releases system memory back to the OS immediately.
func (s *Daemon) FreeMemory(ctx context.Context, req *pb.FreeMemoryRequest) (*pb.FreeMemoryResponse, error) {
	s.logger.Info("Forcing manual garbage collection and freeing system memory...")

	// Force GC
	runtime.GC()
	// Return memory to OS immediately
	debug.FreeOSMemory()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return &pb.FreeMemoryResponse{
		Message:     "Garbage collection executed and memory released to the OS successfully.",
		MemoryBytes: m.Alloc,
	}, nil
}

// StopDaemon initiates graceful shutdown of the daemon.
func (s *Daemon) StopDaemon(ctx context.Context, req *pb.StopDaemonRequest) (*pb.StopDaemonResponse, error) {
	s.logger.Info("Stop request received from CLI client")

	go func() {
		time.Sleep(200 * time.Millisecond)
		s.shutdownFunc()
	}()

	return &pb.StopDaemonResponse{
		Message: "Daemon shutdown initiated successfully.",
	}, nil
}

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
		&chat.MessageEvent{},
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

			payloadBytes, err := json.Marshal(ev)
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

type manualPriceJSONEntry struct {
	BuyKeys   int     `json:"buy_keys"`
	BuyMetal  float64 `json:"buy_metal"`
	SellKeys  int     `json:"sell_keys"`
	SellMetal float64 `json:"sell_metal"`
}

// UpdateManualPrices updates manual pricing values for items in the daemon.
func (s *Daemon) UpdateManualPrices(
	ctx context.Context,
	req *pb.UpdateManualPricesRequest,
) (*pb.UpdateManualPricesResponse, error) {
	s.logger.Info("Update manual prices request received", log.Int("count", len(req.GetPrices())))

	s.mu.Lock()
	defer s.mu.Unlock()

	prices := make(map[string]manualPriceJSONEntry)

	data, err := os.ReadFile(s.cfg.ManualPricesPath)
	if err == nil {
		_ = json.Unmarshal(data, &prices)
	} else if !os.IsNotExist(err) {
		s.logger.Warn("Failed to read manual prices file", log.Err(err))
	}

	for sku, entry := range req.GetPrices() {
		s.logger.Info("Updating manual price",
			log.String("sku", sku),
			log.Uint32("buy_keys", entry.GetBuyKeys()),
			log.Float64("buy_metal", entry.GetBuyMetal()),
			log.Uint32("sell_keys", entry.GetSellKeys()),
			log.Float64("sell_metal", entry.GetSellMetal()),
		)

		prices[sku] = manualPriceJSONEntry{
			BuyKeys:   int(entry.GetBuyKeys()),
			BuyMetal:  entry.GetBuyMetal(),
			SellKeys:  int(entry.GetSellKeys()),
			SellMetal: entry.GetSellMetal(),
		}
	}

	newData, err := json.MarshalIndent(prices, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manual prices: %w", err)
	}

	dir := filepath.Dir(s.cfg.ManualPricesPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create directory for manual prices: %w", err)
	}

	tmpPath := s.cfg.ManualPricesPath + ".tmp"
	if err := os.WriteFile(tmpPath, newData, 0o600); err != nil {
		return nil, fmt.Errorf("failed to write manual prices: %w", err)
	}

	if err := os.Rename(tmpPath, s.cfg.ManualPricesPath); err != nil {
		return nil, fmt.Errorf("failed to save manual prices: %w", err)
	}

	return &pb.UpdateManualPricesResponse{
		Message: fmt.Sprintf("Successfully processed %d price updates.", len(req.GetPrices())),
		Success: true,
	}, nil
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

// generateMemoryProfile collects runtime MemStats and formats them into a clean ASCII table.
func (s *Daemon) generateMemoryProfile() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	var sb strings.Builder
	sb.WriteString("\n=== G-MAN DAEMON DETAILED MEMORY PROFILE ===\n")

	formatBytes := func(bytes uint64) string {
		return fmt.Sprintf("%.2f MB (%d bytes)", float64(bytes)/1024.0/1024.0, bytes)
	}

	w := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Memory Metric\tValue\n")
	fmt.Fprintf(w, "-------------\t-----\n")
	fmt.Fprintf(w, "Alloc (Live Heap):\t%s\n", formatBytes(m.Alloc))
	fmt.Fprintf(w, "Total Allocated (Cumulative):\t%s\n", formatBytes(m.TotalAlloc))
	fmt.Fprintf(w, "OS Reserved Virtual Memory (Sys):\t%s\n", formatBytes(m.Sys))
	fmt.Fprintf(w, "Heap Idle (Unused spans):\t%s\n", formatBytes(m.HeapIdle))
	fmt.Fprintf(w, "Heap In-Use (Active allocated spans):\t%s\n", formatBytes(m.HeapInuse))
	fmt.Fprintf(w, "Heap Released back to OS:\t%s\n", formatBytes(m.HeapReleased))
	fmt.Fprintf(w, "Stack In-Use (Goroutine stacks):\t%s\n", formatBytes(m.StackInuse))
	fmt.Fprintf(w, "MSpan In-Use:\t%s\n", formatBytes(m.MSpanInuse))
	fmt.Fprintf(w, "MCache In-Use:\t%s\n", formatBytes(m.MCacheInuse))
	fmt.Fprintf(w, "GC Metadata Memory:\t%s\n", formatBytes(m.GCSys))
	fmt.Fprintf(w, "Next GC Heap Target:\t%s\n", formatBytes(m.NextGC))
	fmt.Fprintf(w, "Live Allocated Objects:\t%d\n", m.HeapObjects)
	fmt.Fprintf(w, "Total Mallocs Count:\t%d\n", m.Mallocs)
	fmt.Fprintf(w, "Total Frees Count:\t%d\n", m.Frees)
	fmt.Fprintf(w, "Completed GC Cycles:\t%d\n", m.NumGC)
	fmt.Fprintf(w, "Forced GC Cycles (FreeMemory):\t%d\n", m.NumForcedGC)

	lastGC := "-"
	if m.LastGC > 0 {
		lastGC = time.Unix(0, int64(m.LastGC)).Format("2006-01-02 15:04:05") //#nosec G115
	}

	fmt.Fprintf(w, "Last GC Time:\t%s\n", lastGC)
	fmt.Fprintf(w, "Total GC Pause Time (STW):\t%v\n", time.Duration(m.PauseTotalNs)) //#nosec G115

	_ = w.Flush()

	return sb.String()
}
