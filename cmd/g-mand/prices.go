// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lemon4ksan/g-man/pkg/log"

	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
)

type manualPriceJSONEntry struct {
	BuyKeys   int     `json:"buy_keys"`
	BuyMetal  float64 `json:"buy_metal"`
	SellKeys  int     `json:"sell_keys"`
	SellMetal float64 `json:"sell_metal"`
}

// UpdateManualPrices updates manual pricing values for items in the daemon.
func (s *Daemon) UpdateManualPrices(
	ctx context.Context,
	req *pb.UpdateManualPricesRequest,
) (*pb.UpdateManualPricesResponse, error) {
	s.logger.Info("Update manual prices request received", log.Int("count", len(req.GetPrices())))

	s.mu.Lock()
	defer s.mu.Unlock()

	prices := make(map[string]manualPriceJSONEntry)

	data, err := os.ReadFile(s.cfg.ManualPricesPath)
	if err == nil {
		_ = json.Unmarshal(data, &prices)
	} else if !os.IsNotExist(err) {
		s.logger.Warn("Failed to read manual prices file", log.Err(err))
	}

	for sku, entry := range req.GetPrices() {
		s.logger.Info("Updating manual price",
			log.String("sku", sku),
			log.Uint32("buy_keys", entry.GetBuyKeys()),
			log.Float64("buy_metal", entry.GetBuyMetal()),
			log.Uint32("sell_keys", entry.GetSellKeys()),
			log.Float64("sell_metal", entry.GetSellMetal()),
		)

		prices[sku] = manualPriceJSONEntry{
			BuyKeys:   int(entry.GetBuyKeys()),
			BuyMetal:  entry.GetBuyMetal(),
			SellKeys:  int(entry.GetSellKeys()),
			SellMetal: entry.GetSellMetal(),
		}
	}

	newData, err := json.MarshalIndent(prices, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manual prices: %w", err)
	}

	dir := filepath.Dir(s.cfg.ManualPricesPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create directory for manual prices: %w", err)
	}

	tmpPath := s.cfg.ManualPricesPath + ".tmp"
	if err := os.WriteFile(tmpPath, newData, 0o600); err != nil {
		return nil, fmt.Errorf("failed to write manual prices: %w", err)
	}

	if err := os.Rename(tmpPath, s.cfg.ManualPricesPath); err != nil {
		return nil, fmt.Errorf("failed to save manual prices: %w", err)
	}

	return &pb.UpdateManualPricesResponse{
		Message: fmt.Sprintf("Successfully processed %d price updates.", len(req.GetPrices())),
		Success: true,
	}, nil
}
