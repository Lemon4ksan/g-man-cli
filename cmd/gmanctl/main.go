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

	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
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

	var (
		ctx    context.Context
		cancel context.CancelFunc
	)

	if command == "events" {
		ctx, cancel = context.WithCancel(context.Background())
	} else {
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	}

	defer cancel()

	dialCtx, dialCancel := context.WithTimeout(context.Background(), 10*time.Second)
	conn, err := GetIPCConnection(dialCtx)

	dialCancel()

	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError connecting to daemon: %v%s\n", ColorRed, err, ColorReset)
		fmt.Fprintf(os.Stderr, "%sIs the daemon 'g-mand' running?%s\n", ColorYellow, ColorReset)

		exitCode = 1

		return
	}

	defer conn.Close()

	client := pb.NewDaemonServiceClient(conn)

	switch command {
	case "status":
		handleStatus(ctx, client)
	case "stop":
		handleStop(ctx, client)
	case "gc":
		handleFreeMemory(ctx, client)
	case "events":
		handleStreamEvents(ctx, client)
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

	case "update-prices":
		if len(os.Args) < 3 {
			fmt.Printf(
				"%sError: 'update-prices' command requires at least one price entry. Example: gmanctl update-prices \"5021;6=1,0,1,0.11\"%s\n",
				ColorRed,
				ColorReset,
			)

			exitCode = 1

			return
		}

		prices := make(map[string]*pb.ManualPriceEntry)
		for _, arg := range os.Args[2:] {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) != 2 {
				fmt.Printf(
					"%sError: Invalid price format %q. Expected: sku=buy_keys,buy_metal,sell_keys,sell_metal%s\n",
					ColorRed,
					arg,
					ColorReset,
				)

				exitCode = 1

				return
			}

			sku := parts[0]

			priceVals := strings.Split(parts[1], ",")
			if len(priceVals) != 4 {
				fmt.Printf(
					"%sError: Invalid price values %q. Expected 4 comma-separated values (buy_keys,buy_metal,sell_keys,sell_metal)%s\n",
					ColorRed,
					parts[1],
					ColorReset,
				)

				exitCode = 1

				return
			}

			buyKeys, err := strconv.ParseUint(priceVals[0], 10, 32)
			if err != nil {
				fmt.Printf("%sError: Invalid buy_keys %q: %v%s\n", ColorRed, priceVals[0], err, ColorReset)

				exitCode = 1

				return
			}

			buyMetal, err := strconv.ParseFloat(priceVals[1], 64)
			if err != nil {
				fmt.Printf("%sError: Invalid buy_metal %q: %v%s\n", ColorRed, priceVals[1], err, ColorReset)

				exitCode = 1

				return
			}

			sellKeys, err := strconv.ParseUint(priceVals[2], 10, 32)
			if err != nil {
				fmt.Printf("%sError: Invalid sell_keys %q: %v%s\n", ColorRed, priceVals[2], err, ColorReset)

				exitCode = 1

				return
			}

			sellMetal, err := strconv.ParseFloat(priceVals[3], 64)
			if err != nil {
				fmt.Printf("%sError: Invalid sell_metal %q: %v%s\n", ColorRed, priceVals[3], err, ColorReset)

				exitCode = 1

				return
			}

			prices[sku] = &pb.ManualPriceEntry{
				BuyKeys:   uint32(buyKeys),
				BuyMetal:  buyMetal,
				SellKeys:  uint32(sellKeys),
				SellMetal: sellMetal,
			}
		}

		handleUpdatePrices(ctx, client, prices)

	case "guard":
		if len(os.Args) < 3 {
			fmt.Printf(
				"%sError: 'guard' command requires a subcommand (status, code, list, respond, import)%s\n",
				ColorRed,
				ColorReset,
			)

			exitCode = 1

			return
		}

		sub := os.Args[2]
		switch sub {
		case "status":
			handleGuardStatus(ctx, client)
		case "code":
			handleGuardCode(ctx, client)
		case "list":
			handleGuardList(ctx, client)
		case "respond":
			if len(os.Args) < 5 {
				fmt.Printf(
					"%sError: 'guard respond' requires <id> <accept|decline>. Example: gmanctl guard respond 12345 accept%s\n",
					ColorRed,
					ColorReset,
				)

				exitCode = 1

				return
			}

			confID, err := strconv.ParseUint(os.Args[3], 10, 64)
			if err != nil {
				fmt.Printf("%sError: Invalid confirmation ID %q: %v%s\n", ColorRed, os.Args[3], err, ColorReset)

				exitCode = 1

				return
			}

			accept := os.Args[4] == "accept" || os.Args[4] == "true" || os.Args[4] == "1"
			handleGuardRespond(ctx, client, confID, accept)

		case "import":
			if len(os.Args) < 6 {
				fmt.Printf(
					"%sError: 'guard import' requires <shared_secret> <identity_secret> <device_id> [account_name]%s\n",
					ColorRed,
					ColorReset,
				)

				exitCode = 1

				return
			}

			accountName := ""
			if len(os.Args) >= 6 {
				accountName = os.Args[5]
			}

			handleGuardImport(ctx, client, os.Args[3], os.Args[4], accountName)

		default:
			fmt.Printf("%sUnknown guard subcommand: %s%s\n", ColorRed, sub, ColorReset)

			exitCode = 1
		}

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
	fmt.Printf("  %-30s %s\n", "events", "Stream real-time daemon and game coordinator events")
	fmt.Println("\nGame Commands:")
	fmt.Printf("  %-30s %s\n", "play <appid>", "Launch game session & initialize Game Coordinator")
	fmt.Printf("  %-30s %s\n", "exit-game", "Close active game session, return to simple online mode")
	fmt.Println("\nAction Commands:")
	fmt.Printf("  %-30s %s\n", "exec <appid> <action> [params]", "Execute game action (e.g., exec 440 craft-metal)")
	fmt.Printf("  %-30s %s\n", "exec <appid> inventory", "Quick shortcut to query game backpack items")
	fmt.Printf("  %-30s %s\n", "update-prices <entry> [entry...]", "Update manual pricing database entries")
	fmt.Println("\nGuard Commands:")
	fmt.Printf("  %-30s %s\n", "guard status", "Show Steam Guard configuration status")
	fmt.Printf("  %-30s %s\n", "guard code", "Generate current Steam Guard 2FA TOTP code")
	fmt.Printf("  %-30s %s\n", "guard list", "List pending Steam Guard confirmations")
	fmt.Printf("  %-30s %s\n", "guard respond <id> <accept|decline>", "Accept or decline a confirmation")
	fmt.Printf("  %-30s %s\n", "guard import <shared> <identity> <device>", "Import Steam Guard secrets")
	fmt.Println("\nGlobal Parameters:")
	fmt.Println("  Arguments for 'exec' actions can be passed in key=value format (e.g., type=scrap).")
	fmt.Println("  Arguments for 'update-prices' are formatted as: sku=buy_keys,buy_metal,sell_keys,sell_metal")
	fmt.Println("  Example: gmanctl update-prices \"5021;6=1,0,1,0.11\"")
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

