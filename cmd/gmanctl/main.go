// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package main

import "os"

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
