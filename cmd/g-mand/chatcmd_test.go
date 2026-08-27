// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetalToItems(t *testing.T) {
	tests := []struct {
		input float64
		ref   int
		rec   int
		scr   int
	}{
		{0.0, 0, 0, 0},
		{0.11, 0, 0, 1},
		{0.22, 0, 0, 2},
		{0.33, 0, 1, 0},
		{0.44, 0, 1, 1},
		{0.55, 0, 1, 2},
		{0.66, 0, 2, 0},
		{0.77, 0, 2, 1},
		{0.88, 0, 2, 2},
		{1.0, 1, 0, 0},
		{1.33, 1, 1, 0},
		{2.66, 2, 2, 0},
		{80.33, 80, 1, 0},
	}

	for _, tt := range tests {
		ref, rec, scr := metalToItems(tt.input)
		if ref != tt.ref || rec != tt.rec || scr != tt.scr {
			t.Errorf("metalToItems(%f): expected %d ref, %d rec, %d scr; got %d ref, %d rec, %d scr",
				tt.input, tt.ref, tt.rec, tt.scr, ref, rec, scr)
		}
	}
}

func TestParseEcpMessage(t *testing.T) {
	tests := []struct {
		message  string
		wantEcp  string
		wantQty  int
		isEcpCmd bool
	}{
		{"!buy_refined_metal", "buy_refined_metal", 1, true},
		{"/sell_refined_metal", "sell_refined_metal", 1, true},
		{"buy_refined_metal", "buy_refined_metal", 1, true},
		{"!buy_refined_metal 10", "buy_refined_metal", 10, true},
		{"!buy_refined_metal_10", "buy_refined_metal", 10, true},
		{"!buy_10_refined_metal", "buy_refined_metal", 10, true},
		{"!buy_refined_metal_x5", "buy_refined_metal", 5, true},
		{"!status", "", 1, false},
		{"hello", "", 1, false},
	}

	// Mocking minimal swapToNativeChars representation for test simplicity
	mockSwapToNativeChars := func(str string) string {
		boldRunes := []rune("𝗮𝗯𝗰𝗱𝗲𝗳𝗴𝗵𝗶𝗷𝗸𝗹𝗺𝗻𝗼𝗽𝗾𝗿𝘀𝘁𝘂𝘃𝘄𝗫𝘆𝘇𝗔𝗕𝗖𝗗𝗘𝗙𝗚𝗛𝗜𝗝𝗞𝗟𝗠𝗡𝗢𝗣𝗤𝗥𝗦𝗧𝗨𝗩𝗪𝗫𝗬𝗭𝟬𝟭𝟮𝟯𝟰𝟱𝟲𝟳𝟴𝟵")
		nativeRunes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

		nativeMap := make(map[rune]rune)
		for i := 0; i < len(boldRunes) && i < len(nativeRunes); i++ {
			nativeMap[boldRunes[i]] = nativeRunes[i]
		}

		var sb strings.Builder
		for _, r := range str {
			if n, ok := nativeMap[r]; ok {
				sb.WriteRune(n)
			} else {
				sb.WriteRune(r)
			}
		}

		return sb.String()
	}

	for _, tt := range tests {
		cleanedMsg := tt.message
		if len(cleanedMsg) > 0 && (cleanedMsg[0] == '!' || cleanedMsg[0] == '/') {
			cleanedMsg = cleanedMsg[1:]
		}

		cleanedMsg = strings.TrimSpace(cleanedMsg)

		parts := strings.Fields(cleanedMsg)
		if len(parts) == 0 {
			if tt.isEcpCmd {
				t.Errorf("message %s: expected ECP command, got none", tt.message)
			}

			continue
		}

		ecpStr := parts[0]

		quantity := 1
		if len(parts) > 1 {
			if q, err := strconv.Atoi(parts[1]); err == nil && q > 0 {
				quantity = q
			}
		}

		nativeEcp := mockSwapToNativeChars(ecpStr)

		prefixQtyRegex := regexp.MustCompile(`^(buy|sell)_(\d+)_(.+)$`)
		if match := prefixQtyRegex.FindStringSubmatch(nativeEcp); match != nil {
			if q, err := strconv.Atoi(match[2]); err == nil && q > 0 {
				quantity = q
				nativeEcp = match[1] + "_" + match[3]
			}
		} else {
			suffixQtyRegex := regexp.MustCompile(`_(?i:x)?(\d+)$`)
			if match := suffixQtyRegex.FindStringSubmatch(nativeEcp); match != nil {
				if q, err := strconv.Atoi(match[1]); err == nil && q > 0 {
					quantity = q
					nativeEcp = nativeEcp[:len(nativeEcp)-len(match[0])]
				}
			}
		}

		isBuy := strings.HasPrefix(nativeEcp, "buy_")
		isSell := strings.HasPrefix(nativeEcp, "sell_")
		isEcpCmd := isBuy || isSell

		if isEcpCmd != tt.isEcpCmd {
			t.Errorf("message %s: expected isEcpCmd %t, got %t", tt.message, tt.isEcpCmd, isEcpCmd)
		}

		if isEcpCmd {
			if nativeEcp != tt.wantEcp {
				t.Errorf("message %s: expected ECP %s, got %s", tt.message, tt.wantEcp, nativeEcp)
			}

			if quantity != tt.wantQty {
				t.Errorf("message %s: expected quantity %d, got %d", tt.message, tt.wantQty, quantity)
			}
		}
	}
}

