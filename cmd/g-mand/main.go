// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/storage/jsonfile"
	"google.golang.org/grpc"

	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load()

	var exitCode int
	defer func() {
		if exitCode != 0 {
			os.Exit(exitCode)
		}
	}()

	var debugEnabled bool
	flag.BoolVar(&debugEnabled, "debug", false, "Enable debug logging")
	flag.BoolVar(&debugEnabled, "d", false, "Enable debug logging (shorthand)")
	flag.Parse()

	if debugEnabled {
		fmt.Println("WARNING: daemon is in the debug mode. Make sure to disable it in production.")

		go func() {
			if err := http.ListenAndServe("localhost:6060", nil); err != nil { //#nosec G114
				fmt.Fprintln(os.Stderr, "Failed to start pprof server:", err)
			}
		}()
	}

	cfg, err := loadEnvConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Config Error:", err)
		return err
	}

	store, err := jsonfile.New(cfg.StoragePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize storage: %v\n", err)
		return err
	}
	defer store.Close()

	logLevel := log.LevelInfo
	if debugEnabled {
		logLevel = log.LevelDebug
	}

	logCfg := log.DefaultConfig(logLevel)
	logCfg.FullPath = true

	logger := log.New(logCfg)
	defer logger.Close()

	shutdownCtx, shutdownFunc := context.WithCancel(context.Background())
	defer shutdownFunc()

	listener, path, err := GetIPCListener()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to listen on IPC interface:", err)
		return err
	}

	daemon, err := NewDaemon(cfg, store, logger, shutdownCtx, shutdownFunc)
	if err != nil {
		logger.Error("Failed to initialize daemon", log.Err(err))
		return err
	}

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(64*1024*1024),
		grpc.MaxSendMsgSize(64*1024*1024),
	)
	pb.RegisterDaemonServiceServer(grpcServer, daemon)

	logger.Info("gRPC IPC server listening",
		log.String("network", listener.Addr().Network()),
		log.String("address", path),
	)

	go func() {
		if err := grpcServer.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			logger.Error("gRPC server error", log.Err(err))
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("System signal received, stopping daemon...", log.String("signal", sig.String()))
		shutdownFunc()
	}()

	if err := daemon.Run(shutdownCtx); err != nil {
		logger.Error("Daemon run failed", log.Err(err))
	} else {
		daemon.StartStartupMaintenance(shutdownCtx)
		<-shutdownCtx.Done()
	}

	logger.Info("Initiating graceful shutdown...")
	grpcServer.GracefulStop()
	daemon.Close()

	if listener.Addr().Network() == "unix" {
		_ = os.Remove(path)
		logger.Info("Removed Unix socket file", log.String("path", path))
	}

	logger.Info("Daemon process exited.")

	return nil
}
