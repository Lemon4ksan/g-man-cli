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
	username, password := os.Getenv("STEAM_USER"), os.Getenv("STEAM_PASS")
	if username == "" || password == "" {
		return Config{}, errors.New("STEAM_USER and STEAM_PASS environment variables are required")
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
		SharedSecret:     os.Getenv("STEAM_SHARED_SECRET"),
		IdentitySecret:   os.Getenv("STEAM_IDENTITY_SECRET"),
		DeviceID:         os.Getenv("STEAM_DEVICE_ID"),
		StoragePath:      storagePath,
		ManualPricesPath: manualPricesPath,
	}, nil
}
