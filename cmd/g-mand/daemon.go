// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
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
	"github.com/lemon4ksan/g-man/pkg/behavior/achievements"
	"github.com/lemon4ksan/g-man/pkg/behavior/guard"
	corecrypto "github.com/lemon4ksan/g-man/pkg/crypto"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/auth"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol/enums"
	"github.com/lemon4ksan/g-man/pkg/steam/social/chat"
	"github.com/lemon4ksan/g-man/pkg/steam/social/chat/commands"
	"github.com/lemon4ksan/g-man/pkg/steam/social/friends"
	"github.com/lemon4ksan/g-man/pkg/steam/socket"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/account"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/apps"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/directory"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/gc"
	"github.com/lemon4ksan/g-man/pkg/storage"
	"github.com/lemon4ksan/g-man/pkg/trading/web"
	"github.com/lemon4ksan/miyako/bus"
	"github.com/lemon4ksan/miyako/generic"

	"github.com/lemon4ksan/g-man-cli/pkg/game"
	gman_crypto "github.com/lemon4ksan/g-man-cli/pkg/guard/crypto"
	tf2driver "github.com/lemon4ksan/g-man-cli/pkg/tf2/driver"
	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
)

type unlockRequest struct {
	passphrase string
	resChan    chan error
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
	tf2          *tf2.TF2
	schemaMgr    *schema.Manager
	registry     *game.Registry
	sub          *bus.Subscription
	orchestrator *behavior.Orchestrator
	wg           sync.WaitGroup

	mu           sync.RWMutex
	currentAppID uint32
	desiredAppID uint32
	uptimeStart  time.Time
	shutdownCtx  context.Context
	shutdownFunc context.CancelFunc

	activeAuthCallbackMu sync.Mutex
	activeAuthCallback   func(string)

	isLocked   bool
	unlockChan chan unlockRequest
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
	clientCfg.PersonaState = enums.EPersonaState(cfg.PersonaState)

	if err := setupProxyConfig(cfg, &clientCfg, logger); err != nil {
		return nil, err
	}

	var logFields []log.Field
	if cfg.Username != "" {
		logFields = append(logFields, log.String("account", cfg.Username))
	}

	if cfg.RefreshToken != "" {
		if id := auth.ExtractSteamIDFromJWT(cfg.RefreshToken); id != 0 {
			logFields = append(logFields, log.SteamID(id.Uint64()))
		}
	}

	if len(logFields) > 0 {
		logger = logger.With(logFields...)
	}

	logger = logger.With(log.Module("daemon"))

	opts := []steam.Option{
		steam.WithLogger(logger),
		chat.WithModule(),
		friends.WithModule(),
		account.WithModule(),
		apps.WithModule(),
		gc.WithModule(),
		web.WithModule(web.DefaultConfig()),
		schema.WithModule(schema.DefaultConfig()),
		tf2.WithModule(),
		backpack.WithModule(),
		guard.WithModule(guard.DefaultGuardConfig(cfg.SharedSecret, cfg.IdentitySecret, cfg.DeviceID)),
		commands.WithModule(),
	}

	client, err := steam.NewClient(clientCfg, opts...)
	if err != nil {
		return nil, fmt.Errorf("steam client initialization failed: %w", err)
	}

	appsMod := apps.From(client)
	gcMod := gc.From(client)

	tf2Mod := tf2.From(client)
	if tf2Mod != nil {
		tf2Mod.SetKeepActive(true)
	}

	schemaMod := schema.From(client)

	registry := game.NewRegistry()
	if err := registry.Register(tf2driver.New(client)); err != nil {
		return nil, fmt.Errorf("failed to register TF2 driver: %w", err)
	}

	isLocked := gman_crypto.IsEncryptedString(cfg.Password) ||
		gman_crypto.IsEncryptedString(cfg.RefreshToken) ||
		gman_crypto.IsEncryptedString(cfg.SharedSecret) ||
		gman_crypto.IsEncryptedString(cfg.IdentitySecret) ||
		cfg.MaFileEncrypted

	d := &Daemon{
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
		isLocked:     isLocked,
		unlockChan:   make(chan unlockRequest),
	}

	return d, nil
}

