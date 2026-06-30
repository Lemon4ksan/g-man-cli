// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package shared

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// MaFileData represents the JSON structure of a Steam Desktop Authenticator .maFile.
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

// ParseMaFile unmarshals raw JSON bytes into MaFileData.
func ParseMaFile(data []byte) (MaFileData, error) {
	var ma MaFileData
	if err := json.Unmarshal(data, &ma); err != nil {
		return MaFileData{}, fmt.Errorf("failed to parse maFile JSON: %w", err)
	}

	return ma, nil
}

// ValidateMaFile checks that required fields are present in the MaFileData.
func ValidateMaFile(ma MaFileData) error {
	if ma.SharedSecret == "" || ma.IdentitySecret == "" {
		return errors.New("invalid maFile: missing shared_secret or identity_secret")
	}

	return nil
}

// SteamIDFromMaFile extracts the numeric SteamID from a MaFileData, checking
// both the top-level steam_id and Session.SteamID fields.
func SteamIDFromMaFile(ma MaFileData) uint64 {
	steamIDStr := ma.SteamID
	if steamIDStr == "" {
		steamIDStr = ma.Session.SteamID
	}

	if steamIDStr == "" {
		return 0
	}

	id, err := strconv.ParseUint(steamIDStr, 10, 64)
	if err != nil || id == 0 {
		return 0
	}

	return id
}
