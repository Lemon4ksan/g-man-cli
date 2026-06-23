// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lemon4ksan/g-man/pkg/crypto"

	guardcrypto "github.com/lemon4ksan/g-man-cli/pkg/guard/crypto"
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

	mafilePath := os.Getenv("STEAM_MAFILE_PATH")
	if flagMaFilePath != "" {
		mafilePath = flagMaFilePath
	}

	var maFileEncrypted bool
	if mafilePath != "" {
		fileData, err := os.ReadFile(mafilePath)
		if err != nil {
			return Config{}, fmt.Errorf("failed to read maFile from path %q: %w", mafilePath, err)
		}

		if len(fileData) >= 8 && string(fileData[:8]) == guardcrypto.CryptoMagic {
			maFileEncrypted = true
		} else {
			var ma maFileData
			if err := json.Unmarshal(fileData, &ma); err != nil {
				return Config{}, fmt.Errorf("failed to parse maFile JSON from %q: %w", mafilePath, err)
			}

			if ma.SharedSecret == "" || ma.IdentitySecret == "" {
				return Config{}, fmt.Errorf(
					"invalid maFile at %q: missing shared_secret or identity_secret",
					mafilePath,
				)
			}

			if username == "" {
				username = ma.AccountName
			}

			if sharedSecret == "" {
				sharedSecret = ma.SharedSecret
			}

			if identitySecret == "" {
				identitySecret = ma.IdentitySecret
			}

			if refreshToken == "" {
				refreshToken = ma.Tokens.RefreshToken
			}

			if deviceID == "" {
				steamID := ma.SteamID
				if steamID == "" {
					steamID = ma.Session.SteamID
				}

				deviceID = getOrGenerateDeviceID(ma.DeviceID, steamID, refreshToken)
			}
		}
	}

	if !maFileEncrypted {
		if username == "" && refreshToken == "" {
			return Config{}, errors.New(
				"STEAM_USER environment variable or -username flag is required when refresh token is missing",
			)
		}

		if password == "" && refreshToken == "" {
			return Config{}, errors.New(
				"either STEAM_PASS/STEAM_REFRESH_TOKEN env or -password/-refresh-token flag is required",
			)
		}
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

	if os.Getenv("GMAN_CONTAINER") == "true" {
		socketPath = getEnvOr("GMAN_SOCKET_PATH", "/tmp/gman.sock")
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

	excludedIDsStr := getEnvOr("STEAM_EXCLUDED_IDS", "")
	if flagExcludedIDs != "" {
		excludedIDsStr = flagExcludedIDs
	}

	var excludedIDs []string
	if excludedIDsStr != "" {
		for s := range strings.SplitSeq(excludedIDsStr, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				excludedIDs = append(excludedIDs, s)
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

	personaStateStr := getEnvOr("STEAM_PERSONA_STATE", "1")

	var personaState int32
	if val, err := strconv.ParseInt(personaStateStr, 10, 32); err == nil {
		personaState = int32(val)
	} else {
		switch strings.ToLower(personaStateStr) {
		case "offline":
			personaState = 0
		case "online":
			personaState = 1
		case "busy":
			personaState = 2
		case "away":
			personaState = 3
		case "snooze":
			personaState = 4
		case "looking_to_trade", "trade":
			personaState = 5
		case "looking_to_play", "play":
			personaState = 6
		case "invisible":
			personaState = 7
		default:
			personaState = 1
		}
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
		ExcludedIDs:           excludedIDs,
		ProxyCM:               proxyCM,
		ProxiesWeb:            proxiesWeb,
		RestLogs:              restLogs,
		EnableAchievements:    enableAchievements,
		DoTEndpoint:           doTEndpoint,
		DoTHost:               doTHost,
		CircuitBreakerEnabled: circuitBreaker,
		MaFilePath:            mafilePath,
		MaFileEncrypted:       maFileEncrypted,
		PersonaState:          personaState,
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

type maFileData struct {
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

func getOrGenerateDeviceID(deviceID, steamIDStr, refreshToken string) string {
	if deviceID != "" {
		return deviceID
	}

	var steamID uint64

	// Try Session.SteamID first
	if steamIDStr != "" {
		if id, err := strconv.ParseUint(steamIDStr, 10, 64); err == nil && id > 0 {
			steamID = id
		}
	}

	// Try RefreshToken if SteamID is still empty
	if steamID == 0 && refreshToken != "" {
		parts := strings.Split(refreshToken, ".")
		if len(parts) == 3 {
			payloadStr := parts[1]
			if pad := len(payloadStr) % 4; pad != 0 {
				payloadStr += strings.Repeat("=", 4-pad)
			}

			payload, err := base64.URLEncoding.DecodeString(payloadStr)
			if err != nil {
				payload, _ = base64.RawURLEncoding.DecodeString(parts[1])
			}

			if len(payload) > 0 {
				var claims struct {
					Sub string `json:"sub"`
				}
				if err := json.Unmarshal(payload, &claims); err == nil {
					if id, err := strconv.ParseUint(claims.Sub, 10, 64); err == nil {
						steamID = id
					}
				}
			}
		}
	}

	if steamID > 0 {
		return crypto.GetDeviceID(steamID)
	}

	// Fallback to a random device ID if we cannot find the SteamID
	var r [16]byte

	_, _ = rand.Read(r[:])
	sum := hex.EncodeToString(r[:])

	return fmt.Sprintf("android:%s-%s-%s-%s-%s",
		sum[:8],
		sum[8:12],
		sum[12:16],
		sum[16:20],
		sum[20:32],
	)
}