// Run starts the daemon and runs the core client services.
func (d *Daemon) Run(ctx context.Context) error {
	if d.isLocked {
		d.logger.Info("Daemon configuration is ENCRYPTED. Waiting for gmanctl guard unlock...")

		for d.isLocked {
			var req unlockRequest
			select {
			case <-ctx.Done():
				return ctx.Err()
			case r := <-d.unlockChan:
				req = r
			}

			d.logger.Info("Decrypting configuration variables...")

			if d.cfg.MaFilePath != "" && d.cfg.MaFileEncrypted {
				d.logger.Info("Decrypting maFile...", log.String("path", d.cfg.MaFilePath))

				fileData, err := os.ReadFile(d.cfg.MaFilePath)
				if err != nil {
					req.resChan <- fmt.Errorf("failed to read maFile: %w", err)
					continue
				}

				decrypted, err := gman_crypto.DecryptData(fileData, req.passphrase)
				if err != nil {
					req.resChan <- fmt.Errorf("failed to decrypt maFile: %w", err)
					continue
				}

				type maFile struct {
					SharedSecret   string `json:"shared_secret"`
					IdentitySecret string `json:"identity_secret"`
					DeviceID       string `json:"device_id"`
					AccountName    string `json:"account_name"`
					SteamID        string `json:"steam_id"`
					Tokens         struct {
						RefreshToken string `json:"refresh_token"`
					} `json:"tokens"`
					Session struct {
						SteamID string `json:"SteamID"`
					} `json:"Session"`
				}

				var ma maFile
				if err := json.Unmarshal(decrypted, &ma); err != nil {
					req.resChan <- fmt.Errorf("failed to parse decrypted maFile JSON: %w", err)
					continue
				}

				if ma.SharedSecret == "" || ma.IdentitySecret == "" {
					req.resChan <- errors.New("invalid maFile: missing shared_secret or identity_secret")
					continue
				}

				steamID := ma.SteamID
				if steamID == "" {
					steamID = ma.Session.SteamID
				}

				var devID string
				if ma.DeviceID != "" {
					devID = ma.DeviceID
				} else if steamID != "" {
					if id, err := strconv.ParseUint(steamID, 10, 64); err == nil && id > 0 {
						devID = corecrypto.GetDeviceID(id)
					}
				}

				if devID == "" {
					var r [16]byte

					_, _ = rand.Read(r[:])
					sum := hex.EncodeToString(r[:])
					devID = fmt.Sprintf("android:%s-%s-%s-%s-%s",
						sum[:8], sum[8:12], sum[12:16], sum[16:20], sum[20:32],
					)
				}

				d.mu.Lock()
				d.cfg.SharedSecret = ma.SharedSecret
				d.cfg.IdentitySecret = ma.IdentitySecret
				d.cfg.DeviceID = devID

				if ma.AccountName != "" && d.cfg.Username == "" {
					d.cfg.Username = ma.AccountName
				}

				if ma.Tokens.RefreshToken != "" && d.cfg.RefreshToken == "" {
					d.cfg.RefreshToken = ma.Tokens.RefreshToken
				}

				d.isLocked = false
				d.mu.Unlock()

				if err := d.configureGuardian(
					ma.SharedSecret,
					ma.IdentitySecret,
					devID,
					d.cfg.Username,
					d.cfg.RefreshToken,
				); err != nil {
					req.resChan <- err

					d.mu.Lock()
					d.isLocked = true
					d.mu.Unlock()

					continue
				}

				d.logger.Info("Configuration successfully loaded from encrypted maFile and guardian configured!")

				req.resChan <- nil

				continue
			}

			decrypt := func(val string) (string, error) {
				if gman_crypto.IsEncryptedString(val) {
					return gman_crypto.DecryptString(val, req.passphrase)
				}

				return val, nil
			}

			decryptedPass, err := decrypt(d.cfg.Password)
			if err != nil {
				req.resChan <- fmt.Errorf("failed to decrypt password: %w", err)
				continue
			}

			decryptedRefresh, err := decrypt(d.cfg.RefreshToken)
			if err != nil {
				req.resChan <- fmt.Errorf("failed to decrypt refresh token: %w", err)
				continue
			}

			decryptedShared, err := decrypt(d.cfg.SharedSecret)
			if err != nil {
				req.resChan <- fmt.Errorf("failed to decrypt shared secret: %w", err)
				continue
			}

			decryptedIdentity, err := decrypt(d.cfg.IdentitySecret)
			if err != nil {
				req.resChan <- fmt.Errorf("failed to decrypt identity secret: %w", err)
				continue
			}

			d.mu.Lock()
			d.cfg.Password = decryptedPass
			d.cfg.RefreshToken = decryptedRefresh
			d.cfg.SharedSecret = decryptedShared
			d.cfg.IdentitySecret = decryptedIdentity
			d.isLocked = false
			d.mu.Unlock()

			if err := d.configureGuardian(
				decryptedShared,
				decryptedIdentity,
				d.cfg.DeviceID,
				d.cfg.Username,
				decryptedRefresh,
			); err != nil {
				req.resChan <- err

				d.mu.Lock()
				d.isLocked = true
				d.mu.Unlock()

				continue
			}

			d.logger.Info("Configuration successfully decrypted and guardian configured!")

			req.resChan <- nil // signal success to the gRPC client
		}
	}

	d.logger.Info("Starting core client services...")

	if err := d.client.Run(); err != nil {
		return fmt.Errorf("client run failed: %w", err)
	}

	server, err := d.discoverCMServer(ctx)
	if err != nil {
		return fmt.Errorf("cm discovery failed: %w", err)
	}

	go func() {
		dir := directory.New(d.client.Service())

		servers, err := dir.GetCMListForConnect(ctx, directory.CMCfg{})
		if err == nil && len(servers) > 0 {
			socketServers := make([]socket.CMServer, len(servers))
			for i, srv := range servers {
				socketServers[i] = socket.CMServer{
					Endpoint: srv.Endpoint,
					Type:     srv.Type,
					Load:     float64(srv.Load),
					Realm:    srv.Realm,
				}
			}

			d.client.Socket().UpdateServers(socketServers)
			d.logger.Info(
				"CM server pool successfully loaded for dynamic rotation",
				log.Int("count", len(socketServers)),
			)
		}
	}()

	d.logger.Info("Optimal CM server found",
		log.String("endpoint", server.Endpoint),
		log.Float64("load", server.Load),
	)

	d.setupOrchestrator()

	if err := d.orchestrator.Start(ctx); err != nil {
		return fmt.Errorf("orchestrator start failed: %w", err)
	}

	// Subscribe to auth and apps events to stay in sync with auto-play status
	d.sub = d.client.Bus().Subscribe(
		&auth.LoggedOnEvent{},
		&auth.LoggedOffEvent{},
		&apps.AppLaunchedEvent{},
		&apps.AppQuitEvent{},
		&auth.SteamGuardRequiredEvent{},
		&web.NewOfferEvent{},
	)

	d.wg.Go(func() {
		d.handleEvents(ctx)
	})

	d.wg.Go(func() {
		d.watchConnection(ctx)
	})

	username := d.cfg.Username
	if username == "" && d.cfg.RefreshToken != "" {
		if id := auth.ExtractSteamIDFromJWT(d.cfg.RefreshToken); id != 0 {
			username = strconv.FormatUint(id.Uint64(), 10)
		}
	}

	d.logger.Info("Connecting and authenticating with Steam...",
		log.String("username", username),
	)

	details := &auth.LogOnDetails{
		AccountName:  username,
		Password:     d.cfg.Password,
		RefreshToken: d.cfg.RefreshToken,
	}
	if err := d.client.ConnectAndLogin(ctx, server, details); err != nil {
		return fmt.Errorf("connect and login failed: %w", err)
	}

	if steamID := d.client.SteamID(); steamID != 0 {
		d.logger = d.logger.With(log.SteamID(steamID.Uint64()))
	}

	d.logger.Info("Bot logged in and fully operational")

	return nil
}

