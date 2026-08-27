// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package driver provides administrative support and command execution for Steam Guard 2FA operations.
package driver

import (
	"context"
	"errors"
	"fmt"

	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/guard"
	pbSteam "github.com/lemon4ksan/g-man/protobuf/steam"

	"github.com/lemon4ksan/g-man-cli/pkg/game"
)

// Sentinel errors for Steam Guard driver operations.
var (
	ErrGuardModuleNotRegistered = errors.New("guard module not registered in steam client")
	ErrNoAuthCodeGenerated      = errors.New("no auth code generated")
	ErrConfirmationNotFound     = errors.New("confirmation not found")
	ErrClientNotLoggedIn        = errors.New("steam client is not logged in")
	ErrNoReplacementToken       = errors.New("steam did not return a replacement token")
)

// Driver provides administrative and command support for Steam Guard authentication and confirmations.
type Driver struct {
	client *steam.Client
}

// New creates a new Steam Guard driver wrapper instance.
func New(client *steam.Client) *Driver {
	return &Driver{
		client: client,
	}
}

// GenerateCode generates the 5-character 2FA TOTP code for the current timestamp.
func (d *Driver) GenerateCode() (string, error) {
	g := guard.From(d.client)
	if g == nil {
		return "", ErrGuardModuleNotRegistered
	}

	val, ok := g.GenerateAuthCode().Value()
	if !ok {
		return "", ErrNoAuthCodeGenerated
	}

	return string(val[:]), nil
}

// FetchConfirmations retrieves all pending Steam Mobile confirmations.
func (d *Driver) FetchConfirmations(ctx context.Context) ([]*guard.Confirmation, error) {
	g := guard.From(d.client)
	if g == nil {
		return nil, ErrGuardModuleNotRegistered
	}

	return g.FetchConfirmations(ctx)
}

// AcceptConfirmation accepts a single pending mobile confirmation by its ID.
func (d *Driver) AcceptConfirmation(ctx context.Context, confID uint64) error {
	g := guard.From(d.client)
	if g == nil {
		return ErrGuardModuleNotRegistered
	}

	confs, err := g.FetchConfirmations(ctx)
	if err != nil {
		return err
	}

	for _, conf := range confs {
		if conf.ID == confID {
			return g.Accept(ctx, conf)
		}
	}

	return fmt.Errorf("%w: %d", ErrConfirmationNotFound, confID)
}

// CancelConfirmation declines a single pending mobile confirmation by its ID.
func (d *Driver) CancelConfirmation(ctx context.Context, confID uint64) error {
	g := guard.From(d.client)
	if g == nil {
		return ErrGuardModuleNotRegistered
	}

	confs, err := g.FetchConfirmations(ctx)
	if err != nil {
		return err
	}

	for _, conf := range confs {
		if conf.ID == confID {
			return g.Cancel(ctx, conf)
		}
	}

	return fmt.Errorf("%w: %d", ErrConfirmationNotFound, confID)
}

// RespondToAll approves or cancels all pending mobile confirmations in batch.
func (d *Driver) RespondToAll(ctx context.Context, accept bool) error {
	g := guard.From(d.client)
	if g == nil {
		return ErrGuardModuleNotRegistered
	}

	confs, err := g.FetchConfirmations(ctx)
	if err != nil || len(confs) == 0 {
		return err
	}

	if accept {
		return g.AcceptMultiple(ctx, confs)
	}

	return g.CancelMultiple(ctx, confs)
}

// QueryStatus queries two-factor setup state and status from Steam.
func (d *Driver) QueryStatus(ctx context.Context) (*pbSteam.CTwoFactor_Status_Response, error) {
	steamID := d.client.Session().SteamID()
	if steamID.Uint64() == 0 {
		return nil, ErrClientNotLoggedIn
	}

	return guard.NewTwoFactorService(d.client).QueryStatus(ctx, steamID)
}

// RemoveAuthenticator revokes the active mobile authenticator using a revocation code.
func (d *Driver) RemoveAuthenticator(ctx context.Context, revocationCode string) error {
	resp, err := guard.NewTwoFactorService(d.client).RemoveAuthenticator(ctx, revocationCode)
	if err != nil {
		return err
	}

	if !resp.GetSuccess() {
		return fmt.Errorf(
			"failed to remove authenticator: %d attempts remaining",
			resp.GetRevocationAttemptsRemaining(),
		)
	}

	return nil
}

// TransferStart initiates an authenticator migration challenge, triggering an SMS code to the registered phone.
func (d *Driver) TransferStart(ctx context.Context) error {
	_, err := guard.NewTwoFactorService(d.client).RemoveAuthenticatorViaChallengeStart(ctx)
	return err
}

