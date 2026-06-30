// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam"
	steamguard "github.com/lemon4ksan/g-man/pkg/steam/guard"

	gd "github.com/lemon4ksan/g-man-cli/pkg/guard/driver"
	"github.com/lemon4ksan/g-man-cli/pkg/shared"
	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
)

type dynamicGuardProvider struct {
	client *steam.Client
}

func (p *dynamicGuardProvider) FetchConfirmations(ctx context.Context) ([]*steamguard.Confirmation, error) {
	g := steamguard.From(p.client)
	if g == nil {
		return nil, steamguard.ErrNotConfigured
	}

	return g.FetchConfirmations(ctx)
}

func (p *dynamicGuardProvider) AcceptMultiple(ctx context.Context, confs []*steamguard.Confirmation) error {
	g := steamguard.From(p.client)
	if g == nil {
		return steamguard.ErrNotConfigured
	}

	return g.AcceptMultiple(ctx, confs)
}

func (d *Daemon) updateEnvFile(sharedSecret, identitySecret, deviceID, accountName, refreshToken string) {
	envPath := getEnvPath()
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		d.logger.Warn("No .env file found to update.", log.String("path", envPath))
		return
	}

	content, err := os.ReadFile(envPath)
	if err != nil {
		d.logger.Error("Failed to read .env file", log.Err(err))
		return
	}

	lines := strings.Split(string(content), "\n")

	keys := map[string]string{
		"STEAM_SHARED_SECRET":   sharedSecret,
		"STEAM_IDENTITY_SECRET": identitySecret,
		"STEAM_DEVICE_ID":       deviceID,
	}
	if accountName != "" {
		keys["STEAM_USER"] = accountName
	}

	if refreshToken != "" {
		keys["STEAM_REFRESH_TOKEN"] = refreshToken
	}

	updatedKeys := make(map[string]bool)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) > 0 {
			k := strings.TrimSpace(parts[0])
			if val, ok := keys[k]; ok {
				lines[i] = fmt.Sprintf("%s=%s", k, val)
				updatedKeys[k] = true
			}
		}
	}

	// Append keys that weren't found
	var toAppend []string
	for k, val := range keys {
		if !updatedKeys[k] {
			toAppend = append(toAppend, fmt.Sprintf("%s=%s", k, val))
		}
	}

	if len(toAppend) > 0 {
		// ensure there is a trailing newline if not already present
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}

		lines = append(lines, toAppend...)
	}

	newContent := strings.Join(lines, "\n")
	if err := os.WriteFile(envPath, []byte(newContent), 0o600); err != nil {
		d.logger.Error("Failed to write to .env file", log.Err(err))
	} else {
		d.logger.Info("Successfully updated .env file with new credentials.")
	}
}

func (d *Daemon) configureGuardian(sharedSecret, identitySecret, deviceID, accountName, refreshToken string) error {
	d.mu.Lock()
	d.cfg.SharedSecret = sharedSecret
	d.cfg.IdentitySecret = identitySecret

	d.cfg.DeviceID = deviceID
	if accountName != "" {
		d.cfg.Username = accountName
	}

	if refreshToken != "" {
		d.cfg.RefreshToken = refreshToken
	}

	d.mu.Unlock()

	d.updateEnvFile(sharedSecret, identitySecret, deviceID, accountName, refreshToken)

	g := steamguard.From(d.client)
	if g == nil {
		d.logger.Info("Registering dynamic Guardian module in memory")

		newG, err := steamguard.New(steamguard.Config{
			SharedSecret:   sharedSecret,
			IdentitySecret: identitySecret,
			DeviceID:       deviceID,
			RateLimit:      2 * time.Second,
		})
		if err != nil {
			return fmt.Errorf("failed to create guardian module: %w", err)
		}

		d.client.RegisterModule(newG)
	} else {
		d.logger.Info("Updating running Guardian module config in memory")
		g.SetConfig(steamguard.Config{
			SharedSecret:   sharedSecret,
			IdentitySecret: identitySecret,
			DeviceID:       deviceID,
			RateLimit:      2 * time.Second,
		})
	}

	return nil
}

// GuardCode generates the current Steam Guard 2FA auth code.
func (d *Daemon) GuardCode(ctx context.Context, req *pb.GuardCodeRequest) (*pb.GuardCodeResponse, error) {
	driver := gd.New(d.client)

	code, err := driver.GenerateCode()
	if err != nil {
		return nil, err
	}

	return &pb.GuardCodeResponse{Code: code}, nil
}

