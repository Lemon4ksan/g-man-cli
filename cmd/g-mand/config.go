// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"os"
	"path/filepath"
)

// Config holds the configuration loaded from environment variables.
type Config struct {
	Username         string
	Password         string
	RefreshToken     string
	SharedSecret     string
	IdentitySecret   string
	DeviceID         string
	StoragePath      string
	ManualPricesPath string
}

func defaultSockPath() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dir := filepath.Join(home, ".config", "gman")
		_ = os.MkdirAll(dir, 0o750)
		return filepath.Join(dir, "gman.sock")
	}

	return "gman.sock"
}

func loadEnvConfig() (Config, error) {
	username := os.Getenv("STEAM_USER")
	password := os.Getenv("STEAM_PASS")
	refreshToken := os.Getenv("STEAM_REFRESH_TOKEN")

	if username == "" && refreshToken == "" {
		return Config{}, errors.New("STEAM_USER + STEAM_PASS or STEAM_REFRESH_TOKEN environment variable is required")
	}

	if username != "" && password == "" && refreshToken == "" {
		return Config{}, errors.New("STEAM_PASS is required when using STEAM_USER without STEAM_REFRESH_TOKEN")
	}

	storagePath := os.Getenv("STEAM_STORAGE_PATH")
	if storagePath == "" {
		storagePath = "storage.json"
	}

	manualPricesPath := os.Getenv("STEAM_MANUAL_PRICES_PATH")
	if manualPricesPath == "" {
		manualPricesPath = "cache/tf2/manual_prices.json"
	}

	return Config{
		Username:         username,
		Password:         password,
		RefreshToken:     refreshToken,
		SharedSecret:     os.Getenv("STEAM_SHARED_SECRET"),
		IdentitySecret:   os.Getenv("STEAM_IDENTITY_SECRET"),
		DeviceID:         os.Getenv("STEAM_DEVICE_ID"),
		StoragePath:      storagePath,
		ManualPricesPath: manualPricesPath,
	}, nil
}
