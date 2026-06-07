// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
)

func GetIPCListener() (net.Listener, string, error) {
	netType := os.Getenv("GMAN_IPC_NET")
	addr := os.Getenv("GMAN_IPC_ADDR")

	if netType == "" {
		if runtime.GOOS == "windows" {
			netType = "tcp"
		} else {
			netType = "unix"
		}
	}

	if addr == "" {
		if netType == "tcp" {
			addr = "127.0.0.1:50051"
		} else {
			addr = defaultSockPath()
		}
	}

	if netType == "unix" {
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			return nil, "", fmt.Errorf("failed to remove stale socket: %w", err)
		}

		if err := os.MkdirAll(filepath.Dir(addr), 0o750); err != nil {
			return nil, "", fmt.Errorf("failed to create socket directory: %w", err)
		}
	}

	var lc net.ListenConfig

	listener, err := lc.Listen(context.Background(), netType, addr)
	if err != nil {
		return nil, "", err
	}

	if netType == "unix" {
		_ = os.Chmod(addr, 0o600)
	}

	return listener, addr, nil
}
