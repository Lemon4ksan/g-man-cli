// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam/guard"

	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
)

// GuardCode generates the current Steam Guard 2FA TOTP code.
func (s *Daemon) GuardCode(ctx context.Context, req *pb.GuardCodeRequest) (*pb.GuardCodeResponse, error) {
	guardModule := guard.From(s.client)
	if guardModule == nil {
		return nil, errors.New("guard module is not loaded")
	}

	code, err := guardModule.GenerateAuthCode()
	if err != nil {
		return nil, fmt.Errorf("failed to generate guard code: %w", err)
	}

	s.logger.Info("Generated Steam Guard code")

	return &pb.GuardCodeResponse{Code: code}, nil
}

// GuardStatus returns the current guard configuration state.
func (s *Daemon) GuardStatus(ctx context.Context, req *pb.GuardStatusRequest) (*pb.GuardStatusResponse, error) {
	guardModule := guard.From(s.client)
	if guardModule == nil {
		return &pb.GuardStatusResponse{Configured: false}, nil
	}

	sharedSecret := s.cfg.SharedSecret
	deviceID := s.cfg.DeviceID

	return &pb.GuardStatusResponse{
		Configured:   sharedSecret != "",
		DeviceId:     deviceID,
		SteamId:      s.client.SteamID().String(),
		SharedSecret: sharedSecret,
	}, nil
}

// GuardList fetches pending Steam Guard confirmations.
func (s *Daemon) GuardList(ctx context.Context, req *pb.GuardListRequest) (*pb.GuardListResponse, error) {
	guardModule := guard.From(s.client)
	if guardModule == nil {
		return nil, errors.New("guard module is not loaded")
	}

	confs, err := guardModule.FetchConfirmations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch confirmations: %w", err)
	}

	result := make([]*pb.GuardConfirmation, 0, len(confs))
	for _, c := range confs {
		result = append(result, &pb.GuardConfirmation{
			Id:          c.ID,
			Nonce:       c.Nonce,
			Title:       c.Title,
			TypeName:    fmt.Sprintf("Type %d", c.Type),
			Description: c.Description,
		})
	}

	s.logger.Info("Fetched guard confirmations", log.Int("count", len(result)))

	return &pb.GuardListResponse{Confirmations: result}, nil
}

// GuardRespond accepts or declines a confirmation.
func (s *Daemon) GuardRespond(ctx context.Context, req *pb.GuardRespondRequest) (*pb.GuardRespondResponse, error) {
	guardModule := guard.From(s.client)
	if guardModule == nil {
		return nil, errors.New("guard module is not loaded")
	}

	if req.GetAll() {
		confs, err := guardModule.FetchConfirmations(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch confirmations: %w", err)
		}

		for _, c := range confs {
			if err := guardModule.Accept(ctx, c); err != nil {
				s.logger.Error("Failed to accept confirmation", log.Uint64("id", c.ID), log.Err(err))
			}
		}

		s.logger.Info("Accepted all confirmations", log.Int("count", len(confs)))

		return &pb.GuardRespondResponse{
			Success: true,
			Message: fmt.Sprintf("Accepted %d confirmations", len(confs)),
		}, nil
	}

	confs, err := guardModule.FetchConfirmations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch confirmations: %w", err)
	}

	for _, c := range confs {
		if c.ID == req.GetConfirmationId() {
			if err := guardModule.Accept(ctx, c); err != nil {
				return nil, fmt.Errorf("failed to accept confirmation: %w", err)
			}

			s.logger.Info("Accepted confirmation", log.Uint64("id", req.GetConfirmationId()))

			return &pb.GuardRespondResponse{Success: true, Message: "Confirmation accepted"}, nil
		}
	}

	return &pb.GuardRespondResponse{Success: false, Message: "Confirmation not found"}, nil
}

// GuardImport imports Steam Guard secrets.
func (s *Daemon) GuardImport(ctx context.Context, req *pb.GuardImportRequest) (*pb.GuardImportResponse, error) {
	if req.GetSharedSecret() == "" {
		return &pb.GuardImportResponse{
			Success: false,
			Message: "shared_secret is required",
		}, nil
	}

	s.cfg.SharedSecret = req.GetSharedSecret()
	s.cfg.IdentitySecret = req.GetIdentitySecret()
	s.cfg.DeviceID = req.GetDeviceId()

	guardModule := guard.From(s.client)
	if guardModule == nil {
		return &pb.GuardImportResponse{
			Success: false,
			Message: "guard module is not loaded",
		}, nil
	}

	guardModule.SetConfig(guard.Config{
		SharedSecret:   req.GetSharedSecret(),
		IdentitySecret: req.GetIdentitySecret(),
		DeviceID:       req.GetDeviceId(),
	})

	s.logger.Info("Guard secrets imported successfully",
		log.String("account", req.GetAccountName()),
	)

	return &pb.GuardImportResponse{
		Success: true,
		Message: "Guard secrets imported successfully",
	}, nil
}