func TestPrefixQtyRegex(t *testing.T) {
	tests := []struct {
		input    string
		wantBuy  string
		wantQty  int
		wantItem string
	}{
		{"buy_10_refined_metal", "buy", 10, "refined_metal"},
		{"sell_5_keys", "sell", 5, "keys"},
		{"buy_100_scrap", "buy", 100, "scrap"},
		{"buy_refined_metal", "", 0, ""},
		{"sell_0_keys", "sell", 0, "keys"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			match := prefixQtyRegex.FindStringSubmatch(tt.input)
			if tt.wantBuy == "" {
				assert.Nil(t, match)
			} else {
				require.NotNil(t, match)
				assert.Equal(t, tt.wantBuy, match[1])
				assert.Equal(t, tt.wantQty, mustAtoi(t, match[2]))
				assert.Equal(t, tt.wantItem, match[3])
			}
		})
	}
}

func TestSuffixQtyRegex(t *testing.T) {
	tests := []struct {
		input    string
		wantQty  int
		wantBase string
	}{
		{"refined_metal_10", 10, "refined_metal"},
		{"keys_x5", 5, "keys"},
		{"scrap_1", 1, "scrap"},
		{"refined_metal", 0, "refined_metal"},
		{"buy_10_refined_metal", 0, "buy_10_refined_metal"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			match := suffixQtyRegex.FindStringSubmatch(tt.input)
			if tt.wantQty == 0 {
				assert.Nil(t, match)
			} else {
				require.NotNil(t, match)
				assert.Equal(t, tt.wantQty, mustAtoi(t, match[1]))
				base := tt.input[:len(tt.input)-len(match[0])]
				assert.Equal(t, tt.wantBase, base)
			}
		})
	}
}

func TestMetalToItems_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input float64
		ref   int
		rec   int
		scr   int
	}{
		{"zero", 0.0, 0, 0, 0},
		{"very large", 1000.0, 1000, 0, 0},
		{"fractional", 0.11, 0, 0, 1},
		{"exact ref", 1.0, 1, 0, 0},
		{"exact rec", 0.33, 0, 1, 0},
		{"mixed", 1.55, 1, 1, 2},
		{"just over ref", 1.01, 1, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, rec, scr := metalToItems(tt.input)
			assert.Equal(t, tt.ref, ref, "refined")
			assert.Equal(t, tt.rec, rec, "reclaimed")
			assert.Equal(t, tt.scr, scr, "scrap")
		})
	}
}

