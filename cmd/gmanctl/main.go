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
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/lemon4ksan/g-man-cli/pkg/protobuf/daemon"
)

// ANSI color escape codes for high-quality terminal visuals
const (
	ColorReset  = "\033[0m"
	ColorBold   = "\033[1m"
	ColorGreen  = "\033[32m"
	ColorRed    = "\033[31m"
	ColorYellow = "\033[33m"
	ColorCyan   = "\033[36m"
	ColorGray   = "\033[90m"
)

func main() {
	var exitCode int
	defer func() {
		if exitCode != 0 {
			os.Exit(exitCode)
		}
	}()

	if len(os.Args) < 2 {
		printUsage()

		exitCode = 1

		return
	}

	command := os.Args[1]

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	conn, err := GetIPCConnection(ctx)
	if err != nil {
		cancel()
		fmt.Fprintf(os.Stderr, "%sError connecting to daemon: %v%s\n", ColorRed, err, ColorReset)
		fmt.Fprintf(os.Stderr, "%sIs the daemon 'g-mand' running?%s\n", ColorYellow, ColorReset)

		exitCode = 1

		return
	}

	defer cancel()
	defer conn.Close()

	client := pb.NewDaemonServiceClient(conn)

	switch command {
	case "status":
		handleStatus(ctx, client)
	case "stop":
		handleStop(ctx, client)
	case "gc":
		handleFreeMemory(ctx, client)
	case "play":
		if len(os.Args) < 3 {
			fmt.Printf("%sError: 'play' command requires an AppID. Example: gmanctl play 440%s\n", ColorRed, ColorReset)

			exitCode = 1

			return
		}

		appID, err := strconv.ParseUint(os.Args[2], 10, 32)
		if err != nil {
			fmt.Printf("%sError: Invalid AppID %q. Must be an integer.%s\n", ColorRed, os.Args[2], ColorReset)

			exitCode = 1

			return
		}

		handlePlay(ctx, client, uint32(appID))

	case "exit-game":
		handleExitGame(ctx, client)
	case "exec":
		if len(os.Args) < 4 {
			fmt.Printf(
				"%sError: 'exec' command requires AppID and Action name. Example: gmanctl exec 440 craft-metal%s\n",
				ColorRed,
				ColorReset,
			)

			exitCode = 1

			return
		}

		appID, err := strconv.ParseUint(os.Args[2], 10, 32)
		if err != nil {
			fmt.Printf("%sError: Invalid AppID %q.%s\n", ColorRed, os.Args[2], ColorReset)

			exitCode = 1

			return
		}

		action := os.Args[3]

		params := make(map[string]string)
		for _, arg := range os.Args[4:] {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) == 2 {
				params[parts[0]] = parts[1]
			} else {
				params[arg] = "true"
			}
		}

		handleExec(ctx, client, uint32(appID), action, params)

	case "help":
		printUsage()
	default:
		fmt.Printf("%sUnknown command: %s%s\n", ColorRed, command, ColorReset)
		printUsage()

		exitCode = 1

		return
	}
}

func printUsage() {
	fmt.Printf("%s%sG-MAN Command Line Control Interface (gmanctl)%s\n\n", ColorCyan, ColorBold, ColorReset)
	fmt.Println("Usage:")
	fmt.Println("  gmanctl [command] [args...]")
	fmt.Println("\nSystem Commands:")
	fmt.Printf("  %-30s %s\n", "status", "Show daemon status, Steam connection and resource metrics")
	fmt.Printf("  %-30s %s\n", "stop", "Stop the background daemon gracefully")
	fmt.Printf("  %-30s %s\n", "gc", "Force manual garbage collection and free physical memory")
	fmt.Println("\nGame Commands:")
	fmt.Printf("  %-30s %s\n", "play <appid>", "Launch game session & initialize Game Coordinator")
	fmt.Printf("  %-30s %s\n", "exit-game", "Close active game session, return to simple online mode")
	fmt.Println("\nAction Commands:")
	fmt.Printf("  %-30s %s\n", "exec <appid> <action> [params]", "Execute game action (e.g., exec 440 craft-metal)")
	fmt.Printf("  %-30s %s\n", "exec <appid> inventory", "Quick shortcut to query game backpack items")
	fmt.Println("\nGlobal Parameters:")
	fmt.Println("  Arguments for 'exec' actions can be passed in key=value format (e.g., type=scrap).")
}

func GetIPCConnection(ctx context.Context) (*grpc.ClientConn, error) {
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
		} else if home, err := os.UserHomeDir(); err == nil && home != "" {
			addr = filepath.Join(home, ".config", "gman", "gman.sock")
		} else {
			addr = "gman.sock"
		}
	}

	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, target string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, netType, target)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial daemon: %w", err)
	}

	return conn, nil
}

