// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package game

import (
	"context"
	"strings"
	"testing"
)

func TestActionRouter_ExecutionAndValidation(t *testing.T) {
	router := NewActionRouter()

	router.Define("craft-metal").
		Desc("Combine metal").
		Param("type", "scrap or reclaimed", true).
		Param("amount", "metal count", false).
		Handle(func(ctx context.Context, p Params) (string, error) {
			metalType := p.Get("type")
			amount := p.IntOr("amount", 1)
			return metalType + ":" + string(rune('0'+amount)), nil
		})

	// Test missing required parameter
	_, err := router.Execute(context.Background(), "craft-metal", map[string]string{})
	if err == nil || !strings.Contains(err.Error(), "requires type parameter") {
		t.Fatalf("expected missing parameter error, got: %v", err)
	}

	// Test successful execution
	res, err := router.Execute(context.Background(), "craft-metal", map[string]string{
		"type":   "scrap",
		"amount": "2",
	})
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if res != "scrap:2" {
		t.Errorf("expected 'scrap:2', got %q", res)
	}

	// Test unsupported action
	_, err = router.Execute(context.Background(), "unknown-action", nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported action") {
		t.Fatalf("expected unsupported action error, got: %v", err)
	}

	// Test Actions() introspection
	actions := router.Actions()
	if len(actions) != 1 || actions[0].Name != "craft-metal" {
		t.Fatalf("unexpected actions: %+v", actions)
	}
}
