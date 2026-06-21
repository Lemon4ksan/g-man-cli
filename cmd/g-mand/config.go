// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
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
	flagProxyCM            string
	flagProxiesWeb         string
	flagRestLogs           bool
	flagEnableAchievements bool
	flagDoTEndpoint        string
	flagDoTHost            string
	flagCircuitBreaker     bool
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
	ProxyCM               string
	ProxiesWeb            []string
	RestLogs              bool
	EnableAchievements    bool
	DoTEndpoint           string
	DoTHost               string
	CircuitBreakerEnabled bool
}

func loadEnvConfig() (Config, error) {
	username := os.Getenv("STEAM_USER")
	if flagUsername != "" {
		username = flagUsername
	}

	password := os.Getenv("STEAM_PASS")
	if flagPassword != "" {
		password = flagPassword
	}

	refreshToken := os.Getenv("STEAM_REFRESH_TOKEN")
	if flagRefreshToken != "" {
		refreshToken = flagRefreshToken
	}

	if username == "" {
		return Config{}, errors.New("STEAM_USER environment variable or -username flag is required")
	}

	if password == "" && refreshToken == "" {
		return Config{}, errors.New(
			"either STEAM_PASS/STEAM_REFRESH_TOKEN env or -password/-refresh-token flag is required",
		)
	}

	sharedSecret := os.Getenv("STEAM_SHARED_SECRET")
	if flagSharedSecret != "" {
		sharedSecret = flagSharedSecret
	}

	identitySecret := os.Getenv("STEAM_IDENTITY_SECRET")
	if flagIdentitySecret != "" {
		identitySecret = flagIdentitySecret
	}

	deviceID := os.Getenv("STEAM_DEVICE_ID")
	if flagDeviceID != "" {
		deviceID = flagDeviceID
	}

	storagePath := getEnvOr("STEAM_STORAGE_PATH", "storage.json")
	if flagStoragePath != "" {
		storagePath = flagStoragePath
	} else if flagUsername != "" && storagePath == "storage.json" {
		storagePath = "storage-" + username + ".json"
	}

	pricesPath := getEnvOr("STEAM_MANUAL_PRICES_PATH", "cache/tf2/manual_prices.json")
	if flagPricesPath != "" {
		pricesPath = flagPricesPath
	} else if flagUsername != "" && pricesPath == "cache/tf2/manual_prices.json" {
		pricesPath = "cache/tf2/manual_prices-" + username + ".json"
	}

	socketPath := getEnvOr("GMAN_SOCKET_PATH", defaultSockPath())
	if flagSocketPath != "" {
		socketPath = flagSocketPath
	}

	trustedIDsStr := getEnvOr("STEAM_TRUSTED_IDS", "")
	if flagTrustedIDs != "" {
		trustedIDsStr = flagTrustedIDs
	}

	var trustedIDs []string
	if trustedIDsStr != "" {
		for s := range strings.SplitSeq(trustedIDsStr, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				trustedIDs = append(trustedIDs, s)
			}
		}
	}

	proxyCM := getEnvOr("STEAM_PROXY_CM", "")
	if flagProxyCM != "" {
		proxyCM = flagProxyCM
	}

	proxiesWebStr := getEnvOr("STEAM_PROXIES_WEB", "")
	if flagProxiesWeb != "" {
		proxiesWebStr = flagProxiesWeb
	}

	var proxiesWeb []string
	if proxiesWebStr != "" {
		for s := range strings.SplitSeq(proxiesWebStr, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				proxiesWeb = append(proxiesWeb, s)
			}
		}
	}

	restLogs := getEnvOr("STEAM_REST_LOGS", "") == "true"

	if flagRestLogs {
		restLogs = true
	}

	enableAchievements := getEnvOr("STEAM_ENABLE_ACHIEVEMENTS", "") == "true"
	if flagEnableAchievements {
		enableAchievements = true
	}

	doTEndpoint := getEnvOr("STEAM_DOT_ENDPOINT", "")
	if flagDoTEndpoint != "" {
		doTEndpoint = flagDoTEndpoint
	}

	doTHost := getEnvOr("STEAM_DOT_HOST", "")
	if flagDoTHost != "" {
		doTHost = flagDoTHost
	}

	circuitBreaker := getEnvOr("STEAM_CIRCUIT_BREAKER", "") == "true"
	if flagCircuitBreaker {
		circuitBreaker = true
	}

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
		ProxyCM:               proxyCM,
		ProxiesWeb:            proxiesWeb,
		RestLogs:              restLogs,
		EnableAchievements:    enableAchievements,
		DoTEndpoint:           doTEndpoint,
		DoTHost:               doTHost,
		CircuitBreakerEnabled: circuitBreaker,
	}, nil
}

func defaultSockPath() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dir := filepath.Join(home, ".config", "gman")
		_ = os.MkdirAll(dir, 0o750)
		return filepath.Join(dir, "gman.sock")
	}

	return "gman.sock"
}

func getEnvOr(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}

	return fallback
}
