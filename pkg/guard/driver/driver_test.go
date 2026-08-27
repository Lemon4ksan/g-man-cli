// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package driver

import (
	"context"
	"testing"

	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/guard"
	pb "github.com/lemon4ksan/g-man/protobuf/steam"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
)

func TestNewDriver(t *testing.T) {
	d := New(nil)
	assert.NotNil(t, d)
}

func TestIsFinalizeWantMore(t *testing.T) {
	t.Run("empty response", func(t *testing.T) {
		resp := &pb.CTwoFactor_FinalizeAddAuthenticator_Response{}
		assert.False(t, guard.IsFinalizeWantMore(resp))
	})

	t.Run("want_more set to true", func(t *testing.T) {
		resp := &pb.CTwoFactor_FinalizeAddAuthenticator_Response{}

		var unknown []byte

		unknown = protowire.AppendTag(unknown, 2, protowire.VarintType)
		unknown = protowire.AppendVarint(unknown, 1)

		resp.ProtoReflect().SetUnknown(unknown)
		assert.True(t, guard.IsFinalizeWantMore(resp))
	})

	t.Run("want_more set to false", func(t *testing.T) {
		resp := &pb.CTwoFactor_FinalizeAddAuthenticator_Response{}

		var unknown []byte

		unknown = protowire.AppendTag(unknown, 2, protowire.VarintType)
		unknown = protowire.AppendVarint(unknown, 0)

		resp.ProtoReflect().SetUnknown(unknown)
		assert.False(t, guard.IsFinalizeWantMore(resp))
	})

	t.Run("other unknown fields present", func(t *testing.T) {
		resp := &pb.CTwoFactor_FinalizeAddAuthenticator_Response{}

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
	resp := &pb.CTwoFactor_FinalizeAddAuthenticator_Response{}
	assert.False(t, guard.IsFinalizeWantMore(resp))
}

func TestIsFinalizeWantMore_MultipleField2Values(t *testing.T) {
	resp := &pb.CTwoFactor_FinalizeAddAuthenticator_Response{}

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

func TestDriver_NoGuardModule(t *testing.T) {
	cfg := steam.DefaultConfig()
	client, err := steam.NewClient(cfg)
	require.NoError(t, err)

	d := New(client)

	_, err = d.GenerateCode()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "guard module not registered")

	_, err = d.FetchConfirmations(context.Background())
	assert.Error(t, err)

	err = d.AcceptConfirmation(context.Background(), 1)
	assert.Error(t, err)

	err = d.CancelConfirmation(context.Background(), 1)
	assert.Error(t, err)

	err = d.RespondToAll(context.Background(), true)
	assert.Error(t, err)
}

func TestDriver_Methods(t *testing.T) {
	cfg := steam.DefaultConfig()
	cfgGuard := guard.DefaultConfig()
	cfgGuard.SharedSecret = "c2hhcmVkX3NlY3JldA=="
	cfgGuard.IdentitySecret = "aWRlbnRpdHlfc2VjcmV0"
	cfgGuard.DeviceID = "android:8ff408c4-e869-42b7-a36a-2d9d1be64889"

	t.Logf("cfgGuard validation: %v", cfgGuard.Validate())

	client, err := steam.NewClient(
		cfg,
		guard.WithModule(cfgGuard),
	)
	require.NoError(t, err)

	err = client.Run()
	require.NoError(t, err)

	defer client.Close()

	d := New(client)

	t.Logf("client before Run: %v", client)
	t.Logf("guard.From(client) before Run: %v", guard.From(client))

	// 1. GenerateCode
	code, err := d.GenerateCode()
	t.Logf("GenerateCode err: %v, code: %q", err, code)
	assert.NoError(t, err)
	assert.Len(t, code, 5)

	// 2. FetchConfirmations
	ctx := context.Background()
	_, err = d.FetchConfirmations(ctx)
	assert.Error(t, err)

	// 3. AcceptConfirmation
	err = d.AcceptConfirmation(ctx, 12345)
	assert.Error(t, err)

	// 4. CancelConfirmation
	err = d.CancelConfirmation(ctx, 12345)
	assert.Error(t, err)

	// 5. RespondToAll
	err = d.RespondToAll(ctx, true)
	assert.Error(t, err)
	err = d.RespondToAll(ctx, false)
	assert.Error(t, err)

	// 6. QueryStatus
	_, err = d.QueryStatus(ctx)
	assert.Error(t, err)

	// 7. RemoveAuthenticator
	err = d.RemoveAuthenticator(ctx, "revocation")
	assert.Error(t, err)

	// 8. TransferStart
	err = d.TransferStart(ctx)
	assert.Error(t, err)

	// 9. TransferFinish
	_, err = d.TransferFinish(ctx, "sms")
	assert.Error(t, err)

	// 10. LinkStart
	_, err = d.LinkStart(ctx, "device")
	assert.Error(t, err)

	// 11. LinkFinalize
	err = d.LinkFinalize(ctx, "shared", 0, "sms")
	assert.Error(t, err)
}
