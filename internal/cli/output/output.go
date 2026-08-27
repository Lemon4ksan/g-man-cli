// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

var (
	// IsTerminal indicates if standard output is an interactive terminal.
	IsTerminal = term.IsTerminal(int(os.Stdout.Fd()))

	// Styles for consistent CLI theming.
	TitleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	SuccessStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	WarningStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	ErrorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	MutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	KeyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	ValueStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255"))
)

// Renderer coordinates human-readable versus JSON output based on flags and TTY state.
type Renderer struct {
	JSON    bool
	NoColor bool
	Writer  io.Writer
}

// NewRenderer creates an initialized Renderer with standard output destination.
func NewRenderer(isJSON, noColor bool) *Renderer {
	return &Renderer{
		JSON:    isJSON,
		NoColor: noColor || !IsTerminal,
		Writer:  os.Stdout,
	}
}

// Render outputs data as formatted JSON if JSON mode is enabled, or executes humanPrinter.
func (r *Renderer) Render(data any, humanPrinter func() error) error {
	if r.JSON {
		return r.PrintJSON(data)
	}

	if humanPrinter != nil {
		return humanPrinter()
	}

	return nil
}

// PrintJSON formats and writes data as indented JSON to the configured writer.
func (r *Renderer) PrintJSON(data any) error {
	enc := json.NewEncoder(r.Writer)
	enc.SetIndent("", "  ")

	if err := enc.Encode(data); err != nil {
		return fmt.Errorf("failed to encode JSON output: %w", err)
	}

	return nil
}

// Success prints a styled success message.
func (r *Renderer) Success(msg string) {
	if r.NoColor {
		fmt.Fprintf(r.Writer, "OK: %s\n", msg)
		return
	}

	fmt.Fprintln(r.Writer, SuccessStyle.Render("✔ "+msg))
}

// Successf prints a formatted styled success message.
func (r *Renderer) Successf(format string, args ...any) {
	r.Success(fmt.Sprintf(format, args...))
}

// Warning prints a styled warning message.
func (r *Renderer) Warning(msg string) {
	if r.NoColor {
		fmt.Fprintf(os.Stderr, "WARN: %s\n", msg)
		return
	}

	fmt.Fprintln(os.Stderr, WarningStyle.Render("▲ "+msg))
}

// Warningf prints a formatted styled warning message.
func (r *Renderer) Warningf(format string, args ...any) {
	r.Warning(fmt.Sprintf(format, args...))
}

// Error prints a styled error message.
func (r *Renderer) Error(msg string) {
	if r.NoColor {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", msg)
		return
	}

	fmt.Fprintln(os.Stderr, ErrorStyle.Render("✖ "+msg))
}

// Errorf prints a formatted styled error message.
func (r *Renderer) Errorf(format string, args ...any) {
	r.Error(fmt.Sprintf(format, args...))
}
