// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"golang.org/x/term"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
)

var (
	flagSocket  string
	flagTCP     string
	flagJSON    bool
	flagNoColor bool
)

func getEnvOr(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}

	return fallback
}

func getEnvPath() string {
	return getEnvOr("GMAN_ENV_PATH", getEnvOr("ENV_PATH", ".env"))
}

type Config struct {
	StatsPath     string
	ConfigPath    string
	PricelistPath string
	CostBasisPath string
	HistoryPath   string
	TradingPath   string
}

func loadConfig() (*Config, error) {
	return &Config{
		StatsPath:     getEnvOr("GMAN_STATS_PATH", "cache/tf2/stats.json"),
		ConfigPath:    getEnvOr("GMAN_CONFIG_PATH", "config.json"),
		PricelistPath: getEnvOr("GMAN_PRICELIST_PATH", "cache/tf2/pricelist.json"),
		CostBasisPath: getEnvOr("GMAN_COST_BASIS_PATH", "cost_basis.json"),
		HistoryPath:   getEnvOr("GMAN_HISTORY_PATH", "cache/history.txt"),
		TradingPath:   getEnvOr("GMAN_TRADING_PATH", "trading.json"),
	}, nil
}

func getIPCConnection() (*grpc.ClientConn, pb.DaemonServiceClient, error) {
	netType := os.Getenv("GMAN_IPC_NET")
	addr := os.Getenv("GMAN_IPC_ADDR")

	if flagSocket != "" {
		netType = "unix"
		addr = flagSocket
	} else if flagTCP != "" {
		netType = "tcp"
		addr = flagTCP
	}

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
		} else if home, err := os.UserHomeDir(); err == nil && home != "" {
			addr = filepath.Join(home, ".config", "gman", "gman.sock")
		} else {
			addr = "gman.sock"
		}
	}

	target := addr
	if netType == "unix" && !strings.HasPrefix(target, "unix:") && !strings.HasPrefix(target, "passthrough:") {
		target = "passthrough:///" + target
	}

	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(64*1024*1024),
			grpc.MaxCallSendMsgSize(64*1024*1024),
		),
		grpc.WithContextDialer(func(ctx context.Context, target string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, netType, target)
		}),
	)
	if err != nil {
		return nil, nil, err
	}

	return conn, pb.NewDaemonServiceClient(conn), nil
}

func readPassphrase(prompt string) (string, error) {
	fmt.Print(prompt)

	bytePassword, err := term.ReadPassword(int(syscall.Stdin))

	fmt.Println()

	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(bytePassword)), nil
}
