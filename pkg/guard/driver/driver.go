// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package driver

import (
	"context"
	"errors"
	"fmt"

	pbSteam "github.com/lemon4ksan/g-man/pkg/protobuf/steam"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/guard"

	"github.com/lemon4ksan/g-man-cli/pkg/game"
)

// Driver provides administrative and command support for Steam Guard.
type Driver struct {
	client *steam.Client
}

// New creates a new Steam Guard driver adapter.
func New(client *steam.Client) *Driver {
	return &Driver{
		client: client,
	}
}

// GenerateCode generates the 5-digit Steam Guard TOTP code for the current time.
func (d *Driver) GenerateCode() (string, error) {
	g := guard.From(d.client)
	if g == nil {
		return "", errors.New("guard module not registered in steam client")
	}

	return g.GenerateAuthCode()
}

// FetchConfirmations fetches the list of pending confirmations from Steam.
func (d *Driver) FetchConfirmations(ctx context.Context) ([]*guard.Confirmation, error) {
	g := guard.From(d.client)
	if g == nil {
		return nil, errors.New("guard module not registered in steam client")
	}

	return g.FetchConfirmations(ctx)
}

// AcceptConfirmation accepts a single confirmation by its ID.
func (d *Driver) AcceptConfirmation(ctx context.Context, confID uint64) error {
	g := guard.From(d.client)
	if g == nil {
		return errors.New("guard module not registered in steam client")
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

	return fmt.Errorf("confirmation %d not found", confID)
}

// CancelConfirmation cancels a single confirmation by its ID.
func (d *Driver) CancelConfirmation(ctx context.Context, confID uint64) error {
	g := guard.From(d.client)
	if g == nil {
		return errors.New("guard module not registered in steam client")
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

	return fmt.Errorf("confirmation %d not found", confID)
}

// RespondToAll approves or cancels all pending confirmations.
func (d *Driver) RespondToAll(ctx context.Context, accept bool) error {
	g := guard.From(d.client)
	if g == nil {
		return errors.New("guard module not registered in steam client")
	}

	confs, err := g.FetchConfirmations(ctx)
	if err != nil {
		return err
	}

	if len(confs) == 0 {
		return nil
	}

	if accept {
		return g.AcceptMultiple(ctx, confs)
	}

	return g.CancelMultiple(ctx, confs)
}

// QueryStatus queries the current two-factor status from Steam.
func (d *Driver) QueryStatus(ctx context.Context) (*pbSteam.CTwoFactor_Status_Response, error) {
	steamID := d.client.Session().SteamID()
	if steamID.Uint64() == 0 {
		return nil, errors.New("steam client is not logged in")
	}

	tfService := guard.NewTwoFactorService(d.client)

	return tfService.QueryStatus(ctx, steamID)
}

// RemoveAuthenticator removes/revokes the current authenticator using a revocation code.
func (d *Driver) RemoveAuthenticator(ctx context.Context, revocationCode string) error {
	tfService := guard.NewTwoFactorService(d.client)

	resp, err := tfService.RemoveAuthenticator(ctx, revocationCode)
	if err != nil {
		return err
	}

	if !resp.GetSuccess() {
		return fmt.Errorf(
			"failed to remove authenticator (success=false), %d attempts remaining",
			resp.GetRevocationAttemptsRemaining(),
		)
	}

	return nil
}

// TransferStart begins the authenticator transfer process.
// It sends a challenge request to Steam, which triggers sending an SMS verification code to the registered phone.
func (d *Driver) TransferStart(ctx context.Context) error {
	tfService := guard.NewTwoFactorService(d.client)
	_, err := tfService.RemoveAuthenticatorViaChallengeStart(ctx)
	return err
}

// TransferFinish completes the transfer process using the received SMS code.
// It returns a replacement token containing the new authenticator secrets.
func (d *Driver) TransferFinish(
	ctx context.Context,
	smsCode string,
) (*pbSteam.CRemoveAuthenticatorViaChallengeContinue_Replacement_Token, error) {
	steamID := d.client.Session().SteamID()
	if steamID.Uint64() == 0 {
		return nil, errors.New("steam client is not logged in")
	}

	tfService := guard.NewTwoFactorService(d.client)

	resp, err := tfService.RemoveAuthenticatorViaChallengeContinue(ctx, steamID, smsCode)
	if err != nil {
		return nil, err
	}

	if resp.GetReplacementToken() == nil {
		return nil, errors.New("steam did not return a replacement token")
	}

	return resp.GetReplacementToken(), nil
}

// LinkStart begins linking a new mobile authenticator to the account.
func (d *Driver) LinkStart(
	ctx context.Context,
	deviceID string,
) (*pbSteam.CTwoFactor_AddAuthenticator_Response, error) {
	steamID := d.client.Session().SteamID()
	if steamID.Uint64() == 0 {
		return nil, errors.New("steam client is not logged in")
	}

	tfService := guard.NewTwoFactorService(d.client)

	return tfService.AddAuthenticator(ctx, steamID, deviceID)
}

// LinkFinalize completes the linking of a new authenticator using the SMS/email verification code.
func (d *Driver) LinkFinalize(ctx context.Context, sharedSecret string, serverTime uint64, smsCode string) error {
	steamID := d.client.Session().SteamID()
	if steamID.Uint64() == 0 {
		return errors.New("steam client is not logged in")
	}

	tfService := guard.NewTwoFactorService(d.client)

	resp, err := tfService.FinalizeAuthenticator(ctx, steamID, sharedSecret, serverTime, smsCode)
	if err != nil {
		return err
	}

	if guard.IsFinalizeWantMore(resp) {
		return errors.New("steam wants more confirmation codes, please try again")
	}

	return nil
}

// Actions returns the list of actions supported by the Guard driver.
func (d *Driver) Actions() []game.ActionInfo {
	return []game.ActionInfo{
		{
			Name:        "status",
			Description: "Show current two-factor status",
		},
		{
			Name:        "code",
			Description: "Generate a 2FA authentication code",
		},
		{
			Name:        "list",
			Description: "List all pending confirmations",
		},
		{
			Name:        "accept",
			Description: "Accept a confirmation by its ID",
			Params: []game.ActionParam{
				{Name: "id", Description: "Confirmation ID to accept", Required: true},
			},
		},
		{
			Name:        "cancel",
			Description: "Cancel a confirmation by its ID",
			Params: []game.ActionParam{
				{Name: "id", Description: "Confirmation ID to cancel", Required: true},
			},
		},
		{
			Name:        "accept-all",
			Description: "Accept all pending confirmations",
		},
		{
			Name:        "cancel-all",
			Description: "Cancel all pending confirmations",
		},
		{
			Name:        "transfer",
			Description: "Transfer mobile authenticator to this client (2 day trade ban)",
		},
		{
			Name:        "link",
			Description: "Set up and link a new mobile authenticator",
		},
		{
			Name:        "qr",
			Description: "Generate QR codes for 2FA setup",
			Params: []game.ActionParam{
				{Name: "ascii", Description: "Use ASCII instead of unicode blocks (true/false)", Required: false},
				{Name: "format", Description: "Format: 'steam', 'bitwarden', or 'keepassxc'", Required: false},
			},
		},
		{
			Name:        "auth",
			Description: "Submit a Steam Guard/Email authentication code to the daemon",
			Params: []game.ActionParam{
				{Name: "code", Description: "Authentication code", Required: true},
			},
		},
		{
			Name:        "import",
			Description: "Import Steam Guard credentials from SDA .maFile",
			Params: []game.ActionParam{
				{Name: "file", Description: "Path to .maFile", Required: true},
			},
		},
	}
}

// Usage returns the usage information/help text for the Guard driver.
func (d *Driver) Usage() string {
	return `Steam Guard Commands:
  status           Show current two-factor status
  code             Generate a 2FA authentication code
  list             List all pending confirmations
  accept <id>      Accept a confirmation by its ID
  cancel <id>      Cancel a confirmation by its ID
  accept-all       Accept all pending confirmations
  cancel-all       Cancel all pending confirmations
  transfer         Transfer mobile authenticator to this client (2 day trade ban)
  link             Set up and link a new mobile authenticator
  import <file>    Import Steam Guard credentials from SDA .maFile
  qr [opts]        Generate QR codes for 2FA setup
  auth <code>      Submit a Steam Guard/Email auth code to the daemon
  encrypt <file>   Encrypt a sensitive file (e.g. .maFile or .env)
  decrypt <file>   Decrypt a previously encrypted file
  unlock           Unlock the daemon by sending the decryption passphrase
  encrypt-env      Encrypt sensitive variables in .env file
  decrypt-env      Decrypt sensitive variables in .env file

QR Options:
  --ascii                        Use ASCII instead of unicode blocks
  --format=<format>              Format: 'steam', 'bitwarden', or 'keepassxc' (default 'steam')`
}
