// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package main

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/lemon4ksan/miyako/generic"
	"github.com/skip2/go-qrcode"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/lemon4ksan/g-man-cli/pkg/guard/crypto"
	"github.com/lemon4ksan/g-man-cli/pkg/guard/driver"
	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
)

// Sentinel errors for Steam Guard CLI management.
var (
	ErrMissingAuthCode         = errors.New("missing authentication code")
	ErrMissingConfirmationID   = errors.New("missing confirmation ID")
	ErrMissingMaFilePath       = errors.New("missing maFile path")
	ErrInvalidMaFileSecrets    = errors.New("invalid maFile: missing shared_secret or identity_secret")
	ErrNotConfiguredGuard      = errors.New("steam guard is not configured: link or transfer authenticator first")
	ErrPassphrasesDoNotMatch   = errors.New("passphrases do not match")
	ErrEmptyPassphrase         = errors.New("passphrase cannot be empty")
	ErrMissingFilePathEncrypt  = errors.New("missing file path to encrypt")
	ErrMissingEncryptedPathDec = errors.New("missing encrypted file path")
)

func printGuardUsage() {
	gd := driver.New(nil)

	for line := range strings.SplitSeq(gd.Usage(), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			fmt.Println()
			continue
		}

		if strings.HasSuffix(trimmed, ":") {
			fmt.Printf("%s%s%s%s\n", ColorCyan, ColorBold, line, ColorReset)
			continue
		}

		if strings.HasPrefix(trimmed, "-") {
			fmt.Println(line)
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))
		fmt.Printf("%s%s%s\n", line[:indent], "guard ", trimmed)
	}
}

func handleGuardCommand(ctx context.Context, client pb.DaemonServiceClient, subcmd string, args []string) error {
	switch subcmd {
	case "help":
		printGuardUsage()
		return nil
	case "auth":
		return handleGuardAuth(ctx, client, args)
	case "status":
		return handleGuardStatusCmd(ctx, client)
	case "code":
		return handleGuardCodeCmd(ctx, client)
	case "list":
		return handleGuardListCmd(ctx, client)
	case "accept", "approve":
		return handleGuardRespondCmd(ctx, client, args, true)
	case "cancel":
		return handleGuardRespondCmd(ctx, client, args, false)
	case "accept-all":
		return handleGuardRespondAllCmd(ctx, client, true)
	case "cancel-all":
		return handleGuardRespondAllCmd(ctx, client, false)
	case "transfer":
		return handleGuardTransfer(ctx, client)
	case "link":
		return handleGuardLink(ctx, client)
	case "import":
		return handleGuardImportCmd(ctx, client, args)
	case "qr":
		return handleGuardQR(ctx, client, args)
	case "encrypt":
		return handleGuardEncryptFile(args)
	case "decrypt":
		return handleGuardDecryptFile(args)
	case "unlock":
		return handleGuardUnlockDaemon(ctx, client)
	case "encrypt-env":
		return handleGuardEncryptEnv()
	case "decrypt-env":
		return handleGuardDecryptEnv()
	default:
		return fmt.Errorf("unknown guard subcommand: %s", subcmd)
	}
}

func handleGuardAuth(ctx context.Context, client pb.DaemonServiceClient, args []string) error {
	if len(args) < 1 {
		return ErrMissingAuthCode
	}

	resp, err := client.GuardSubmitAuthCode(ctx, &pb.GuardSubmitAuthCodeRequest{Code: args[0]})
	if err != nil {
		return err
	}

	fmt.Println(resp.GetMessage())

	return nil
}

func handleGuardStatusCmd(ctx context.Context, client pb.DaemonServiceClient) error {
	resp, err := client.GuardStatus(ctx, &pb.GuardStatusRequest{})
	if err != nil {
		return err
	}

	fmt.Printf("=== Steam Guard Status ===\nConfigured: %v\nDevice ID:  %s\nSteam ID:   %s\nState:      %d\n",
		resp.GetConfigured(), resp.GetDeviceId(), resp.GetSteamId(), resp.GetState())

	return nil
}

