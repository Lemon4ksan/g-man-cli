// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package game

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Params provides typed, error-safe helper accessors over action key-value parameters.
type Params map[string]string

// Get retrieves the string parameter value for key.
func (p Params) Get(key string) string {
	if p == nil {
		return ""
	}

	return strings.TrimSpace(p[key])
}

// Has checks if the parameter key is present.
func (p Params) Has(key string) bool {
	if p == nil {
		return false
	}

	_, exists := p[key]

	return exists
}

// StringOr returns the parameter value or fallback if empty.
func (p Params) StringOr(key, fallback string) string {
	if val := p.Get(key); val != "" {
		return val
	}

	return fallback
}

// Uint64 parses the parameter as uint64.
func (p Params) Uint64(key string) (uint64, error) {
	val := p.Get(key)
	if val == "" {
		return 0, fmt.Errorf("missing parameter %q", key)
	}

	v, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid uint64 for %q (%q): %w", key, val, err)
	}

	return v, nil
}

// Uint64Or parses uint64 or returns fallback on missing/invalid value.
func (p Params) Uint64Or(key string, fallback uint64) uint64 {
	if v, err := p.Uint64(key); err == nil {
		return v
	}

	return fallback
}

// Uint32 parses the parameter as uint32.
func (p Params) Uint32(key string) (uint32, error) {
	v, err := p.Uint64(key)
	if err != nil {
		return 0, err
	}

	return uint32(v), nil
}

// Uint32Or parses uint32 or returns fallback.
func (p Params) Uint32Or(key string, fallback uint32) uint32 {
	if v, err := p.Uint32(key); err == nil {
		return v
	}

	return fallback
}

// Int parses the parameter as integer.
func (p Params) Int(key string) (int, error) {
	val := p.Get(key)
	if val == "" {
		return 0, fmt.Errorf("missing parameter %q", key)
	}

	v, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("invalid int for %q (%q): %w", key, val, err)
	}

	return v, nil
}

// IntOr parses int or returns fallback.
func (p Params) IntOr(key string, fallback int) int {
	if v, err := p.Int(key); err == nil {
		return v
	}

	return fallback
}

// Float64 parses the parameter as float64.
func (p Params) Float64(key string) (float64, error) {
	val := p.Get(key)
	if val == "" {
		return 0, fmt.Errorf("missing parameter %q", key)
	}

	v, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid float64 for %q (%q): %w", key, val, err)
	}

	return v, nil
}

// Float64Or parses float64 or returns fallback.
func (p Params) Float64Or(key string, fallback float64) float64 {
	if v, err := p.Float64(key); err == nil {
		return v
	}

	return fallback
}

// Bool returns true if the parameter is "true", "1", "yes", or "on".
func (p Params) Bool(key string) bool {
	val := strings.ToLower(p.Get(key))
	return val == "true" || val == "1" || val == "yes" || val == "on"
}

// ActionHandler defines the execution function signature for a game action.
type ActionHandler func(ctx context.Context, p Params) (string, error)

// ActionDef encapsulates declarative metadata and execution handler for an action.
type ActionDef struct {
	info    ActionInfo
	handler ActionHandler
	aliases []string
}

// Desc sets the action human-readable documentation description.
func (a *ActionDef) Desc(desc string) *ActionDef {
	a.info.Description = desc
	return a
}

// Param registers an input parameter with validation rules.
func (a *ActionDef) Param(name, desc string, required bool) *ActionDef {
	a.info.Params = append(a.info.Params, ActionParam{
		Name:        name,
		Description: desc,
		Required:    required,
	})

	return a
}

// Aliases registers alternative command invocation names for this action.
func (a *ActionDef) Aliases(aliases ...string) *ActionDef {
	a.aliases = append(a.aliases, aliases...)
	return a
}

// Handle sets the execution function for the action.
func (a *ActionDef) Handle(fn ActionHandler) *ActionDef {
	a.handler = fn
	return a
}

// ActionRouter coordinates fast O(1) action dispatching, parameter validation, and documentation generation.
type ActionRouter struct {
	actions map[string]*ActionDef
	ordered []*ActionDef
}

// NewActionRouter constructs a new ActionRouter instance.
func NewActionRouter() *ActionRouter {
	return &ActionRouter{
		actions: make(map[string]*ActionDef),
		ordered: make([]*ActionDef, 0, 32),
	}
}

// Define begins declarative definition of an action.
func (r *ActionRouter) Define(name string) *ActionDef {
	def := &ActionDef{
		info: ActionInfo{
			Name:   name,
			Params: make([]ActionParam, 0, 4),
		},
	}
	r.actions[name] = def
	r.ordered = append(r.ordered, def)

	return def
}

// Execute validates parameters and runs the designated action handler in O(1) time.
func (r *ActionRouter) Execute(ctx context.Context, action string, rawParams map[string]string) (string, error) {
	def, exists := r.actions[action]
	if !exists {
		return "", fmt.Errorf("unsupported action: %s", action)
	}

	if def.handler == nil {
		return "", fmt.Errorf("action %q has no registered handler", action)
	}

	p := Params(rawParams)
	if p == nil {
		p = make(Params)
	}

	// Validate required parameters automatically before handler invocation
	for _, param := range def.info.Params {
		if param.Required && !p.Has(param.Name) {
			return "", fmt.Errorf("action %q requires %s parameter", action, param.Name)
		}
	}

	return def.handler(ctx, p)
}

// Actions returns the complete list of unique action metadata for introspection and CLI help.
func (r *ActionRouter) Actions() []ActionInfo {
	list := make([]ActionInfo, len(r.ordered))
	for i, def := range r.ordered {
		list[i] = def.info
	}

	return list
}
