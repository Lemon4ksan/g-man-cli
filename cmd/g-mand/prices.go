// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	log "github.com/lemon4ksan/foundation/async/logkit"
	"github.com/lemon4ksan/foundation/codec/json"

	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
)

type manualPriceJSONEntry struct {
	BuyKeys   int     `json:"buy_keys"`
	BuyMetal  float64 `json:"buy_metal"`
	SellKeys  int     `json:"sell_keys"`
	SellMetal float64 `json:"sell_metal"`
}

// ManualPriceManager encapsulates thread-safe in-memory caching and atomic file persistence
// for manual item pricing overrides.
type ManualPriceManager struct {
	mu     sync.RWMutex
	prices map[string]manualPriceJSONEntry
	path   string
	logger log.Logger
}

// NewManualPriceManager constructs a new price manager and loads saved manual price entries from disk.
func NewManualPriceManager(filePath string, logger log.Logger) *ManualPriceManager {
	m := &ManualPriceManager{
		prices: make(map[string]manualPriceJSONEntry),
		path:   filePath,
		logger: logger.With(log.Module("price_manager")),
	}
	m.load()

	return m
}

// Get returns a copy of the manual price entry for the given SKU if defined.
func (m *ManualPriceManager) Get(sku string) (manualPriceJSONEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.prices[sku]

	return entry, ok
}

// Update adds or updates pricing entries and flushes the entire map atomically to disk.
func (m *ManualPriceManager) Update(updates map[string]*pb.ManualPriceEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for sku, entry := range updates {
		m.logger.Info("Updating manual price entry",
			log.String("sku", sku),
			log.Uint32("buy_keys", entry.GetBuyKeys()),
			log.Float64("buy_metal", entry.GetBuyMetal()),
			log.Uint32("sell_keys", entry.GetSellKeys()),
			log.Float64("sell_metal", entry.GetSellMetal()),
		)

		m.prices[sku] = manualPriceJSONEntry{
			BuyKeys:   int(entry.GetBuyKeys()),
			BuyMetal:  entry.GetBuyMetal(),
			SellKeys:  int(entry.GetSellKeys()),
			SellMetal: entry.GetSellMetal(),
		}
	}

	return m.saveLocked()
}

func (m *ManualPriceManager) load() {
	data, err := os.ReadFile(m.path)
	if err != nil {
		if !os.IsNotExist(err) {
			m.logger.Warn("Failed to read manual prices file", log.Err(err))
		}

		return
	}

	if err := json.Unmarshal(data, &m.prices); err != nil {
		m.logger.Error("Failed to parse manual prices JSON", log.Err(err))
	} else {
		m.logger.Info("Loaded manual prices into memory", log.Int("count", len(m.prices)))
	}
}

func (m *ManualPriceManager) saveLocked() error {
	newData, err := json.MarshalIndent(m.prices, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manual prices: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.path), 0o750); err != nil {
		return fmt.Errorf("failed to create directory for manual prices: %w", err)
	}

	tmpPath := m.path + ".tmp"
	if err := os.WriteFile(tmpPath, newData, 0o600); err != nil {
		return fmt.Errorf("failed to write manual prices temp file: %w", err)
	}

	if err := os.Rename(tmpPath, m.path); err != nil {
		return fmt.Errorf("failed to atomically replace manual prices file: %w", err)
	}

	return nil
}

// UpdateManualPrices handles gRPC requests to modify manual price entries on the daemon.
func (d *Daemon) UpdateManualPrices(
	ctx context.Context,
	req *pb.UpdateManualPricesRequest,
) (*pb.UpdateManualPricesResponse, error) {
	d.logger.Info("Update manual prices request received", log.Int("count", len(req.GetPrices())))

	if err := d.priceMgr.Update(req.GetPrices()); err != nil {
		return nil, err
	}

	return &pb.UpdateManualPricesResponse{
		Message: fmt.Sprintf("Successfully processed %d price updates.", len(req.GetPrices())),
		Success: true,
	}, nil
}
