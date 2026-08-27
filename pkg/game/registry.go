// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package game

import (
	"errors"
	"fmt"
	"sync"
)

// Sentinel errors for game driver registry operations.
var (
	ErrDriverAlreadyRegistered = errors.New("game driver already registered for appid")
)

// Registry manages a thread-safe catalog of active game drivers indexed by their AppID.
type Registry struct {
	mu      sync.RWMutex
	drivers map[uint32]Driver
}

// NewRegistry constructs a new Game Driver Registry instance.
func NewRegistry() *Registry {
	return &Registry{
		drivers: make(map[uint32]Driver),
	}
}

// Register registers a game driver. Returns [ErrDriverAlreadyRegistered] if the AppID is taken.
func (r *Registry) Register(d Driver) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	appid := d.AppID()
	if _, exists := r.drivers[appid]; exists {
		return fmt.Errorf("%w: %d", ErrDriverAlreadyRegistered, appid)
	}

	r.drivers[appid] = d

	return nil
}

// Get retrieves a registered game driver by its AppID.
func (r *Registry) Get(appid uint32) (Driver, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	d, ok := r.drivers[appid]

	return d, ok
}

// List returns a snapshot slice of all registered game drivers.
func (r *Registry) List() []Driver {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]Driver, 0, len(r.drivers))
	for _, d := range r.drivers {
		list = append(list, d)
	}

	return list
}
