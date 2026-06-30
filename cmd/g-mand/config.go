// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lemon4ksan/miyako/generic"

	guardcrypto "github.com/lemon4ksan/g-man-cli/pkg/guard/crypto"
	"github.com/lemon4ksan/g-man-cli/pkg/shared"
)

var (
	flagUsername           string
	flagPassword           string
	flagRefreshToken       string
	flagSharedSecret       string
	flagIdentitySecret     string
	flagDeviceID           string
	flagStoragePath        string
	flagPricesPath         string
	flagSocketPath         string
	flagTrustedIDs         string
	flagExcludedIDs        string
	flagProxyCM            string
	flagProxiesWeb         string
	flagRestLogs           bool
	flagEnableAchievements bool
	flagDoTEndpoint        string
	flagDoTHost            string
	flagCircuitBreaker     bool
	flagMaFilePath         string
)

func init() {
	flag.StringVar(&flagUsername, "username", "", "Steam username")
	flag.StringVar(&flagUsername, "u", "", "Steam username (shorthand)")
	flag.StringVar(&flagPassword, "password", "", "Steam password")
	flag.StringVar(&flagPassword, "p", "", "Steam password (shorthand)")
	flag.StringVar(&flagRefreshToken, "refresh-token", "", "Steam refresh token")
	flag.StringVar(&flagSharedSecret, "shared-secret", "", "Steam Guard shared secret")
	flag.StringVar(&flagIdentitySecret, "identity-secret", "", "Steam Guard identity secret")
	flag.StringVar(&flagDeviceID, "device-id", "", "Steam Guard device ID")
	flag.StringVar(&flagStoragePath, "storage", "", "Path to storage.json")
	flag.StringVar(&flagPricesPath, "prices", "", "Path to manual_prices.json")
	flag.StringVar(&flagSocketPath, "socket", "", "Path to IPC socket")
	flag.StringVar(&flagTrustedIDs, "trusted-ids", "", "Comma-separated list of trusted Steam IDs")
	flag.StringVar(&flagExcludedIDs, "excluded-ids", "", "Comma-separated list of excluded Steam IDs to auto-decline")
	flag.StringVar(&flagProxyCM, "proxy-cm", "", "Proxy URL for Connection Manager (CM) socket (SOCKS5/HTTP)")
	flag.StringVar(&flagProxiesWeb, "proxies-web", "", "Comma-separated list of proxy URLs for web API requests")
	flag.BoolVar(&flagRestLogs, "enable-rest-logs", false, "Enable REST API request logging middleware")
	flag.BoolVar(
		&flagEnableAchievements,
		"enable-achievements",
		false,
		"Enable human-like achievements simulation behavior",
	)
	flag.StringVar(&flagDoTEndpoint, "dot-endpoint", "", "DNS-over-TLS resolver endpoint (e.g. 1.1.1.1:853)")
	flag.StringVar(&flagDoTHost, "dot-host", "", "DNS-over-TLS TLS SNI hostname (e.g. cloudflare-dns.com)")
	flag.BoolVar(&flagCircuitBreaker, "circuit-breaker", false, "Enable circuit breaker middleware for REST API")
	flag.StringVar(&flagMaFilePath, "mafile", "", "Path to SDA .maFile")
}

// Config holds the configuration loaded from environment variables.
type Config struct {
	Username              string
	Password              string
	RefreshToken          string
	SharedSecret          string
	IdentitySecret        string
	DeviceID              string
	StoragePath           string
	ManualPricesPath      string
	SocketPath            string
	TrustedIDs            []string
	ExcludedIDs           []string
	ProxyCM               string
	ProxiesWeb            []string
	RestLogs              bool
	EnableAchievements    bool
	DoTEndpoint           string
	DoTHost               string
	CircuitBreakerEnabled bool
	MaFilePath            string
	MaFileEncrypted       bool
	PersonaState          int32
}

