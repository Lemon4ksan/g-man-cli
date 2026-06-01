// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"flag"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	"github.com/lemon4ksan/g-man/pkg/behavior"
	"github.com/lemon4ksan/g-man/pkg/behavior/guard"
	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/auth"
	"github.com/lemon4ksan/g-man/pkg/steam/socket"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/apps"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/directory"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/gc"
	"github.com/lemon4ksan/g-man/pkg/storage"
	"github.com/lemon4ksan/g-man/pkg/storage/jsonfile"
	"github.com/lemon4ksan/g-man/pkg/trading/web"
	"google.golang.org/grpc"

	"github.com/lemon4ksan/g-man-cli/pkg/game"
	pb "github.com/lemon4ksan/g-man-cli/pkg/protobuf/daemon"
	tf2driver "github.com/lemon4ksan/g-man-cli/pkg/tf2"
)

// Config holds the configuration loaded from environment variables.
type Config struct {
	Username       string
	Password       string
	SharedSecret   string
	IdentitySecret string
	DeviceID       string
	StoragePath    string
}

// Daemon implements the DaemonService gRPC server and runs the bot loop.
type Daemon struct {
	pb.UnimplementedDaemonServiceServer

	cfg          Config
	store        storage.Provider
	logger       log.Logger
	client       *steam.Client
	apps         *apps.Apps
	gc           *gc.Coordinator
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
	}

	client, err := steam.NewClient(clientCfg, opts...)
	if err != nil {
		return nil, fmt.Errorf("steam client initialization failed: %w", err)
	}

	appsMod := apps.From(client)
	gcMod := gc.From(client)

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
		AccountName: s.cfg.Username,
		Password:    s.cfg.Password,
	}
	if err := s.client.ConnectAndLogin(ctx, server, details); err != nil {
		return fmt.Errorf("connect and login failed: %w", err)
	}

	s.logger.Info("Bot logged in and fully operational")

	return nil
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
	s.orchestrator = behavior.NewOrchestrator(s.logger, s.client.Bus())
	guardModule := guard.From(s.client)

	guardBehaviorCfg := guard.Config{
		AutoAcceptTypes: []guard.ConfirmationType{
			guard.ConfTypeTrade,
			guard.ConfTypeMarket,
			guard.ConfTypeLogin,
		},
		PollOnStart: true,
	}

	s.orchestrator.Install(guard.AutoAccept(guardModule, guardBehaviorCfg))
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
				s.mu.Lock()
				s.currentAppID = ev.AppID
				s.mu.Unlock()
				s.logger.Info("Detected game launched", log.Uint32("appid", ev.AppID))
				// Initialize GC session if a driver exists
				if driver, ok := s.registry.Get(ev.AppID); ok {
					s.logger.Info(
						"Initializing game coordinator session for auto-launched game",
						log.Uint32("appid", ev.AppID),
					)

					if err := driver.OnStartGC(ctx); err != nil {
						s.logger.Error("GC startup failed on driver", log.Uint32("appid", ev.AppID), log.Err(err))
					}
				}

			case *apps.AppQuitEvent:
				s.mu.Lock()
				if s.currentAppID == ev.AppID {
					s.currentAppID = 0
				}

				s.mu.Unlock()
				s.logger.Info("Detected game quit", log.Uint32("appid", ev.AppID))
				// Terminate GC session if a driver exists
				if driver, ok := s.registry.Get(ev.AppID); ok {
					s.logger.Info("Stopping game coordinator session for quit game", log.Uint32("appid", ev.AppID))

					_ = driver.OnStopGC(ctx)
				}
			}
		}
	}
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
			// Resolve name from driver
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

	driver, ok := s.registry.Get(req.GetAppid())
	if !ok {
		return nil, fmt.Errorf("no game driver registered for appid %d", req.GetAppid())
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

	// Delegate execution entirely to the game-specific driver.
	// It returns a formatted response or inventory table as a direct text block.
	details, err := driver.InventoryProvider().ExecuteAction(ctx, req.GetAction(), req.GetParams())
	if err != nil {
		return nil, fmt.Errorf("action execution failed: %w", err)
	}

	return &pb.ExecActionResponse{
		Message: "Operation completed successfully.",
		Details: details,
	}, nil
}

