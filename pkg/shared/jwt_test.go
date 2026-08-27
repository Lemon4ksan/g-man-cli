// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by aBSD-style
// license that can be found in the LICENSE file.

package shared_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lemon4ksan/g-man-cli/pkg/shared"
)

func TestExtractSteamIDFromJWT(t *testing.T) {
	createJWT := func(sub string, validBase64 bool) string {
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
		claims := struct {
			Sub string `json:"sub"`
		}{Sub: sub}
		claimsBytes, _ := json.Marshal(claims)

		var payload string
		if validBase64 {
			payload = base64.RawURLEncoding.EncodeToString(claimsBytes)
		} else {
			payload = string(claimsBytes)
		}

		signature := "dummysignature"

		return fmt.Sprintf("%s.%s.%s", header, payload, signature)
	}

	jwt1 := createJWT("76561197970265728", true)
	assert.Equal(t, uint64(76561197970265728), shared.ExtractSteamIDFromJWT(jwt1))

	assert.Equal(t, uint64(0), shared.ExtractSteamIDFromJWT("invalidjwt"))

	jwtEmpty := createJWT("", true)
	assert.Equal(t, uint64(0), shared.ExtractSteamIDFromJWT(jwtEmpty))

	jwtInvalidSub := createJWT("not_a_number", true)
	assert.Equal(t, uint64(0), shared.ExtractSteamIDFromJWT(jwtInvalidSub))

	jwtBadBase64 := createJWT("76561197970265728", false)
	assert.Equal(t, uint64(0), shared.ExtractSteamIDFromJWT(jwtBadBase64))
}
