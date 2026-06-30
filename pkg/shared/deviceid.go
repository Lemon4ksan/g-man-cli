// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package shared

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/lemon4ksan/g-man/pkg/crypto"
)

// GenerateRandomDeviceID creates a random Android-style Steam Guard device ID
// in the format "android:XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX".
func GenerateRandomDeviceID() string {
	var r [16]byte

	_, _ = rand.Read(r[:])
	sum := hex.EncodeToString(r[:])

	return fmt.Sprintf("android:%s-%s-%s-%s-%s",
		sum[:8], sum[8:12], sum[12:16], sum[16:20], sum[20:32],
	)
}

// ResolveDeviceID returns deviceID if non-empty, derives one from steamID via
// crypto.GetDeviceID, or falls back to a random device ID.
func ResolveDeviceID(deviceID string, steamID uint64) string {
	if deviceID != "" {
		return deviceID
	}

	if steamID > 0 {
		return crypto.GetDeviceID(steamID)
	}

	return GenerateRandomDeviceID()
}

// ResolveDeviceIDFromStr resolves a device ID from a string steamID representation.
// Returns ResolveDeviceID with a parsed steamID, or falls back to a random ID on parse error.
func ResolveDeviceIDFromStr(deviceID, steamIDStr string) string {
	if deviceID != "" {
		return deviceID
	}

	if steamIDStr != "" {
		if id, err := strconv.ParseUint(steamIDStr, 10, 64); err == nil && id > 0 {
			return crypto.GetDeviceID(id)
		}
	}

	return GenerateRandomDeviceID()
}