// TransferFinish completes the authenticator transfer using the SMS code, returning replacement tokens.
func (d *Driver) TransferFinish(
	ctx context.Context,
	smsCode string,
) (*pbSteam.CRemoveAuthenticatorViaChallengeContinue_Replacement_Token, error) {
	steamID := d.client.Session().SteamID()
	if steamID.Uint64() == 0 {
		return nil, ErrClientNotLoggedIn
	}

	resp, err := guard.NewTwoFactorService(d.client).RemoveAuthenticatorViaChallengeContinue(ctx, steamID, smsCode)
	if err != nil {
		return nil, err
	}

	if resp.GetReplacementToken() == nil {
		return nil, ErrNoReplacementToken
	}

	return resp.GetReplacementToken(), nil
}

// LinkStart begins linking a new mobile authenticator to the logged in account.
func (d *Driver) LinkStart(
	ctx context.Context,
	deviceID string,
) (*pbSteam.CTwoFactor_AddAuthenticator_Response, error) {
	steamID := d.client.Session().SteamID()
	if steamID.Uint64() == 0 {
		return nil, ErrClientNotLoggedIn
	}

	return guard.NewTwoFactorService(d.client).AddAuthenticator(ctx, steamID, deviceID)
}

// LinkFinalize finalizes linking a new mobile authenticator using the received SMS verification code.
func (d *Driver) LinkFinalize(ctx context.Context, sharedSecret string, serverTime uint64, smsCode string) error {
	steamID := d.client.Session().SteamID()
	if steamID.Uint64() == 0 {
		return ErrClientNotLoggedIn
	}

	resp, err := guard.NewTwoFactorService(d.client).
		FinalizeAuthenticator(ctx, steamID, sharedSecret, serverTime, smsCode)
	if err != nil {
		return err
	}

	if guard.IsFinalizeWantMore(resp) {
		return errors.New("steam requires additional confirmation codes")
	}

	return nil
}

// Actions returns the list of administrative commands supported by the Guard driver.
func (d *Driver) Actions() []game.ActionInfo {
	return []game.ActionInfo{
		{Name: "status", Description: "Show current two-factor status"},
		{Name: "code", Description: "Generate a 2FA authentication code"},
		{Name: "list", Description: "List all pending confirmations"},
		{
			Name: "accept", Description: "Accept a confirmation by its ID",
			Params: []game.ActionParam{{Name: "id", Description: "Confirmation ID to accept", Required: true}},
		},
		{
			Name: "cancel", Description: "Cancel a confirmation by its ID",
			Params: []game.ActionParam{{Name: "id", Description: "Confirmation ID to cancel", Required: true}},
		},
		{Name: "accept-all", Description: "Accept all pending confirmations"},
		{Name: "cancel-all", Description: "Cancel all pending confirmations"},
		{Name: "transfer", Description: "Transfer mobile authenticator to this client"},
		{Name: "link", Description: "Set up and link a new mobile authenticator"},
		{
			Name: "qr", Description: "Generate QR codes for 2FA setup",
			Params: []game.ActionParam{
				{Name: "ascii", Description: "Use ASCII instead of unicode blocks", Required: false},
				{Name: "format", Description: "Format: 'steam', 'bitwarden', or 'keepassxc'", Required: false},
			},
		},
		{
			Name: "auth", Description: "Submit a Steam Guard auth code to daemon",
			Params: []game.ActionParam{{Name: "code", Description: "Authentication code", Required: true}},
		},
		{
			Name: "import", Description: "Import Steam Guard credentials from SDA .maFile",
			Params: []game.ActionParam{{Name: "file", Description: "Path to .maFile", Required: true}},
		},
	}
}

// Usage returns terminal usage help text for Steam Guard CLI commands.
func (d *Driver) Usage() string {
	return `Steam Guard Commands:
  status           Show current two-factor status
  code             Generate a 2FA authentication code
  list             List all pending confirmations
  accept <id>      Accept a confirmation by its ID
  cancel <id>      Cancel a confirmation by its ID
  accept-all       Accept all pending confirmations
  cancel-all       Cancel all pending confirmations
  transfer         Transfer mobile authenticator to this client
  link             Set up and link a new mobile authenticator
  import <file>    Import Steam Guard credentials from SDA .maFile
  qr [opts]        Generate QR codes for 2FA setup
  auth <code>      Submit a Steam Guard auth code
  encrypt <file>   Encrypt a sensitive file
  decrypt <file>   Decrypt an encrypted file
  unlock           Unlock daemon configuration with passphrase
  encrypt-env      Encrypt sensitive variables in .env
  decrypt-env      Decrypt sensitive variables in .env`
}
