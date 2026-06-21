// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package driver

import (
	"testing"

	pbSteam "github.com/lemon4ksan/g-man/pkg/protobuf/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/guard"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/encoding/protowire"
)

func TestNewDriver(t *testing.T) {
	d := New(nil)
	assert.NotNil(t, d)
}

func TestIsFinalizeWantMore(t *testing.T) {
	t.Run("empty response", func(t *testing.T) {
		resp := &pbSteam.CTwoFactor_FinalizeAddAuthenticator_Response{}
		assert.False(t, guard.IsFinalizeWantMore(resp))
	})

	t.Run("want_more set to true", func(t *testing.T) {
		resp := &pbSteam.CTwoFactor_FinalizeAddAuthenticator_Response{}

		var unknown []byte

		unknown = protowire.AppendTag(unknown, 2, protowire.VarintType)
		unknown = protowire.AppendVarint(unknown, 1)

		resp.ProtoReflect().SetUnknown(unknown)
		assert.True(t, guard.IsFinalizeWantMore(resp))
	})

	t.Run("want_more set to false", func(t *testing.T) {
		resp := &pbSteam.CTwoFactor_FinalizeAddAuthenticator_Response{}

		var unknown []byte

		unknown = protowire.AppendTag(unknown, 2, protowire.VarintType)
		unknown = protowire.AppendVarint(unknown, 0)

		resp.ProtoReflect().SetUnknown(unknown)
		assert.False(t, guard.IsFinalizeWantMore(resp))
	})

	t.Run("other unknown fields present", func(t *testing.T) {
		resp := &pbSteam.CTwoFactor_FinalizeAddAuthenticator_Response{}

		var unknown []byte

		unknown = protowire.AppendTag(unknown, 5, protowire.VarintType)
		unknown = protowire.AppendVarint(unknown, 100)
		unknown = protowire.AppendTag(unknown, 2, protowire.VarintType)
		unknown = protowire.AppendVarint(unknown, 1)

		resp.ProtoReflect().SetUnknown(unknown)
		assert.True(t, guard.IsFinalizeWantMore(resp))
	})
}

func TestDriver_Actions(t *testing.T) {
	d := New(nil)
	actions := d.Actions()

	assert.Len(t, actions, 12)

	names := make(map[string]bool)
	for _, a := range actions {
		names[a.Name] = true
	}

	expectedNames := []string{
		"status", "code", "list", "accept", "cancel",
		"accept-all", "cancel-all", "transfer", "link",
		"qr", "auth", "import",
	}

	for _, name := range expectedNames {
		assert.True(t, names[name], "missing action: %s", name)
	}
}

func TestDriver_Usage(t *testing.T) {
	d := New(nil)
	usage := d.Usage()

	assert.Contains(t, usage, "Steam Guard Commands:")
	assert.Contains(t, usage, "status")
	assert.Contains(t, usage, "code")
	assert.Contains(t, usage, "accept")
	assert.Contains(t, usage, "cancel")
	assert.Contains(t, usage, "transfer")
	assert.Contains(t, usage, "link")
	assert.Contains(t, usage, "qr")
	assert.Contains(t, usage, "encrypt")
	assert.Contains(t, usage, "decrypt")
	assert.Contains(t, usage, "unlock")
}

func TestIsFinalizeWantMore_NilResponse(t *testing.T) {
	assert.NotPanics(t, func() {
		result := guard.IsFinalizeWantMore(nil)
		assert.False(t, result)
	})
}

func TestIsFinalizeWantMore_EmptyUnknownFields(t *testing.T) {
	resp := &pbSteam.CTwoFactor_FinalizeAddAuthenticator_Response{}
	assert.False(t, guard.IsFinalizeWantMore(resp))
}

func TestIsFinalizeWantMore_MultipleField2Values(t *testing.T) {
	resp := &pbSteam.CTwoFactor_FinalizeAddAuthenticator_Response{}

	var unknown []byte

	unknown = protowire.AppendTag(unknown, 2, protowire.VarintType)
	unknown = protowire.AppendVarint(unknown, 0)
	unknown = protowire.AppendTag(unknown, 2, protowire.VarintType)
	unknown = protowire.AppendVarint(unknown, 1)

	resp.ProtoReflect().SetUnknown(unknown)
	// IsFinalizeWantMore reads the first field 2 value (false)
	assert.False(t, guard.IsFinalizeWantMore(resp))
}

func TestDriver_NewWithNilClient(t *testing.T) {
	d := New(nil)
	assert.NotNil(t, d)
	assert.Nil(t, d.client)
}