func handleStatus(ctx context.Context, client pb.DaemonServiceClient) {
	resp, err := client.GetStatus(ctx, &pb.GetStatusRequest{})
	if err != nil {
		fmt.Printf("%sFailed to get status: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	fmt.Printf("%s%s=== G-MAN DAEMON STATUS ===%s\n", ColorBold, ColorCyan, ColorReset)

	var connStr string
	if resp.GetConnected() {
		connStr = fmt.Sprintf("%sONLINE [Steam]%s", ColorGreen, ColorReset)
	} else {
		connStr = fmt.Sprintf("%sOFFLINE [Steam]%s", ColorRed, ColorReset)
	}

	fmt.Printf("Connection State: %s\n", connStr)

	steamID := resp.GetSteamId()
	if steamID == "" || steamID == "0" {
		steamID = fmt.Sprintf("%sNot logged in%s", ColorGray, ColorReset)
	}

	fmt.Printf("Steam ID:         %s\n", steamID)

	gameStr := fmt.Sprintf("%sNone%s", ColorGray, ColorReset)
	if resp.GetCurrentAppid() != 0 {
		gameStr = fmt.Sprintf("%s%d (%s)%s", ColorGreen, resp.GetCurrentAppid(), resp.GetCurrentAppName(), ColorReset)
	}

	fmt.Printf("Active Game:      %s\n", gameStr)

	fmt.Printf("Daemon Uptime:    %s%s%s\n", ColorYellow, resp.GetUptime(), ColorReset)

	memMB := float64(resp.GetMemoryBytes()) / 1024.0 / 1024.0
	fmt.Printf("Memory Usage:     %s%.2f MB%s\n", ColorYellow, memMB, ColorReset)
}

func handleStop(ctx context.Context, client pb.DaemonServiceClient) {
	fmt.Printf("%sStopping g-man daemon...%s\n", ColorYellow, ColorReset)

	resp, err := client.StopDaemon(ctx, &pb.StopDaemonRequest{})
	if err != nil {
		fmt.Printf("%sFailed to stop daemon: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	fmt.Printf("%s%s%s\n", ColorGreen, resp.GetMessage(), ColorReset)
}

func handleFreeMemory(ctx context.Context, client pb.DaemonServiceClient) {
	fmt.Printf("%sTriggering manual Garbage Collection in daemon...%s\n", ColorYellow, ColorReset)

	resp, err := client.FreeMemory(ctx, &pb.FreeMemoryRequest{})
	if err != nil {
		fmt.Printf("%sFailed to trigger GC: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	fmt.Printf("%s%s%s\n", ColorGreen, resp.GetMessage(), ColorReset)
	memMB := float64(resp.GetMemoryBytes()) / 1024.0 / 1024.0
	fmt.Printf("New Memory Usage: %s%.2f MB%s\n", ColorYellow, memMB, ColorReset)
}

func handlePlay(ctx context.Context, client pb.DaemonServiceClient, appID uint32) {
	fmt.Printf("%sLaunching session for game AppID %d...\n", ColorCyan, appID)

	resp, err := client.PlayGame(ctx, &pb.PlayGameRequest{Appid: appID})
	if err != nil {
		fmt.Printf("%sFailed to play game: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	fmt.Printf("%s%s%s\n", ColorGreen, resp.GetMessage(), ColorReset)
}

func handleExitGame(ctx context.Context, client pb.DaemonServiceClient) {
	fmt.Printf("%sStopping playing session and exiting game...%s\n", ColorCyan, ColorReset)

	resp, err := client.ExitGame(ctx, &pb.ExitGameRequest{})
	if err != nil {
		fmt.Printf("%sFailed to exit game: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	fmt.Printf("%s%s%s\n", ColorGreen, resp.GetMessage(), ColorReset)
}

func handleExec(
	ctx context.Context,
	client pb.DaemonServiceClient,
	appID uint32,
	action string,
	params map[string]string,
) {
	fmt.Printf(
		"%sExecuting action %s%q%s on game AppID %s%d%s...\n",
		ColorCyan,
		ColorBold,
		action,
		ColorReset,
		ColorCyan,
		appID,
		ColorReset,
	)

	resp, err := client.ExecAction(ctx, &pb.ExecActionRequest{
		Appid:  appID,
		Action: action,
		Params: params,
	})
	if err != nil {
		fmt.Printf("%sAction failed: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	fmt.Printf("\n%s%sAction completed! Result message:%s\n", ColorBold, ColorGreen, ColorReset)
	fmt.Println(resp.GetMessage())

	if resp.GetDetails() != "" {
		fmt.Println(resp.GetDetails())
	}
}
