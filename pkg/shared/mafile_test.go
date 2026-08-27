// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by aBSD-style
// license that can be found in the LICENSE file.

package shared_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lemon4ksan/g-man-cli/pkg/shared"
)

func TestParseMaFile(t *testing.T) {
	jsonData := `{
		"shared_secret": "shared",
		"identity_secret": "identity",
		"device_id": "device",
		"account_name": "account",
		"steam_id": "76561197970265728",
		"tokens": {
			"refresh_token": "token"
		},
		"Session": {
			"SteamID": "76561197970265728"
		}
	}`
	ma, err := shared.ParseMaFile([]byte(jsonData))
	assert.NoError(t, err)
	assert.Equal(t, "shared", ma.SharedSecret)
	assert.Equal(t, "identity", ma.IdentitySecret)
	assert.Equal(t, "device", ma.DeviceID)
	assert.Equal(t, "account", ma.AccountName)
	assert.Equal(t, "76561197970265728", ma.SteamID)
	assert.Equal(t, "token", ma.Tokens.RefreshToken)
	assert.Equal(t, "76561197970265728", ma.Session.SteamID)

	_, err = shared.ParseMaFile([]byte(`invalid json`))
	assert.Error(t, err)
}

func TestValidateMaFile(t *testing.T) {
	ma1 := shared.MaFileData{
		SharedSecret:   "shared",
		IdentitySecret: "identity",
	}
	assert.NoError(t, shared.ValidateMaFile(ma1))

	ma2 := shared.MaFileData{
		IdentitySecret: "identity",
	}
	assert.Error(t, shared.ValidateMaFile(ma2))

	ma3 := shared.MaFileData{
		SharedSecret: "shared",
	}
	assert.Error(t, shared.ValidateMaFile(ma3))
}

func TestSteamIDFromMaFile(t *testing.T) {
	ma1 := shared.MaFileData{SteamID: "76561197970265728"}
	assert.Equal(t, uint64(76561197970265728), shared.SteamIDFromMaFile(ma1))

	ma2 := shared.MaFileData{}
	ma2.Session.SteamID = "76561197970265728"
	assert.Equal(t, uint64(76561197970265728), shared.SteamIDFromMaFile(ma2))

	ma3 := shared.MaFileData{}
	assert.Equal(t, uint64(0), shared.SteamIDFromMaFile(ma3))

	ma4 := shared.MaFileData{SteamID: "invalid"}
	assert.Equal(t, uint64(0), shared.SteamIDFromMaFile(ma4))
}