func handleStreamEvents(ctx context.Context, client pb.DaemonServiceClient) {
	fmt.Printf("%sStreaming real-time daemon events... (Press Ctrl+C to exit)%s\n\n", ColorCyan, ColorReset)

	stream, err := client.StreamEvents(ctx, &pb.StreamEventsRequest{})
	if err != nil {
		fmt.Printf("%sFailed to start event stream: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	for {
		resp, err := stream.Recv()
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			fmt.Printf("%sStream connection lost: %v%s\n", ColorRed, err, ColorReset)

			return
		}

		t := time.Unix(resp.GetTimestamp(), 0).Format("15:04:05")

		evType := resp.GetEventType()
		if idx := strings.LastIndex(evType, "."); idx != -1 {
			evType = evType[idx+1:]
		}

		evType = strings.TrimPrefix(evType, "*")

		fmt.Printf("[%s] %s%s%s: %s\n",
			t,
			ColorGreen,
			evType,
			ColorReset,
			resp.GetPayloadJson(),
		)
	}
}

func handleUpdatePrices(ctx context.Context, client pb.DaemonServiceClient, prices map[string]*pb.ManualPriceEntry) {
	fmt.Printf("%sSending price updates to daemon...%s\n", ColorCyan, ColorReset)

	resp, err := client.UpdateManualPrices(ctx, &pb.UpdateManualPricesRequest{
		Prices: prices,
	})
	if err != nil {
		fmt.Printf("%sFailed to update prices: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	if resp.GetSuccess() {
		fmt.Printf("%s%s%s\n", ColorGreen, resp.GetMessage(), ColorReset)
	} else {
		fmt.Printf("%sFailed: %s%s\n", ColorRed, resp.GetMessage(), ColorReset)
	}
}

func handleGuardStatus(ctx context.Context, client pb.DaemonServiceClient) {
	resp, err := client.GuardStatus(ctx, &pb.GuardStatusRequest{})
	if err != nil {
		fmt.Printf("%sFailed to get guard status: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	fmt.Printf("%s%s=== STEAM GUARD STATUS ===%s\n", ColorBold, ColorCyan, ColorReset)

	if resp.GetConfigured() {
		fmt.Printf("Status:    %sConfigured%s\n", ColorGreen, ColorReset)
	} else {
		fmt.Printf("Status:    %sNot configured%s\n", ColorRed, ColorReset)
	}

	if resp.GetDeviceId() != "" {
		fmt.Printf("Device ID: %s%s%s\n", ColorYellow, resp.GetDeviceId(), ColorReset)
	}

	if resp.GetSteamId() != "" {
		fmt.Printf("Steam ID:  %s%s%s\n", ColorYellow, resp.GetSteamId(), ColorReset)
	}
}

func handleGuardCode(ctx context.Context, client pb.DaemonServiceClient) {
	resp, err := client.GuardCode(ctx, &pb.GuardCodeRequest{})
	if err != nil {
		fmt.Printf("%sFailed to generate guard code: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	fmt.Printf("%s%sCurrent Steam Guard Code: %s%s\n", ColorBold, ColorGreen, resp.GetCode(), ColorReset)
}

func handleGuardList(ctx context.Context, client pb.DaemonServiceClient) {
	fmt.Printf("%sFetching pending confirmations...%s\n", ColorCyan, ColorReset)

	resp, err := client.GuardList(ctx, &pb.GuardListRequest{})
	if err != nil {
		fmt.Printf("%sFailed to list confirmations: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	confs := resp.GetConfirmations()
	if len(confs) == 0 {
		fmt.Printf("%sNo pending confirmations%s\n", ColorGray, ColorReset)
		return
	}

	fmt.Printf("%s%s=== PENDING CONFIRMATIONS (%d) ===%s\n", ColorBold, ColorCyan, len(confs), ColorReset)

	for i, c := range confs {
		fmt.Printf("\n%s[%d]%s %s%s%s\n", ColorBold, i+1, ColorReset, ColorGreen, c.GetTitle(), ColorReset)
		fmt.Printf(
			"    ID: %s%d%s  Type: %s%s%s\n",
			ColorYellow,
			c.GetId(),
			ColorReset,
			ColorYellow,
			c.GetTypeName(),
			ColorReset,
		)

		if c.GetDescription() != "" {
			fmt.Printf("    %s%s%s\n", ColorGray, c.GetDescription(), ColorReset)
		}
	}

	fmt.Printf("\n%sTo accept: gmanctl guard respond <id> accept%s\n", ColorGray, ColorReset)
	fmt.Printf("%sTo decline: gmanctl guard respond <id> decline%s\n", ColorGray, ColorReset)
}

func handleGuardRespond(ctx context.Context, client pb.DaemonServiceClient, confID uint64, accept bool) {
	action := "declining"
	if accept {
		action = "accepting"
	}

	fmt.Printf("%s%s confirmation %d...%s\n", ColorCyan, action, confID, ColorReset)

	resp, err := client.GuardRespond(ctx, &pb.GuardRespondRequest{
		ConfirmationId: confID,
		Accept:         accept,
	})
	if err != nil {
		fmt.Printf("%sFailed to respond to confirmation: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	if resp.GetSuccess() {
		fmt.Printf("%s%s%s\n", ColorGreen, resp.GetMessage(), ColorReset)
	} else {
		fmt.Printf("%sFailed: %s%s\n", ColorRed, resp.GetMessage(), ColorReset)
	}
}

func handleGuardImport(
	ctx context.Context,
	client pb.DaemonServiceClient,
	sharedSecret, identitySecret, accountName string,
) {
	fmt.Printf("%sImporting Steam Guard secrets...%s\n", ColorCyan, ColorReset)

	resp, err := client.GuardImport(ctx, &pb.GuardImportRequest{
		SharedSecret:   sharedSecret,
		IdentitySecret: identitySecret,
		AccountName:    accountName,
	})
	if err != nil {
		fmt.Printf("%sFailed to import guard secrets: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	if resp.GetSuccess() {
		fmt.Printf("%s%s%s\n", ColorGreen, resp.GetMessage(), ColorReset)
	} else {
		fmt.Printf("%sFailed: %s%s\n", ColorRed, resp.GetMessage(), ColorReset)
	}
}
