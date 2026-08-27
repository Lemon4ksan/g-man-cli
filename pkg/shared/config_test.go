// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by aBSD-style
// license that can be found in the LICENSE file.

package shared_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lemon4ksan/g-man-cli/pkg/shared"
)

func TestGetEnvOr(t *testing.T) {
	key := "TEST_ENV_GET_ENV_OR"
	assert.Equal(t, "fallback", shared.GetEnvOr(key, "fallback"))

	os.Setenv(key, "actual")
	defer os.Unsetenv(key)

	assert.Equal(t, "actual", shared.GetEnvOr(key, "fallback"))
}

func TestStringEnvOrFlag(t *testing.T) {
	key := "TEST_ENV_STRING_FLAG"
	assert.Equal(t, "flag", shared.StringEnvOrFlag("flag", key, "fallback"))
	assert.Equal(t, "fallback", shared.StringEnvOrFlag("", key, "fallback"))

	os.Setenv(key, "env_val")
	defer os.Unsetenv(key)

	assert.Equal(t, "flag", shared.StringEnvOrFlag("flag", key, "fallback"))
	assert.Equal(t, "env_val", shared.StringEnvOrFlag("", key, "fallback"))
}

func TestBoolEnvOrFlag(t *testing.T) {
	key := "TEST_ENV_BOOL_FLAG"
	assert.True(t, shared.BoolEnvOrFlag(true, key))
	assert.False(t, shared.BoolEnvOrFlag(false, key))

	os.Setenv(key, "true")
	defer os.Unsetenv(key)

	assert.True(t, shared.BoolEnvOrFlag(false, key))

	os.Setenv(key, "false")
	assert.False(t, shared.BoolEnvOrFlag(false, key))
}

func TestSplitCommaList(t *testing.T) {
	assert.Nil(t, shared.SplitCommaList(""))
	assert.Equal(t, []string{"a", "b", "c"}, shared.SplitCommaList("a, b ,c"))
	assert.Equal(t, []string{"foo"}, shared.SplitCommaList("  , foo, "))
}

func TestParsePersonaState(t *testing.T) {
	tests := []struct {
		input    string
		expected int32
	}{
		{"0", 0},
		{"1", 1},
		{"offline", 0},
		{"online", 1},
		{"", 1},
		{"busy", 2},
		{"away", 3},
		{"snooze", 4},
		{"looking_to_trade", 5},
		{"trade", 5},
		{"looking_to_play", 6},
		{"play", 6},
		{"invisible", 7},
		{"unknown", 1},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, shared.ParsePersonaState(tt.input))
		})
	}
}