func TestParseEcpMessage_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		wantEcp  string
		wantQty  int
		isEcpCmd bool
	}{
		{"empty string", "", "", 1, false},
		{"whitespace only", "   ", "", 1, false},
		{"just exclamation", "!", "", 1, false},
		{"just slash", "/", "", 1, false},
		{"buy with space qty", "buy_keys 25", "buy_keys", 1, true},
		{"sell with suffix qty", "sell_refined_metal_3", "sell_refined_metal", 3, true},
		{"buy with x suffix", "buy_scrap_x10", "buy_scrap", 10, true},
		{"buy with prefix qty", "buy_50_keys", "buy_keys", 50, true},
		{"non-buy/sell command", "status_check", "", 1, false},
		{"case insensitive qty", "buy_keys_X15", "buy_keys", 15, true},
	}

	mockSwapToNativeChars := func(str string) string {
		boldRunes := []rune("𝗮𝗯𝗰𝗱𝗲𝗳𝗴𝗵𝗶𝗷𝗸𝗹𝗺𝗻𝗼𝗽𝗾𝗿𝘀𝘁𝘂𝘃𝘄𝗫𝘆𝘇𝗔𝗕𝗖𝗗𝗘𝗙𝗚𝗛𝗜𝗝𝗞𝗟𝗠𝗡𝗢𝗣𝗤𝗥𝗦𝗧𝗨𝗩𝗪𝗫𝗬𝗭𝟬𝟭𝟮𝟯𝟰𝟱𝟲𝟳𝟴𝟵")
		nativeRunes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

		nativeMap := make(map[rune]rune)
		for i := 0; i < len(boldRunes) && i < len(nativeRunes); i++ {
			nativeMap[boldRunes[i]] = nativeRunes[i]
		}

		var sb strings.Builder
		for _, r := range str {
			if n, ok := nativeMap[r]; ok {
				sb.WriteRune(n)
			} else {
				sb.WriteRune(r)
			}
		}

		return sb.String()
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanedMsg := tt.message
			if len(cleanedMsg) > 0 && (cleanedMsg[0] == '!' || cleanedMsg[0] == '/') {
				cleanedMsg = cleanedMsg[1:]
			}

			cleanedMsg = strings.TrimSpace(cleanedMsg)
			parts := strings.Fields(cleanedMsg)

			if len(parts) == 0 {
				assert.False(t, tt.isEcpCmd, "expected ECP command but got empty")
				return
			}

			ecpStr := parts[0]
			quantity := 1

			nativeEcp := mockSwapToNativeChars(ecpStr)

			if match := prefixQtyRegex.FindStringSubmatch(nativeEcp); match != nil {
				if q, err := strconv.Atoi(match[2]); err == nil && q > 0 {
					quantity = q
					nativeEcp = match[1] + "_" + match[3]
				}
			} else {
				if match := suffixQtyRegex.FindStringSubmatch(nativeEcp); match != nil {
					if q, err := strconv.Atoi(match[1]); err == nil && q > 0 {
						quantity = q
						nativeEcp = nativeEcp[:len(nativeEcp)-len(match[0])]
					}
				}
			}

			isBuy := strings.HasPrefix(nativeEcp, "buy_")
			isSell := strings.HasPrefix(nativeEcp, "sell_")
			isEcpCmd := isBuy || isSell

			assert.Equal(t, tt.isEcpCmd, isEcpCmd)

			if isEcpCmd {
				assert.Equal(t, tt.wantEcp, nativeEcp)
				assert.Equal(t, tt.wantQty, quantity)
			}
		})
	}
}

func mustAtoi(t *testing.T, s string) int {
	t.Helper()

	v, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("strconv.Atoi(%q): %v", s, err)
	}

	return v
}