// StartStartupMaintenance launches a background process to maintain inventory
// after the Game Coordinator transitions to the Connected state.
func (d *Daemon) StartStartupMaintenance(ctx context.Context) {
	driver, ok := d.registry.Get(tf2driver.AppID)
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

		timeout := time.After(60 * time.Second)

		for {
			select {
			case <-ctx.Done():
				return
			case <-timeout:
				d.logger.Error(
					"Startup inventory maintenance cancelled: timed out waiting for TF2 Game Coordinator connection",
				)

				return

			case <-ticker.C:
				if d.tf2 != nil && d.tf2.Connected() {
					if err := tf2Driver.RunMaintenance(ctx, d.logger); err != nil {
						d.logger.Error("Startup inventory maintenance failed", log.Err(err))
					}

					return
				}
			}
		}
	}()
}

// Close stops the daemon and cleans up resources.
func (d *Daemon) Close() {
	d.logger.Info("Stopping active game sessions...")
	d.mu.Lock()
	currentApp := d.currentAppID
	d.currentAppID = 0
	d.desiredAppID = 0
	d.mu.Unlock()

	if currentApp != 0 {
		if driver, ok := d.registry.Get(currentApp); ok {
			d.logger.Info("Sending goodbye to game coordinator...", log.Uint32("appid", currentApp))

			_ = driver.OnStopGC(context.Background())
		}

		_ = d.apps.StopPlaying(context.Background())
	}

	if d.orchestrator != nil {
		d.orchestrator.Stop()
		d.logger.Info("Behavior orchestrator stopped")
	}

	if d.sub != nil {
		d.sub.Unsubscribe()
	}

	d.wg.Wait()

	if err := d.client.Close(); err != nil {
		d.logger.Error("Error during client shutdown", log.Err(err))
	} else {
		d.logger.Info("Client session closed")
	}

	d.logger.Info("Bot shut down successfully")
}