func handleGuardCodeCmd(ctx context.Context, client pb.DaemonServiceClient) error {
	resp, err := client.GuardCode(ctx, &pb.GuardCodeRequest{})
	if err != nil {
		return err
	}

	fmt.Printf("Steam Guard Code: %s%s%s\n", ColorGreen, resp.GetCode(), ColorReset)

	return nil
}

func handleGuardListCmd(ctx context.Context, client pb.DaemonServiceClient) error {
	resp, err := client.GuardList(ctx, &pb.GuardListRequest{})
	if err != nil {
		return err
	}

	fmt.Printf("=== Pending Confirmations (%d) ===\n", len(resp.GetConfirmations()))

	for _, conf := range resp.GetConfirmations() {
		fmt.Printf("[%d] %s (Type: %s, Description: %s)\n",
			conf.GetId(), conf.GetTitle(), conf.GetTypeName(), conf.GetDescription())
	}

	return nil
}

func handleGuardRespondCmd(ctx context.Context, client pb.DaemonServiceClient, args []string, accept bool) error {
	if len(args) < 1 {
		return ErrMissingConfirmationID
	}

	confID, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid confirmation ID: %w", err)
	}

	resp, err := client.GuardRespond(ctx, &pb.GuardRespondRequest{
		ConfirmationId: confID,
		Accept:         accept,
	})
	if err != nil {
		return err
	}

	if !resp.GetSuccess() {
		return errors.New(resp.GetMessage())
	}

	actionStr := "approved"
	if !accept {
		actionStr = "cancelled"
	}

	fmt.Printf("Confirmation %s successfully.\n", actionStr)

	return nil
}

func handleGuardRespondAllCmd(ctx context.Context, client pb.DaemonServiceClient, accept bool) error {
	resp, err := client.GuardRespond(ctx, &pb.GuardRespondRequest{
		All:    true,
		Accept: accept,
	})
	if err != nil {
		return err
	}

	if !resp.GetSuccess() {
		return errors.New(resp.GetMessage())
	}

	actionStr := "approved"
	if !accept {
		actionStr = "cancelled"
	}

	fmt.Printf("All confirmations %s successfully.\n", actionStr)

	return nil
}

func handleGuardTransfer(ctx context.Context, client pb.DaemonServiceClient) error {
	fmt.Println(
		"Beginning transfer of mobile authenticator...\nWarning: You can't transfer an authenticator without a phone number on the account.",
	)

	resp, err := client.GuardTransferStart(ctx, &pb.GuardTransferStartRequest{})
	if err != nil {
		return err
	}

	fmt.Println(resp.GetMessage())

	var smsCode string

	fmt.Print("Enter SMS verification code: ")

	if _, err := fmt.Scanln(&smsCode); err != nil {
		return fmt.Errorf("failed to read SMS code: %w", err)
	}

	finishResp, err := client.GuardTransferFinish(ctx, &pb.GuardTransferFinishRequest{
		SmsCode: strings.TrimSpace(smsCode),
	})
	if err != nil {
		return err
	}

	fmt.Println("\nTransfer completed successfully!")
	fmt.Println("==========================================================")
	fmt.Printf(
		"Shared Secret:    %s\nIdentity Secret:  %s\nRevocation Code:  %s\nDevice ID:        %s\nURI:              %s\n",
		finishResp.GetSharedSecret(),
		finishResp.GetIdentitySecret(),
		finishResp.GetRevocationCode(),
		finishResp.GetDeviceId(),
		finishResp.GetUri(),
	)
	fmt.Println("==========================================================")

	return nil
}

