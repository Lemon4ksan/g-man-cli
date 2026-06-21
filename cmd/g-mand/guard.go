// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lemon4ksan/g-man/pkg/crypto"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam"
	steamguard "github.com/lemon4ksan/g-man/pkg/steam/guard"

	gd "github.com/lemon4ksan/g-man-cli/pkg/guard/driver"
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

func (s *Daemon) updateEnvFile(sharedSecret, identitySecret, deviceID, accountName, refreshToken string) {
	envPath := ".env"
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		s.logger.Warn("No .env file found in current directory to update.")
		return
	}

	content, err := os.ReadFile(envPath)
	if err != nil {
		s.logger.Error("Failed to read .env file", log.Err(err))
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
		s.logger.Error("Failed to write to .env file", log.Err(err))
	} else {
		s.logger.Info("Successfully updated .env file with new credentials.")
	}
}

func (s *Daemon) configureGuardian(sharedSecret, identitySecret, deviceID, accountName, refreshToken string) error {
	s.mu.Lock()
	s.cfg.SharedSecret = sharedSecret
	s.cfg.IdentitySecret = identitySecret

	s.cfg.DeviceID = deviceID
	if accountName != "" {
		s.cfg.Username = accountName
	}

	if refreshToken != "" {
		s.cfg.RefreshToken = refreshToken
	}

	s.mu.Unlock()

	s.updateEnvFile(sharedSecret, identitySecret, deviceID, accountName, refreshToken)

	g := steamguard.From(s.client)
	if g == nil {
		s.logger.Info("Registering dynamic Guardian module in memory")

		newG, err := steamguard.New(steamguard.Config{
			SharedSecret:   sharedSecret,
			IdentitySecret: identitySecret,
			DeviceID:       deviceID,
			RateLimit:      2 * time.Second,
		})
		if err != nil {
			return fmt.Errorf("failed to create guardian module: %w", err)
		}

		s.client.RegisterModule(newG)
	} else {
		s.logger.Info("Updating running Guardian module config in memory")
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
func (s *Daemon) GuardCode(ctx context.Context, req *pb.GuardCodeRequest) (*pb.GuardCodeResponse, error) {
	d := gd.New(s.client)

	code, err := d.GenerateCode()
	if err != nil {
		return nil, err
	}

	return &pb.GuardCodeResponse{Code: code}, nil
}

// GuardStatus returns the status of Steam Guard on the daemon.
func (s *Daemon) GuardStatus(ctx context.Context, req *pb.GuardStatusRequest) (*pb.GuardStatusResponse, error) {
	d := gd.New(s.client)

	resp, err := d.QueryStatus(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.GuardStatusResponse{
		Configured:   s.cfg.SharedSecret != "" || s.cfg.IdentitySecret != "",
		DeviceId:     s.cfg.DeviceID,
		SteamId:      s.client.SteamID().String(),
		State:        resp.GetState(),
		SharedSecret: s.cfg.SharedSecret,
		AccountName:  s.cfg.Username,
	}, nil
}

// GuardList retrieves the pending confirmations list.
func (s *Daemon) GuardList(ctx context.Context, req *pb.GuardListRequest) (*pb.GuardListResponse, error) {
	d := gd.New(s.client)

	confs, err := d.FetchConfirmations(ctx)
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
func (s *Daemon) GuardRespond(ctx context.Context, req *pb.GuardRespondRequest) (*pb.GuardRespondResponse, error) {
	d := gd.New(s.client)

	var err error
	switch {
	case req.GetAll():
		err = d.RespondToAll(ctx, req.GetAccept())
	case req.GetAccept():
		err = d.AcceptConfirmation(ctx, req.GetConfirmationId())
	default:
		err = d.CancelConfirmation(ctx, req.GetConfirmationId())
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
func (s *Daemon) GuardTransferStart(
	ctx context.Context,
	req *pb.GuardTransferStartRequest,
) (*pb.GuardTransferStartResponse, error) {
	d := gd.New(s.client)

	err := d.TransferStart(ctx)
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
func (s *Daemon) GuardTransferFinish(
	ctx context.Context,
	req *pb.GuardTransferFinishRequest,
) (*pb.GuardTransferFinishResponse, error) {
	d := gd.New(s.client)

	token, err := d.TransferFinish(ctx, req.GetSmsCode())
	if err != nil {
		return &pb.GuardTransferFinishResponse{
			Success: false,
		}, err
	}

	sharedSecret := base64.StdEncoding.EncodeToString(token.GetSharedSecret())
	identitySecret := base64.StdEncoding.EncodeToString(token.GetIdentitySecret())

	devID := s.cfg.DeviceID
	if devID == "" {
		devID = crypto.GetDeviceID(s.client.SteamID().Uint64())
	}

	if err := s.configureGuardian(sharedSecret, identitySecret, devID, "", ""); err != nil {
		s.logger.Error("Failed to dynamically configure guardian on transfer completion", log.Err(err))
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
func (s *Daemon) GuardLinkStart(
	ctx context.Context,
	req *pb.GuardLinkStartRequest,
) (*pb.GuardLinkStartResponse, error) {
	d := gd.New(s.client)

	devID := req.GetDeviceId()
	if devID == "" {
		devID = crypto.GetDeviceID(s.client.SteamID().Uint64())
	}

	resp, err := d.LinkStart(ctx, devID)
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
func (s *Daemon) GuardLinkFinalize(
	ctx context.Context,
	req *pb.GuardLinkFinalizeRequest,
) (*pb.GuardLinkFinalizeResponse, error) {
	d := gd.New(s.client)

	err := d.LinkFinalize(ctx, req.GetSharedSecret(), req.GetServerTime(), req.GetSmsCode())
	if err != nil {
		return &pb.GuardLinkFinalizeResponse{ //nolint:nilerr
			Success: false,
			Message: err.Error(),
		}, nil
	}

	devID := req.GetDeviceId()
	if devID == "" {
		devID = crypto.GetDeviceID(s.client.SteamID().Uint64())
	}

	if err := s.configureGuardian(req.GetSharedSecret(), req.GetIdentitySecret(), devID, "", ""); err != nil {
		s.logger.Error("Failed to dynamically configure guardian on link completion", log.Err(err))

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
func (s *Daemon) GuardSubmitAuthCode(
	ctx context.Context,
	req *pb.GuardSubmitAuthCodeRequest,
) (*pb.GuardSubmitAuthCodeResponse, error) {
	s.logger.Info("Received submitted auth code")

	s.activeAuthCallbackMu.Lock()
	callback := s.activeAuthCallback
	s.activeAuthCallback = nil
	s.activeAuthCallbackMu.Unlock()

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
func (s *Daemon) GuardImport(
	ctx context.Context,
	req *pb.GuardImportRequest,
) (*pb.GuardImportResponse, error) {
	s.logger.Info("Received request to import Steam Guard credentials")

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
		steamID := s.client.SteamID().Uint64()
		if steamID == 0 && req.GetRefreshToken() != "" {
			parts := strings.Split(req.GetRefreshToken(), ".")
			if len(parts) == 3 {
				payloadStr := parts[1]
				if pad := len(payloadStr) % 4; pad != 0 {
					payloadStr += strings.Repeat("=", 4-pad)
				}
				if payload, err := base64.URLEncoding.DecodeString(payloadStr); err == nil {
					var claims struct {
						Sub string `json:"sub"`
					}
					if err := json.Unmarshal(payload, &claims); err == nil {
						if idVal, err := strconv.ParseUint(claims.Sub, 10, 64); err == nil {
							steamID = idVal
						}
					}
				}
			}
		}

		if steamID > 0 {
			devID = crypto.GetDeviceID(steamID)
		} else {
			var r [16]byte
			_, _ = rand.Read(r[:])
			sum := hex.EncodeToString(r[:])
			devID = fmt.Sprintf("android:%s-%s-%s-%s-%s",
				sum[:8],
				sum[8:12],
				sum[12:16],
				sum[16:20],
				sum[20:32],
			)
		}
	}

	if err := s.configureGuardian(
		sharedSecret,
		identitySecret,
		devID,
		req.GetAccountName(),
		req.GetRefreshToken(),
	); err != nil {
		s.logger.Error("Failed to dynamically configure guardian on import", log.Err(err))

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
func (s *Daemon) GuardUnlock(ctx context.Context, req *pb.GuardUnlockRequest) (*pb.GuardUnlockResponse, error) {
	s.mu.RLock()
	locked := s.isLocked
	s.mu.RUnlock()

	if !locked {
		return &pb.GuardUnlockResponse{
			Success: true,
			Message: "Daemon is already unlocked.",
		}, nil
	}

	resChan := make(chan error, 1)
	s.unlockChan <- unlockRequest{
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
