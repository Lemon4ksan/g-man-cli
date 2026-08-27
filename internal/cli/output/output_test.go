// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderer_PrintJSON(t *testing.T) {
	var buf bytes.Buffer

	r := &Renderer{
		JSON:   true,
		Writer: &buf,
	}

	data := map[string]string{"status": "ok", "version": "1.0"}
	if err := r.PrintJSON(data); err != nil {
		t.Fatalf("PrintJSON failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"status": "ok"`) {
		t.Errorf("expected json output, got: %s", out)
	}
}

func TestRenderer_RenderHumanVsJSON(t *testing.T) {
	var buf bytes.Buffer

	rHuman := &Renderer{
		JSON:    false,
		NoColor: true,
		Writer:  &buf,
	}

	humanCalled := false

	err := rHuman.Render(nil, func() error {
		humanCalled = true

		rHuman.Success("Operation successful")

		return nil
	})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !humanCalled {
		t.Error("expected human printer to be invoked")
	}

	if !strings.Contains(buf.String(), "OK: Operation successful") {
		t.Errorf("unexpected output: %s", buf.String())
	}
}