func handleGuardLink(ctx context.Context, client pb.DaemonServiceClient) error {
	var deviceID string

	fmt.Print("Enter Device ID (leave empty to generate): ")

	_, _ = fmt.Scanln(&deviceID)

	startResp, err := client.GuardLinkStart(ctx, &pb.GuardLinkStartRequest{DeviceId: strings.TrimSpace(deviceID)})
	if err != nil {
		return err
	}

	fmt.Printf(
		"Successfully registered with Steam.\nPhoneNumberHint: %s\nRevocation Code: %s\n",
		startResp.GetPhoneNumberHint(),
		startResp.GetRevocationCode(),
	)

	var smsCode string

	fmt.Print("Enter verification code (SMS/Email): ")

	if _, err := fmt.Scanln(&smsCode); err != nil {
		return fmt.Errorf("failed to read SMS code: %w", err)
	}

	finalizeResp, err := client.GuardLinkFinalize(ctx, &pb.GuardLinkFinalizeRequest{
		SharedSecret:   startResp.GetSharedSecret(),
		ServerTime:     startResp.GetServerTime(),
		SmsCode:        strings.TrimSpace(smsCode),
		IdentitySecret: startResp.GetIdentitySecret(),
		DeviceId:       startResp.GetDeviceId(),
	})
	if err != nil {
		return err
	}

	fmt.Println(finalizeResp.GetMessage())

	return nil
}

func handleGuardImportCmd(ctx context.Context, client pb.DaemonServiceClient, args []string) error {
	if len(args) < 1 {
		return ErrMissingMaFilePath
	}

	type maFile struct {
		SharedSecret   string `json:"shared_secret"`
		IdentitySecret string `json:"identity_secret"`
		DeviceID       string `json:"device_id"`
		RevocationCode string `json:"revocation_code"`
		AccountName    string `json:"account_name"`
		SteamID        string `json:"steam_id"`
		Tokens         struct {
			RefreshToken string `json:"refresh_token"`
		} `json:"tokens"`
		Session struct {
			SteamID string `json:"SteamID"`
		} `json:"Session"`
	}

	fileData, err := readImportFile(args[0])
	if err != nil {
		return err
	}

	var ma maFile
	if err := json.Unmarshal(fileData, &ma); err != nil {
		return fmt.Errorf("failed to parse maFile JSON: %w", err)
	}

	if ma.SharedSecret == "" || ma.IdentitySecret == "" {
		return ErrInvalidMaFileSecrets
	}

	steamID := generic.Coalesce(ma.SteamID, ma.Session.SteamID)
	devID := getOrGenerateDeviceID(ma.DeviceID, steamID, ma.Tokens.RefreshToken)

	resp, err := client.GuardImport(ctx, &pb.GuardImportRequest{
		SharedSecret:   ma.SharedSecret,
		IdentitySecret: ma.IdentitySecret,
		DeviceId:       devID,
		AccountName:    ma.AccountName,
		RefreshToken:   ma.Tokens.RefreshToken,
	})
	if err != nil {
		if status.Code(err) == codes.Unavailable {
			fmt.Println("Daemon is not running. Performing offline import directly to .env...")
			return handleGuardImportOffline(args)
		}

		return err
	}

	if !resp.GetSuccess() {
		return errors.New(resp.GetMessage())
	}

	fmt.Println("==========================================================")
	fmt.Println("Steam Guard credentials imported successfully!")
	fmt.Printf("Device ID:  %s\n", ma.DeviceID)

	if ma.RevocationCode != "" {
		fmt.Printf("Revocation: %s\n", ma.RevocationCode)
	}

	fmt.Println("==========================================================")

	return nil
}

