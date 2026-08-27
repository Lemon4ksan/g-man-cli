// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package shared

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

var ErrMissingSecretsInMaFile = errors.New("invalid maFile: missing required shared_secret or identity_secret")

// MaFileData represents the JSON payload structure of a Steam Desktop Authenticator .maFile.
type MaFileData struct {
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

// ParseMaFile unmarshals JSON byte payloads into a [MaFileData] structure.
func ParseMaFile(data []byte) (MaFileData, error) {
	var ma MaFileData
	if err := json.Unmarshal(data, &ma); err != nil {
		return MaFileData{}, fmt.Errorf("failed to parse maFile JSON: %w", err)
	}

	return ma, nil
}

// ValidateMaFile validates presence of mandatory 2FA secret keys in [MaFileData].
func ValidateMaFile(ma MaFileData) error {
	if ma.SharedSecret == "" || ma.IdentitySecret == "" {
		return ErrMissingSecretsInMaFile
	}

	return nil
}

// SteamIDFromMaFile resolves the 64-bit SteamID from top-level or nested session fields.
func SteamIDFromMaFile(ma MaFileData) uint64 {
	steamIDStr := ma.SteamID
	if steamIDStr == "" {
		steamIDStr = ma.Session.SteamID
	}

	if steamIDStr == "" {
		return 0
	}

	steamID, err := strconv.ParseUint(steamIDStr, 10, 64)
	if err != nil || steamID == 0 {
		return 0
	}

	return steamID
}