// GuardStatus returns the status of Steam Guard on the daemon.
func (d *Daemon) GuardStatus(ctx context.Context, req *pb.GuardStatusRequest) (*pb.GuardStatusResponse, error) {
	driver := gd.New(d.client)

	resp, err := driver.QueryStatus(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.GuardStatusResponse{
		Configured:   d.cfg.SharedSecret != "" || d.cfg.IdentitySecret != "",
		DeviceId:     d.cfg.DeviceID,
		SteamId:      d.client.Session().SteamID().String(),
		State:        resp.GetState(),
		SharedSecret: d.cfg.SharedSecret,
		AccountName:  d.cfg.Username,
	}, nil
}

// GuardList retrieves the pending confirmations list.
func (d *Daemon) GuardList(ctx context.Context, req *pb.GuardListRequest) (*pb.GuardListResponse, error) {
	driver := gd.New(d.client)

	confs, err := driver.FetchConfirmations(ctx)
	if err != nil {
		return nil, err
	}

	pbConfs := make([]*pb.GuardConfirmation, len(confs))
	for i, c := range confs {
		pbConfs[i] = &pb.GuardConfirmation{
			Id:          c.ID,
			Nonce:       c.Nonce,
			Title:       c.Title,
			TypeName:    c.Type.String(),
			Description: c.Description,
		}
	}

	return &pb.GuardListResponse{Confirmations: pbConfs}, nil
}

// GuardRespond allows accepting or cancelling a confirmation.
func (d *Daemon) GuardRespond(ctx context.Context, req *pb.GuardRespondRequest) (*pb.GuardRespondResponse, error) {
	driver := gd.New(d.client)

	var err error
	switch {
	case req.GetAll():
		err = driver.RespondToAll(ctx, req.GetAccept())
	case req.GetAccept():
		err = driver.AcceptConfirmation(ctx, req.GetConfirmationId())
	default:
		err = driver.CancelConfirmation(ctx, req.GetConfirmationId())
	}

	if err != nil {
		return &pb.GuardRespondResponse{ //nolint:nilerr
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.GuardRespondResponse{
		Success: true,
		Message: "Confirmation response processed successfully.",
	}, nil
}

// GuardTransferStart starts the mobile authenticator transfer challenge.
func (d *Daemon) GuardTransferStart(
	ctx context.Context,
	req *pb.GuardTransferStartRequest,
) (*pb.GuardTransferStartResponse, error) {
	driver := gd.New(d.client)

	err := driver.TransferStart(ctx)
	if err != nil {
		return &pb.GuardTransferStartResponse{ //nolint:nilerr
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.GuardTransferStartResponse{
		Success: true,
		Message: "Authenticator transfer initiated successfully. An SMS code has been sent to your phone number.",
	}, nil
}

// GuardTransferFinish completes the mobile authenticator transfer using the SMS code.
func (d *Daemon) GuardTransferFinish(
	ctx context.Context,
	req *pb.GuardTransferFinishRequest,
) (*pb.GuardTransferFinishResponse, error) {
	driver := gd.New(d.client)

	token, err := driver.TransferFinish(ctx, req.GetSmsCode())
	if err != nil {
		return &pb.GuardTransferFinishResponse{
			Success: false,
		}, err
	}

	sharedSecret := base64.StdEncoding.EncodeToString(token.GetSharedSecret())
	identitySecret := base64.StdEncoding.EncodeToString(token.GetIdentitySecret())

	devID := shared.ResolveDeviceID(d.cfg.DeviceID, d.client.Session().SteamID().Uint64())

	if err := d.configureGuardian(sharedSecret, identitySecret, devID, "", ""); err != nil {
		d.logger.Error("Failed to dynamically configure guardian on transfer completion", log.Err(err))
	}

	return &pb.GuardTransferFinishResponse{
		Success:        true,
		SharedSecret:   sharedSecret,
		IdentitySecret: identitySecret,
		RevocationCode: token.GetRevocationCode(),
		DeviceId:       devID,
		Uri:            token.GetUri(),
	}, nil
}

// GuardLinkStart starts linking a new mobile authenticator.
func (d *Daemon) GuardLinkStart(
	ctx context.Context,
	req *pb.GuardLinkStartRequest,
) (*pb.GuardLinkStartResponse, error) {
	driver := gd.New(d.client)

	devID := shared.ResolveDeviceID(req.GetDeviceId(), d.client.Session().SteamID().Uint64())

	resp, err := driver.LinkStart(ctx, devID)
	if err != nil {
		return &pb.GuardLinkStartResponse{Success: false}, err
	}

	return &pb.GuardLinkStartResponse{
		Success:         true,
		SharedSecret:    base64.StdEncoding.EncodeToString(resp.GetSharedSecret()),
		IdentitySecret:  base64.StdEncoding.EncodeToString(resp.GetIdentitySecret()),
		RevocationCode:  resp.GetRevocationCode(),
		DeviceId:        devID,
		Uri:             resp.GetUri(),
		PhoneNumberHint: resp.GetPhoneNumberHint(),
		ServerTime:      resp.GetServerTime(),
	}, nil
}

// GuardLinkFinalize finalizes linking a new mobile authenticator.
func (d *Daemon) GuardLinkFinalize(
	ctx context.Context,
	req *pb.GuardLinkFinalizeRequest,
) (*pb.GuardLinkFinalizeResponse, error) {
	driver := gd.New(d.client)

	err := driver.LinkFinalize(ctx, req.GetSharedSecret(), req.GetServerTime(), req.GetSmsCode())
	if err != nil {
		return &pb.GuardLinkFinalizeResponse{ //nolint:nilerr
			Success: false,
			Message: err.Error(),
		}, nil
	}

	devID := shared.ResolveDeviceID(req.GetDeviceId(), d.client.Session().SteamID().Uint64())

	if err := d.configureGuardian(req.GetSharedSecret(), req.GetIdentitySecret(), devID, "", ""); err != nil {
		d.logger.Error("Failed to dynamically configure guardian on link completion", log.Err(err))

		return &pb.GuardLinkFinalizeResponse{
			Success: false,
			Message: fmt.Sprintf("Authenticator linked, but daemon config failed: %v", err),
		}, nil
	}

	return &pb.GuardLinkFinalizeResponse{
		Success: true,
		Message: "Authenticator successfully finalized and linked.",
	}, nil
}

// GuardSubmitAuthCode submits a 2FA/Email authentication code to continue logon.
func (d *Daemon) GuardSubmitAuthCode(
	ctx context.Context,
	req *pb.GuardSubmitAuthCodeRequest,
) (*pb.GuardSubmitAuthCodeResponse, error) {
	d.logger.Info("Received submitted auth code")

	d.activeAuthCallbackMu.Lock()
	callback := d.activeAuthCallback
	d.activeAuthCallback = nil
	d.activeAuthCallbackMu.Unlock()

	if callback == nil {
		return &pb.GuardSubmitAuthCodeResponse{
			Success: false,
			Message: "No active authentication session waiting for a code",
		}, nil
	}

	// Submit the code to the callback in a separate goroutine to prevent gRPC deadlock
	go callback(req.GetCode())

	return &pb.GuardSubmitAuthCodeResponse{
		Success: true,
		Message: "Authentication code submitted successfully",
	}, nil
}

// GuardImport dynamically configures Steam Guard secrets in the daemon.
func (d *Daemon) GuardImport(
	ctx context.Context,
	req *pb.GuardImportRequest,
) (*pb.GuardImportResponse, error) {
	d.logger.Info("Received request to import Steam Guard credentials")

	sharedSecret := req.GetSharedSecret()
	identitySecret := req.GetIdentitySecret()
	devID := req.GetDeviceId()

	if sharedSecret == "" || identitySecret == "" {
		return &pb.GuardImportResponse{
			Success: false,
			Message: "Shared secret and Identity secret cannot be empty",
		}, nil
	}

	if devID == "" {
		steamID := d.client.Session().SteamID().Uint64()
		if steamID == 0 && req.GetRefreshToken() != "" {
			steamID = shared.ExtractSteamIDFromJWT(req.GetRefreshToken())
		}

		devID = shared.ResolveDeviceID("", steamID)
	}

	if err := d.configureGuardian(
		sharedSecret,
		identitySecret,
		devID,
		req.GetAccountName(),
		req.GetRefreshToken(),
	); err != nil {
		d.logger.Error("Failed to dynamically configure guardian on import", log.Err(err))

		return &pb.GuardImportResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to configure Guardian: %v", err),
		}, nil
	}

	return &pb.GuardImportResponse{
		Success: true,
		Message: "Steam Guard credentials imported successfully",
	}, nil
}

// GuardUnlock handles unlocking the daemon by decrypting credentials using a passphrase.
func (d *Daemon) GuardUnlock(ctx context.Context, req *pb.GuardUnlockRequest) (*pb.GuardUnlockResponse, error) {
	d.mu.RLock()
	locked := d.isLocked
	d.mu.RUnlock()

	if !locked {
		return &pb.GuardUnlockResponse{
			Success: true,
			Message: "Daemon is already unlocked.",
		}, nil
	}

	resChan := make(chan error, 1)
	d.unlockChan <- unlockRequest{
		passphrase: req.GetPassphrase(),
		resChan:    resChan,
	}

	unlockErr := <-resChan
	if unlockErr != nil {
		//nolint:nilerr
		return &pb.GuardUnlockResponse{
			Success: false,
			Message: unlockErr.Error(),
		}, nil
	}

	return &pb.GuardUnlockResponse{
		Success: true,
		Message: "Daemon unlocked successfully.",
	}, nil
}