func (d *Daemon) setupOrchestrator() {
	d.orchestrator = behavior.NewOrchestrator(d.client.Bus(), d.logger)
	provider := &dynamicGuardProvider{client: d.client}

	guardBehaviorCfg := guard.Config{
		AutoAcceptTypes: generic.NewSet(
			guard.ConfTypeTrade,
			guard.ConfTypeMarket,
			guard.ConfTypeLogin,
		),
		PollOnStart: true,
	}

	guard.AutoAccept(d.orchestrator, provider, guardBehaviorCfg)

	if d.cfg.EnableAchievements {
		d.logger.Info("Installing human-like achievements simulation behavior")
		achievements.Simulate(d.orchestrator, d.tf2, tf2.AchievementConfig())
	}
}

func (d *Daemon) discoverCMServer(ctx context.Context) (socket.CMServer, error) {
	dirCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	d.logger.Info("Discovering optimal Steam Connection Manager server...")
	dir := directory.New(d.client.Service())

	return dir.GetOptimalCMServer(dirCtx)
}

// GetStatus returns the daemon state, connection status, memory usage, and active game.
func (d *Daemon) GetStatus(ctx context.Context, req *pb.GetStatusRequest) (*pb.GetStatusResponse, error) {
	connected := d.client.Socket().IsConnected()
	steamID := d.client.SteamID().String()

	d.mu.RLock()
	currentApp := d.currentAppID
	locked := d.isLocked
	d.mu.RUnlock()

	uptime := time.Since(d.uptimeStart).Truncate(time.Second).String()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	appName := "Unknown Steam Game"
	if currentApp != 0 {
		if _, ok := d.registry.Get(currentApp); ok {
			// Resolve name from driver
			if currentApp == tf2.AppID {
				appName = "Team Fortress 2"
			}
		}
	}

	acc := account.From(d.client)

	var (
		personaName       string
		ipCountry         string
		walletBalance     int64
		walletCurrency    string
		isLimited         bool
		isCommunityBanned bool
		vacBansCount      uint32
		emailAddress      string
	)
	if acc != nil {
		info := acc.GetAccountInfo()
		personaName = info.PersonaName
		ipCountry = info.IPCountry

		wallet := acc.GetWalletInfo()
		walletBalance = wallet.Balance

		curCode := enums.ECurrencyCode(wallet.Currency)
		if curName, exists := enums.ECurrencyCode_name[curCode]; exists {
			walletCurrency = curName
		} else {
			walletCurrency = curCode.String()
		}

		limitations := acc.GetLimitations()
		isLimited = limitations.IsLimitedAccount
		isCommunityBanned = limitations.IsCommunityBanned

		vac := acc.GetVACBans()
		vacBansCount = vac.NumBans

		email := acc.GetEmailInfo()
		emailAddress = email.EmailAddress
	}

	return &pb.GetStatusResponse{
		Connected:         connected,
		SteamId:           steamID,
		CurrentAppid:      currentApp,
		CurrentAppName:    appName,
		Uptime:            uptime,
		MemoryBytes:       m.Alloc,
		Locked:            locked,
		PersonaName:       personaName,
		IpCountry:         ipCountry,
		WalletBalance:     walletBalance,
		WalletCurrency:    walletCurrency,
		IsLimited:         isLimited,
		IsCommunityBanned: isCommunityBanned,
		VacBansCount:      vacBansCount,
		EmailAddress:      emailAddress,
		TrustedIds:        d.cfg.TrustedIDs,
		ExcludedIds:       d.cfg.ExcludedIDs,
	}, nil
}