func handleGuardQR(ctx context.Context, client pb.DaemonServiceClient, args []string) error {
	format := "steam"
	ascii := false

	for _, arg := range args {
		if arg == "--ascii" {
			ascii = true
		} else if after, ok := strings.CutPrefix(arg, "--format="); ok {
			format = after
		}
	}

	statusResp, err := client.GuardStatus(ctx, &pb.GuardStatusRequest{})
	if err != nil {
		return err
	}

	if !statusResp.GetConfigured() {
		return ErrNotConfiguredGuard
	}

	rawSecret, err := base64.StdEncoding.DecodeString(statusResp.GetSharedSecret())
	if err != nil {
		rawSecret, err = hex.DecodeString(statusResp.GetSharedSecret())
		if err != nil {
			return fmt.Errorf("failed to decode shared secret: %w", err)
		}
	}

	secretB64 := base64.StdEncoding.EncodeToString(rawSecret)
	secretB32 := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(rawSecret)
	username := generic.Coalesce(statusResp.GetAccountName(), "account")

	var qrContent string
	switch format {
	case "steam":
		qrContent = fmt.Sprintf("otpauth://totp/Steam:%s?secret=%s&issuer=Steam", url.PathEscape(username), secretB32)
	case "bitwarden":
		qrContent = "steam://" + secretB64
	case "keepassxc":
		qrContent = fmt.Sprintf(
			"otpauth://totp/Steam:%s?secret=%s&period=30&digits=5&issuer=Steam&encoder=steam",
			url.PathEscape(username),
			secretB64,
		)

	default:
		return fmt.Errorf("unknown QR format: %s", format)
	}

	qr, err := qrcode.New(qrContent, qrcode.Medium)
	if err != nil {
		return fmt.Errorf("failed to generate QR: %w", err)
	}

	if ascii {
		for _, row := range qr.Bitmap() {
			for _, val := range row {
				if val {
					fmt.Print("##")
				} else {
					fmt.Print("  ")
				}
			}

			fmt.Println()
		}
	} else {
		fmt.Println(qr.ToSmallString(false))
	}

	return nil
}

func handleGuardEncryptFile(args []string) error {
	if len(args) < 1 {
		return ErrMissingFilePathEncrypt
	}

	filePath := args[0]

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	passphrase, err := readPassphrase("Enter passphrase: ")
	if err != nil {
		return fmt.Errorf("failed to read passphrase: %w", err)
	}

	confirm, err := readPassphrase("Confirm passphrase: ")
	if err != nil {
		return fmt.Errorf("failed to read confirmation: %w", err)
	}

	if passphrase != confirm {
		return ErrPassphrasesDoNotMatch
	}

	if len(passphrase) == 0 {
		return ErrEmptyPassphrase
	}

	encrypted, err := crypto.EncryptData(fileData, passphrase)
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}

	outPath := filePath + ".enc"
	if len(args) > 1 {
		outPath = args[1]
	}

	if err := os.WriteFile(outPath, encrypted, 0o600); err != nil {
		return fmt.Errorf("failed to write encrypted file: %w", err)
	}

	fmt.Printf("File encrypted successfully and saved to: %s\n", outPath)

	return nil
}

func handleGuardDecryptFile(args []string) error {
	if len(args) < 1 {
		return ErrMissingEncryptedPathDec
	}

	filePath := args[0]

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	passphrase, err := readPassphrase("Enter passphrase: ")
	if err != nil {
		return fmt.Errorf("failed to read passphrase: %w", err)
	}

	decrypted, err := crypto.DecryptData(fileData, passphrase)
	if err != nil {
		return err
	}

	outPath := ""
	if len(args) > 1 {
		outPath = args[1]
	} else if before, ok := strings.CutSuffix(filePath, ".enc"); ok {
		outPath = before
	} else {
		outPath = filePath + ".dec"
	}

	if err := os.WriteFile(outPath, decrypted, 0o600); err != nil {
		return fmt.Errorf("failed to write decrypted file: %w", err)
	}

	fmt.Printf("File decrypted successfully and saved to: %s\n", outPath)

	return nil
}

func handleGuardUnlockDaemon(ctx context.Context, client pb.DaemonServiceClient) error {
	passphrase, err := readPassphrase("Enter passphrase to unlock daemon: ")
	if err != nil {
		return fmt.Errorf("failed to read passphrase: %w", err)
	}

	resp, err := client.GuardUnlock(ctx, &pb.GuardUnlockRequest{Passphrase: passphrase})
	if err != nil {
		return fmt.Errorf("unlock call failed: %w", err)
	}

	if !resp.GetSuccess() {
		return fmt.Errorf("failed to unlock: %s", resp.GetMessage())
	}

	fmt.Println(resp.GetMessage())

	return nil
}

