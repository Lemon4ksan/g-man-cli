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

// GetIPCListener resolves socket network types (Unix domain socket or TCP port)
// and opens a net.Listener bound to the local IPC interface.
//
// On POSIX platforms, automatically cleans up stale Unix socket files before binding.
func GetIPCListener(cfg Config) (net.Listener, string, error) {
	netType, addr := resolveIPCAddress(cfg.SocketPath)

	if netType == "unix" {
		if err := prepareUnixSocket(addr); err != nil {
			return nil, "", err
		}
	}

	var lc net.ListenConfig

	listener, err := lc.Listen(context.Background(), netType, addr)
	if err != nil {
		return nil, "", fmt.Errorf("failed to listen on %s (%s): %w", addr, netType, err)
	}

	if netType == "unix" {
		_ = os.Chmod(addr, 0o666) //nolint:gosec
	}

	return listener, addr, nil
}

func resolveIPCAddress(defaultSocketPath string) (netType, addr string) {
	netType = os.Getenv("GMAN_IPC_NET")
	addr = os.Getenv("GMAN_IPC_ADDR")

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
			addr = defaultSocketPath
		}
	}

	return netType, addr
}

func prepareUnixSocket(addr string) error {
	if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove stale socket at %q: %w", addr, err)
	}

	dir := filepath.Dir(addr)
	if err := os.MkdirAll(dir, 0o755); err != nil { //#nosec G301
		return fmt.Errorf("failed to create socket directory %q: %w", dir, err)
	}

	return nil
}
