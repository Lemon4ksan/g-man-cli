// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by aBSD-style
// license that can be found in the LICENSE file.

package shared_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lemon4ksan/g-man-cli/pkg/shared"
)

func TestGenerateRandomDeviceID(t *testing.T) {
	id := shared.GenerateRandomDeviceID()
	assert.True(t, strings.HasPrefix(id, "android:"))
	assert.Len(t, id, 8+36) // android: + UUID format
}

func TestResolveDeviceID(t *testing.T) {
	// 1. deviceID is not empty -> returns deviceID
	assert.Equal(t, "custom_id", shared.ResolveDeviceID("custom_id", 0))
	assert.Equal(t, "custom_id", shared.ResolveDeviceID("custom_id", 12345))

	// 2. steamID > 0 -> returns GetDeviceID(steamID)
	idFromSteam := shared.ResolveDeviceID("", 76561197970265728)
	assert.True(t, strings.HasPrefix(idFromSteam, "android:"))

	// 3. steamID == 0 -> returns random device ID
	idRandom := shared.ResolveDeviceID("", 0)
	assert.True(t, strings.HasPrefix(idRandom, "android:"))
	assert.Len(t, idRandom, 8+36)
}

func TestResolveDeviceIDFromStr(t *testing.T) {
	// 1. deviceID is not empty -> returns deviceID
	assert.Equal(t, "custom_id", shared.ResolveDeviceIDFromStr("custom_id", ""))
	assert.Equal(t, "custom_id", shared.ResolveDeviceIDFromStr("custom_id", "76561197970265728"))

	// 2. steamIDStr is valid uint64 -> returns GetDeviceID
	idFromSteam := shared.ResolveDeviceIDFromStr("", "76561197970265728")
	assert.True(t, strings.HasPrefix(idFromSteam, "android:"))

	// 3. steamIDStr is invalid -> returns random device ID
	idRandom := shared.ResolveDeviceIDFromStr("", "invalid_steamid")
	assert.True(t, strings.HasPrefix(idRandom, "android:"))
	assert.Len(t, idRandom, 8+36)

	// 4. steamIDStr is empty -> returns random device ID
	idRandom2 := shared.ResolveDeviceIDFromStr("", "")
	assert.True(t, strings.HasPrefix(idRandom2, "android:"))
}
