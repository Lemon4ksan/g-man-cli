// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package game

import (
	"fmt"
	"sync"
)

// Registry manages the collection of registered game drivers.
type Registry struct {
	mu      sync.RWMutex
	drivers map[uint32]Driver
}

// NewRegistry creates a new Game Driver Registry.
func NewRegistry() *Registry {
	return &Registry{
		drivers: make(map[uint32]Driver),
	}
}

// Register adds a driver to the registry. Returns an error if the AppID is already registered.
func (r *Registry) Register(d Driver) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	appid := d.AppID()
	if _, exists := r.drivers[appid]; exists {
		return fmt.Errorf("game driver for appid %d already registered", appid)
	}

	r.drivers[appid] = d

	return nil
}

// Get retrieves a driver by AppID from the registry.
func (r *Registry) Get(appid uint32) (Driver, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	d, ok := r.drivers[appid]

	return d, ok
}

// List returns a copy of all registered drivers.
func (r *Registry) List() []Driver {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]Driver, 0, len(r.drivers))
	for _, d := range r.drivers {
		list = append(list, d)
	}

	return list
}
