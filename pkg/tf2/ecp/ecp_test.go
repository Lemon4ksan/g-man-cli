// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package ecp

import (
	"testing"
)

func TestEasyCopyPaste_ToEcpAndReverse_ExpectedFormAndIntent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		originalName string
		botIntent    string
		useBold      bool
		useWordSwap  bool
		wantEcpStr   string
		wantIntent   string
	}{
		{
			name:         "basic_encoding_without_modifiers",
			originalName: "Mann Co. Supply Crate Key",
			botIntent:    "sell",
			useBold:      false,
			useWordSwap:  false,
			wantEcpStr:   "buy_Mann_Co_Supply_Crate_Key",
			wantIntent:   "buy",
		},
		{
			name:         "word_swap_and_bold_enabled",
			originalName: "Specialized Killstreak Scattergun",
			botIntent:    "sell",
			useBold:      true,
			useWordSwap:  true,
			wantEcpStr:   "𝗯𝘂𝘆_𝗦𝗽𝗲𝗰_𝗞𝘀_𝗦𝗰𝗮𝘁𝘁𝗲𝗿𝗴𝘂𝗻",
			wantIntent:   "buy",
		},
		{
			name:         "australium_with_word_swap",
			originalName: "Strange Golden Frying Pan",
			botIntent:    "buy",
			useBold:      false,
			useWordSwap:  true,
			wantEcpStr:   "sell_Strange_Golden_Frying_Pan",
			wantIntent:   "sell",
		},
		{
			name:         "name_with_seed_in_parentheses",
			originalName: "Strange Woodland Warrior (Seed #123)",
			botIntent:    "sell",
			useBold:      false,
			useWordSwap:  false,
			wantEcpStr:   "buy_Strange_Woodland_Warrior_Seed_123",
			wantIntent:   "buy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e := New()
			e.SetUseBoldChars(tt.useBold)
			e.SetUseWordSwap(tt.useWordSwap)

			// Test encoding phase
			ecpStr, err := e.ToEcpString(tt.originalName, tt.botIntent)
			if err != nil {
				t.Fatalf("Failed to encode item name: %v", err)
			}

			if ecpStr != tt.wantEcpStr {
				t.Errorf("ToEcpString(%q, %q) = %q, want %q", tt.originalName, tt.botIntent, ecpStr, tt.wantEcpStr)
			}

			// Test stateful decoding phase
			decoded, err := e.ReverseEcpString(ecpStr)
			if err != nil {
				t.Fatalf("Failed to decode ECP string: %v", err)
			}

			if decoded.OriginalItemName != tt.originalName {
				t.Errorf("ReverseEcpString(%q) name = %q, want %q", ecpStr, decoded.OriginalItemName, tt.originalName)
			}

			if decoded.DecodedIntent != tt.wantIntent {
				t.Errorf("ReverseEcpString(%q) intent = %q, want %q", ecpStr, decoded.DecodedIntent, tt.wantIntent)
			}
		})
	}
}