// FreeMemory triggers manual Garbage Collection and releases system memory back to the OS immediately.
func (d *Daemon) FreeMemory(ctx context.Context, req *pb.FreeMemoryRequest) (*pb.FreeMemoryResponse, error) {
	d.logger.Info("Forcing manual garbage collection and freeing system memory...")

	runtime.GC()
	debug.FreeOSMemory()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return &pb.FreeMemoryResponse{
		Message:     "Garbage collection executed and memory released to the OS successfully.",
		MemoryBytes: m.Alloc,
	}, nil
}

// StopDaemon initiates graceful shutdown of the daemon.
func (d *Daemon) StopDaemon(ctx context.Context, req *pb.StopDaemonRequest) (*pb.StopDaemonResponse, error) {
	d.logger.Info("Stop request received from CLI client")

	go func() {
		time.Sleep(200 * time.Millisecond)
		d.shutdownFunc()
	}()

	return &pb.StopDaemonResponse{
		Message: "Daemon shutdown initiated successfully.",
	}, nil
}

// generateMemoryProfile collects runtime MemStats and formats them into a clean ASCII table.
func (d *Daemon) generateMemoryProfile() string {
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

// SetFriendNickname sets a custom nickname for a specific friend.
func (d *Daemon) SetFriendNickname(
	ctx context.Context,
	req *pb.SetFriendNicknameRequest,
) (*pb.SetFriendNicknameResponse, error) {
	d.logger.Info("Set friend nickname request",
		log.Uint64("steam_id", req.GetSteamId()),
		log.String("nickname", req.GetNickname()),
	)

	mgr := friends.From(d.client)
	if mgr == nil {
		return nil, errors.New("friends manager not initialized")
	}

	if err := mgr.SetFriendNickname(ctx, req.GetSteamId(), req.GetNickname()); err != nil {
		return nil, err
	}

	return &pb.SetFriendNicknameResponse{
		Success: true,
		Message: "Friend nickname updated successfully.",
	}, nil
}
