// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package driver

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

// Define strict presentation sections for logical grouping.
const (
	SectionPureCurrency = 1
	SectionWeapons      = 2
	SectionCosmetics    = 3
	SectionTaunts       = 4
	SectionToolsActions = 5
	SectionCratesCases  = 6
)

// GetSectionPriority resolves the item's presentation section.
func GetSectionPriority(item *tf2.Item, s *schema.Schema) int {
	sch := s.ItemByDef(int(item.DefIndex))
	if sch == nil {
		return SectionToolsActions
	}

	norm := s.NormalizeDefindex(int(item.DefIndex))
	if norm == schema.DefKey || norm == schema.DefRefined || norm == schema.DefReclaimed || norm == schema.DefScrap {
		return SectionPureCurrency
	}

	if sch.IsWeapon() {
		return SectionWeapons
	}

	if sch.IsCosmetic() {
		return SectionCosmetics
	}

	if sch.IsTaunt() {
		return SectionTaunts
	}

	if sch.ItemClass == "supply_crate" {
		return SectionCratesCases
	}

	return SectionToolsActions
}

func fallbackQualityName(quality uint32) string {
	switch quality {
	case 0:
		return "Normal"
	case 1:
		return "Genuine"
	case 2:
		return "Rare"
	case 3:
		return "Vintage"
	case 4:
		return "Artisan"
	case 5:
		return "Unusual"
	case 6:
		return "Unique"
	case 7:
		return "Community"
	case 8:
		return "Valve"
	case 9:
		return "Self-Made"
	case 10:
		return "Customized"
	case 11:
		return "Strange"
	case 12:
		return "Completed"
	case 13:
		return "Haunted"
	case 14:
		return "Collector's"
	case 15:
		return "Decorated Weapon"
	default:
		return strconv.FormatUint(uint64(quality), 10)
	}
}

func extractQuotedString(s string) (string, bool) {
	s = strings.TrimSpace(s)
	// Check ASCII double quotes
	if strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") && len(s) >= 2 {
		return s[1 : len(s)-1], true
	}

	// Check ASCII double single quotes (used in TF2 custom names/descriptions from Steam Web API)
	if strings.HasPrefix(s, "''") && strings.HasSuffix(s, "''") && len(s) >= 4 {
		return s[2 : len(s)-2], true
	}

	// Check Unicode curly quotes (e.g. “ and ”)
	runes := []rune(s)
	if len(runes) >= 2 {
		first := runes[0]

		last := runes[len(runes)-1]
		if (first == '\u201c' && last == '\u201d') || (first == '‘' && last == '’') {
			return string(runes[1 : len(runes)-1]), true
		}
	}

	return "", false
}

func formatBytes(b uint64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.2f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.2f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