func loadEnvConfig() (Config, error) {
	username := shared.StringEnvOrFlag(flagUsername, "STEAM_USER", "")
	password := shared.StringEnvOrFlag(flagPassword, "STEAM_PASS", "")
	refreshToken := shared.StringEnvOrFlag(flagRefreshToken, "STEAM_REFRESH_TOKEN", "")
	sharedSecret := shared.StringEnvOrFlag(flagSharedSecret, "STEAM_SHARED_SECRET", "")
	identitySecret := shared.StringEnvOrFlag(flagIdentitySecret, "STEAM_IDENTITY_SECRET", "")
	deviceID := shared.StringEnvOrFlag(flagDeviceID, "STEAM_DEVICE_ID", "")
	mafilePath := shared.StringEnvOrFlag(flagMaFilePath, "STEAM_MAFILE_PATH", "")

	maFileEncrypted, err := loadMaFile(mafilePath, &username, &sharedSecret, &identitySecret, &refreshToken, &deviceID)
	if err != nil {
		return Config{}, err
	}

	if !maFileEncrypted {
		if err := validateRequiredCredentials(username, password, refreshToken); err != nil {
			return Config{}, err
		}
	}

	storagePath := resolvePerUserPath(flagStoragePath, "STEAM_STORAGE_PATH", "storage.json", flagUsername, username)
	pricesPath := resolvePerUserPath(
		flagPricesPath, "STEAM_MANUAL_PRICES_PATH",
		"cache/tf2/manual_prices.json", flagUsername, username,
	)
	socketPath := resolveSocketPath(flagSocketPath)
	trustedIDs := shared.SplitCommaList(shared.StringEnvOrFlag(flagTrustedIDs, "STEAM_TRUSTED_IDS", ""))
	excludedIDs := shared.SplitCommaList(shared.StringEnvOrFlag(flagExcludedIDs, "STEAM_EXCLUDED_IDS", ""))
	proxyCM := shared.StringEnvOrFlag(flagProxyCM, "STEAM_PROXY_CM", "")
	proxiesWeb := shared.SplitCommaList(shared.StringEnvOrFlag(flagProxiesWeb, "STEAM_PROXIES_WEB", ""))

	return Config{
		Username:              username,
		Password:              password,
		RefreshToken:          refreshToken,
		SharedSecret:          sharedSecret,
		IdentitySecret:        identitySecret,
		DeviceID:              deviceID,
		StoragePath:           storagePath,
		ManualPricesPath:      pricesPath,
		SocketPath:            socketPath,
		TrustedIDs:            trustedIDs,
		ExcludedIDs:           excludedIDs,
		ProxyCM:               proxyCM,
		ProxiesWeb:            proxiesWeb,
		RestLogs:              shared.BoolEnvOrFlag(flagRestLogs, "STEAM_REST_LOGS"),
		EnableAchievements:    shared.BoolEnvOrFlag(flagEnableAchievements, "STEAM_ENABLE_ACHIEVEMENTS"),
		DoTEndpoint:           shared.StringEnvOrFlag(flagDoTEndpoint, "STEAM_DOT_ENDPOINT", ""),
		DoTHost:               shared.StringEnvOrFlag(flagDoTHost, "STEAM_DOT_HOST", ""),
		CircuitBreakerEnabled: shared.BoolEnvOrFlag(flagCircuitBreaker, "STEAM_CIRCUIT_BREAKER"),
		MaFilePath:            mafilePath,
		MaFileEncrypted:       maFileEncrypted,
		PersonaState:          shared.ParsePersonaState(shared.GetEnvOr("STEAM_PERSONA_STATE", "1")),
	}, nil
}

func loadMaFile(
	mafilePath string,
	username, sharedSecret, identitySecret, refreshToken, deviceID *string,
) (encrypted bool, err error) {
	if mafilePath == "" {
		return false, nil
	}

	fileData, err := os.ReadFile(mafilePath)
	if err != nil {
		return false, fmt.Errorf("failed to read maFile from path %q: %w", mafilePath, err)
	}

	if len(fileData) >= 8 && string(fileData[:8]) == guardcrypto.CryptoMagic {
		return true, nil
	}

	ma, err := shared.ParseMaFile(fileData)
	if err != nil {
		return false, fmt.Errorf("failed to parse maFile JSON from %q: %w", mafilePath, err)
	}

	if err := shared.ValidateMaFile(ma); err != nil {
		return false, fmt.Errorf("invalid maFile at %q: %w", mafilePath, err)
	}

	*username = generic.Coalesce(*username, ma.AccountName)
	*sharedSecret = generic.Coalesce(ma.SharedSecret, *sharedSecret)
	*identitySecret = generic.Coalesce(ma.IdentitySecret, *identitySecret)
	*refreshToken = generic.Coalesce(ma.Tokens.RefreshToken, *refreshToken)

	if *deviceID == "" {
		steamID := generic.Coalesce(ma.SteamID, ma.Session.SteamID)
		*deviceID = shared.ResolveDeviceIDFromStr(ma.DeviceID, steamID)
	}

	return false, nil
}

func validateRequiredCredentials(username, password, refreshToken string) error {
	if username == "" && refreshToken == "" {
		return errors.New(
			"STEAM_USER environment variable or -username flag is required when refresh token is missing",
		)
	}

	if password == "" && refreshToken == "" {
		return errors.New(
			"either STEAM_PASS/STEAM_REFRESH_TOKEN env or -password/-refresh-token flag is required",
		)
	}

	return nil
}

func resolvePerUserPath(flagValue, envKey, defaultValue, flagUsernameVal, username string) string {
	path := shared.StringEnvOrFlag(flagValue, envKey, defaultValue)
	if flagValue == "" && flagUsernameVal != "" && path == defaultValue {
		ext := filepath.Ext(defaultValue)
		base := defaultValue[:len(defaultValue)-len(ext)]
		path = base + "-" + username + ext
	}

	return path
}

func resolveSocketPath(flagValue string) string {
	if os.Getenv("GMAN_CONTAINER") == "true" {
		return shared.StringEnvOrFlag(flagValue, "GMAN_SOCKET_PATH", "/tmp/gman.sock")
	}

	return shared.StringEnvOrFlag(flagValue, "GMAN_SOCKET_PATH", defaultSockPath())
}

func defaultSockPath() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dir := filepath.Join(home, ".config", "gman")
		_ = os.MkdirAll(dir, 0o750)
		return filepath.Join(dir, "gman.sock")
	}

	return "gman.sock"
}