func handleGuardEncryptEnv() error {
	passphrase, err := readPassphrase("Enter passphrase to encrypt .env: ")
	if err != nil {
		return fmt.Errorf("failed to read passphrase: %w", err)
	}

	confirm, err := readPassphrase("Confirm passphrase: ")
	if err != nil {
		return fmt.Errorf("failed to read confirmation: %w", err)
	}

	if passphrase != confirm {
		return ErrPassphrasesDoNotMatch
	}

	if len(passphrase) == 0 {
		return ErrEmptyPassphrase
	}

	return encryptEnvFile(passphrase)
}

func handleGuardDecryptEnv() error {
	passphrase, err := readPassphrase("Enter passphrase to decrypt .env: ")
	if err != nil {
		return fmt.Errorf("failed to read passphrase: %w", err)
	}

	return decryptEnvFile(passphrase)
}

func readImportFile(filePath string) ([]byte, error) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	if len(fileData) >= 8 && string(fileData[:8]) == crypto.CryptoMagic {
		fmt.Println("File is encrypted. Please enter the passphrase to decrypt it.")

		passphrase, err := readPassphrase("Passphrase: ")
		if err != nil {
			return nil, fmt.Errorf("failed to read passphrase: %w", err)
		}

		return crypto.DecryptData(fileData, passphrase)
	}

	return fileData, nil
}

func handleGuardImportOffline(args []string) error {
	if len(args) < 1 {
		return ErrMissingMaFilePath
	}

	type maFile struct {
		SharedSecret   string `json:"shared_secret"`
		IdentitySecret string `json:"identity_secret"`
		DeviceID       string `json:"device_id"`
		RevocationCode string `json:"revocation_code"`
		AccountName    string `json:"account_name"`
		Tokens         struct {
			RefreshToken string `json:"refresh_token"`
		} `json:"tokens"`
		Session struct {
			SteamID string `json:"SteamID"`
		} `json:"Session"`
	}

	fileData, err := readImportFile(args[0])
	if err != nil {
		return err
	}

	var ma maFile
	if err := json.Unmarshal(fileData, &ma); err != nil {
		return fmt.Errorf("failed to parse maFile JSON: %w", err)
	}

	if ma.SharedSecret == "" || ma.IdentitySecret == "" {
		return ErrInvalidMaFileSecrets
	}

	devID := getOrGenerateDeviceID(ma.DeviceID, ma.Session.SteamID, ma.Tokens.RefreshToken)
	updateLocalEnvFile(ma.SharedSecret, ma.IdentitySecret, devID, ma.AccountName, ma.Tokens.RefreshToken)

	fmt.Println("==========================================================")
	fmt.Println("Offline Steam Guard credentials imported successfully!")
	fmt.Println("New secrets have been saved to your .env file.")
	fmt.Printf("Device ID:  %s\n", devID)

	if ma.RevocationCode != "" {
		fmt.Printf("Revocation: %s\n", ma.RevocationCode)
	}

	fmt.Println("==========================================================")

	return nil
}

func updateLocalEnvFile(sharedSecret, identitySecret, deviceID, accountName, refreshToken string) {
	envPath := ".env"
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		if err := os.WriteFile(envPath, []byte(""), 0o600); err != nil {
			fmt.Printf("Warning: file %q not found and failed to create one: %v\n", envPath, err)
			return
		}
	}

	content, err := os.ReadFile(envPath)
	if err != nil {
		fmt.Printf("Error reading .env file: %v\n", err)
		return
	}

	lines := strings.Split(string(content), "\n")
	keys := map[string]string{
		"STEAM_SHARED_SECRET":   sharedSecret,
		"STEAM_IDENTITY_SECRET": identitySecret,
		"STEAM_DEVICE_ID":       deviceID,
	}

	if accountName != "" {
		keys["STEAM_USER"] = accountName
	}

	if refreshToken != "" {
		keys["STEAM_REFRESH_TOKEN"] = refreshToken
	}

	updatedKeys := make(map[string]bool)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) > 0 {
			k := strings.TrimSpace(parts[0])
			if val, ok := keys[k]; ok {
				lines[i] = fmt.Sprintf("%s=%s", k, val)
				updatedKeys[k] = true
			}
		}
	}

	var toAppend []string
	for k, val := range keys {
		if !updatedKeys[k] {
			toAppend = append(toAppend, fmt.Sprintf("%s=%s", k, val))
		}
	}

	if len(toAppend) > 0 {
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}

		lines = append(lines, toAppend...)
	}

	if err := os.WriteFile(envPath, []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		fmt.Printf("Error writing to .env file: %v\n", err)
	} else {
		fmt.Println("Successfully updated local .env file.")
	}
}

