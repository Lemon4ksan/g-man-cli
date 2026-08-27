// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package shared

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
)

// ExtractSteamIDFromJWT extracts the 64-bit SteamID from the "sub" claim of a Steam JWT refresh token.
// Returns 0 if decoding fails or the token structure is invalid.
func ExtractSteamIDFromJWT(refreshToken string) uint64 {
	parts := strings.Split(refreshToken, ".")
	if len(parts) != 3 {
		return 0
	}

	payloadStr := parts[1]
	if pad := len(payloadStr) % 4; pad != 0 {
		payloadStr += strings.Repeat("=", 4-pad)
	}

	payload, err := base64.URLEncoding.DecodeString(payloadStr)
	if err != nil {
		payload, _ = base64.RawURLEncoding.DecodeString(parts[1])
	}

	if len(payload) == 0 {
		return 0
	}

	var claims struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return 0
	}

	steamID, err := strconv.ParseUint(claims.Sub, 10, 64)
	if err != nil {
		return 0
	}

	return steamID
}
