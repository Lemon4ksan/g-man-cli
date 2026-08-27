// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

// Package shared provides shared configuration parsing, environment helpers,
// JWT parsing, and SDA .maFile utilities across all binaries.
package shared

import (
	"os"
	"strconv"
	"strings"
)

// GetEnvOr returns the value of an environment variable or fallback if empty.
func GetEnvOr(name, fallback string) string {
	if val := os.Getenv(name); val != "" {
		return val
	}

	return fallback
}

// StringEnvOrFlag returns flagValue if non-empty, else env variable value, else fallback.
func StringEnvOrFlag(flagValue, envKey, fallback string) string {
	if flagValue != "" {
		return flagValue
	}

	if val := os.Getenv(envKey); val != "" {
		return val
	}

	return fallback
}

// BoolEnvOrFlag returns true if flagValue is true or the env variable equals "true".
func BoolEnvOrFlag(flagValue bool, envKey string) bool {
	if flagValue {
		return true
	}

	return os.Getenv(envKey) == "true"
}

// SplitCommaList splits a comma-separated string, trimming spaces and dropping empty segments.
func SplitCommaList(s string) []string {
	if s == "" {
		return nil
	}

	var result []string
	for part := range strings.SplitSeq(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}

	return result
}

// ParsePersonaState converts persona state strings ("online", "busy", "1") to int32 enums.
// Defaults to 1 (online) for unrecognized strings.
func ParsePersonaState(s string) int32 {
	if val, err := strconv.ParseInt(s, 10, 32); err == nil {
		return int32(val)
	}

	switch strings.ToLower(s) {
	case "offline":
		return 0
	case "online", "":
		return 1
	case "busy":
		return 2
	case "away":
		return 3
	case "snooze":
		return 4
	case "looking_to_trade", "trade":
		return 5
	case "looking_to_play", "play":
		return 6
	case "invisible":
		return 7
	default:
		return 1
	}
}