func defaultSockPath() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dir := filepath.Join(home, ".config", "gman")
		_ = os.MkdirAll(dir, 0o755)
		return filepath.Join(dir, "gman.sock")
	}

	return "gman.sock"
}

func GetIPCListener() (net.Listener, string, error) {
	netType := os.Getenv("GMAN_IPC_NET")
	addr := os.Getenv("GMAN_IPC_ADDR")

	if netType == "" {
		if runtime.GOOS == "windows" {
			netType = "tcp"
		} else {
			netType = "unix"
		}
	}

	if addr == "" {
		if netType == "tcp" {
			addr = "127.0.0.1:50051"
		} else {
			addr = defaultSockPath()
		}
	}

	if netType == "unix" {
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			return nil, "", fmt.Errorf("failed to remove stale socket: %w", err)
		}

		if err := os.MkdirAll(filepath.Dir(addr), 0o755); err != nil {
			return nil, "", fmt.Errorf("failed to create socket directory: %w", err)
		}
	}

	listener, err := net.Listen(netType, addr)
	if err != nil {
		return nil, "", err
	}

	if netType == "unix" {
		_ = os.Chmod(addr, 0o660)
	}

	return listener, addr, nil
}

func loadEnvConfig() (Config, error) {
	username, password := os.Getenv("STEAM_USER"), os.Getenv("STEAM_PASS")
	if username == "" || password == "" {
		return Config{}, errors.New("STEAM_USER and STEAM_PASS environment variables are required")
	}

	storagePath := os.Getenv("STEAM_STORAGE_PATH")
	if storagePath == "" {
		storagePath = "storage.json"
	}

	return Config{
		Username:       username,
		Password:       password,
		SharedSecret:   os.Getenv("STEAM_SHARED_SECRET"),
		IdentitySecret: os.Getenv("STEAM_IDENTITY_SECRET"),
		DeviceID:       os.Getenv("STEAM_DEVICE_ID"),
		StoragePath:    storagePath,
	}, nil
}

func main() {
	var debugEnabled bool
	flag.BoolVar(&debugEnabled, "debug", false, "Enable debug logging")
	flag.Parse()

	cfg, err := loadEnvConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Config Error:", err)
		os.Exit(1)
	}

	store, err := jsonfile.New(cfg.StoragePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize storage: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	logLevel := log.LevelInfo
	if debugEnabled {
		logLevel = log.LevelDebug
	}

	logCfg := log.DefaultConfig(logLevel)
	logCfg.FullPath = true

	logger := log.New(logCfg)
	defer logger.Close()

	shutdownCtx, shutdownFunc := context.WithCancel(context.Background())
	defer shutdownFunc()

	daemon, err := NewDaemon(cfg, store, logger, shutdownCtx, shutdownFunc)
	if err != nil {
		logger.Error("Failed to initialize daemon", log.Err(err))
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterDaemonServiceServer(grpcServer, daemon)

	listener, path, err := GetIPCListener()
	if err != nil {
		logger.Error("Failed to listen on IPC interface", log.Err(err))
		os.Exit(1)
	}

	logger.Info("gRPC IPC server listening",
		log.String("network", listener.Addr().Network()),
		log.String("address", path),
	)

	go func() {
		if err := grpcServer.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			logger.Error("gRPC server error", log.Err(err))
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("System signal received, stopping daemon...", log.String("signal", sig.String()))
		shutdownFunc()
	}()

	if err := daemon.Run(shutdownCtx); err != nil {
		logger.Error("Daemon run failed", log.Err(err))
	} else {
		// Run initial non-interactive maintenance routine at startup
		if driver, ok := daemon.registry.Get(440); ok {
			if tf2Driver, ok := driver.(*tf2driver.Driver); ok {
				go func() {
					// Add small startup delay to ensure GC is fully ready
					time.Sleep(3 * time.Second)

					if err := tf2Driver.RunMaintenance(shutdownCtx, logger); err != nil {
						logger.Error("Startup inventory maintenance failed", log.Err(err))
					}
				}()
			}
		}

		<-shutdownCtx.Done()
	}

	logger.Info("Initiating graceful shutdown...")
	grpcServer.GracefulStop()
	daemon.Close()

	if listener.Addr().Network() == "unix" {
		_ = os.Remove(path)
		logger.Info("Removed Unix socket file", log.String("path", path))
	}

	logger.Info("Daemon process exited.")
}