func encryptEnvFile(passphrase string) error {
	envPath := ".env"

	content, err := os.ReadFile(envPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	keysToEncrypt := map[string]bool{
		"STEAM_PASS":            true,
		"STEAM_SHARED_SECRET":   true,
		"STEAM_IDENTITY_SECRET": true,
		"STEAM_REFRESH_TOKEN":   true,
	}

	modified := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])

			if keysToEncrypt[k] && v != "" && !crypto.IsEncryptedString(v) {
				enc, err := crypto.EncryptString(v, passphrase)
				if err != nil {
					return fmt.Errorf("failed to encrypt %s: %w", k, err)
				}

				lines[i] = fmt.Sprintf("%s=%s", k, enc)
				modified = true
			}
		}
	}

	if !modified {
		fmt.Println("No modifications needed (all sensitive values are already encrypted or empty).")
		return nil
	}

	if err := os.WriteFile(envPath, []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		return err
	}

	fmt.Println("Sensitive credentials in .env have been successfully encrypted.")

	return nil
}

func decryptEnvFile(passphrase string) error {
	envPath := ".env"

	content, err := os.ReadFile(envPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	modified := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])

			if crypto.IsEncryptedString(v) {
				dec, err := crypto.DecryptString(v, passphrase)
				if err != nil {
					return fmt.Errorf("failed to decrypt %s: %w", k, err)
				}

				lines[i] = fmt.Sprintf("%s=%s", k, dec)
				modified = true
			}
		}
	}

	if !modified {
		fmt.Println("No encrypted credentials found in .env.")
		return nil
	}

	if err := os.WriteFile(envPath, []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		return err
	}

	fmt.Println("Credentials in .env have been successfully decrypted.")

	return nil
}

func getOrGenerateDeviceID(deviceID, steamIDStr, refreshToken string) string {
	if deviceID != "" {
		return deviceID
	}

	var steamID uint64
	if steamIDStr != "" {
		if id, err := strconv.ParseUint(steamIDStr, 10, 64); err == nil && id > 0 {
			steamID = id
		}
	}

	if steamID == 0 && refreshToken != "" {
		parts := strings.Split(refreshToken, ".")
		if len(parts) == 3 {
			payloadStr := parts[1]
			if pad := len(payloadStr) % 4; pad != 0 {
				payloadStr += strings.Repeat("=", 4-pad)
			}

			payload, err := base64.URLEncoding.DecodeString(payloadStr)
			if err != nil {
				payload, _ = base64.RawURLEncoding.DecodeString(parts[1])
			}

			if len(payload) > 0 {
				var claims struct {
					Sub string `json:"sub"`
				}
				if err := json.Unmarshal(payload, &claims); err == nil {
					if id, err := strconv.ParseUint(claims.Sub, 10, 64); err == nil {
						steamID = id
					}
				}
			}
		}
	}

	if steamID > 0 {
		return crypto.GetDeviceID(steamID)
	}

	var r [16]byte

	_, _ = rand.Read(r[:])
	sum := hex.EncodeToString(r[:])

	return fmt.Sprintf("android:%s-%s-%s-%s-%s", sum[:8], sum[8:12], sum[12:16], sum[16:20], sum[20:32])
}
