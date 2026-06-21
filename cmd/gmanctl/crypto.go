// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// readPassphrase securely reads a passphrase from the terminal without echoing it.
func readPassphrase(prompt string) (string, error) {
	fmt.Print(prompt)

	// #nosec G115
	bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}

	fmt.Println()

	return string(bytePassword), nil
}
