// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package shared

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/lemon4ksan/g-man-cli/pkg/guard/crypto"
)

// GenerateRandomDeviceID creates a random Android-style Steam Guard Device ID.
func GenerateRandomDeviceID() string {
	var r [16]byte

	_, _ = rand.Read(r[:])
	sum := hex.EncodeToString(r[:])

	return fmt.Sprintf("android:%s-%s-%s-%s-%s",
		sum[:8], sum[8:12], sum[12:16], sum[16:20], sum[20:32],
	)
}

// ResolveDeviceID returns deviceID if set, derives a deterministic one from steamID, or generates a random one.
func ResolveDeviceID(deviceID string, steamID uint64) string {
	if deviceID != "" {
		return deviceID
	}

	if steamID > 0 {
		return crypto.GetDeviceID(steamID)
	}

	return GenerateRandomDeviceID()
}

// ResolveDeviceIDFromStr parses steamIDStr and resolves a Steam Device ID via [ResolveDeviceID].
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
