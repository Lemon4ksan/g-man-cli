// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package crypto provides AES-256-GCM authenticated encryption, key derivation,
// and Steam Device ID formatting for secure storage of credentials.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// CryptoMagic is the magic header prefix for encrypted binary payloads.
const CryptoMagic = "GMANENC\x00"

// EncryptedPrefix is the prefix identifying encrypted values inside .env files.
const EncryptedPrefix = "GMANENC:"

// Sentinel cryptographic errors.
var (
	ErrDataTooShort           = errors.New("invalid or corrupted encrypted data: payload too short")
	ErrMissingMagicHeader     = errors.New("invalid encrypted data: missing GMANENC magic header")
	ErrDecryptionFailed       = errors.New("decryption failed: invalid passphrase or corrupted data")
	ErrInvalidEncryptedFormat = errors.New("invalid encrypted string format: must have 3 colon-separated parts")
	ErrInvalidSalt            = errors.New("invalid salt encoding or size")
	ErrInvalidNonce           = errors.New("invalid nonce encoding or size")
	ErrInvalidCiphertext      = errors.New("invalid ciphertext encoding")
)

// DeriveKey derives a 32-byte key from a passphrase and salt using 100,000 rounds of SHA-256 hashing.
func DeriveKey(passphrase string, salt []byte) []byte {
	key := []byte(passphrase)
	for range 100000 {
		h := sha256.New()
		h.Write(key)
		h.Write(salt)
		key = h.Sum(nil)
	}

	return key
}

// EncryptData encrypts plaintext bytes using AES-256-GCM.
// Output Format: CryptoMagic (8 bytes) + Salt (16 bytes) + Nonce (12 bytes) + Ciphertext + Tag.
func EncryptData(plaintext []byte, passphrase string) ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	key := DeriveKey(passphrase, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	out := make([]byte, 0, len(CryptoMagic)+len(salt)+len(nonce)+len(ciphertext))
	out = append(out, CryptoMagic...)
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)

	return out, nil
}

// DecryptData decrypts ciphertext bytes produced by [EncryptData] using AES-256-GCM.
func DecryptData(data []byte, passphrase string) ([]byte, error) {
	magicLen := len(CryptoMagic)
	minLen := magicLen + 16 + 12 + 16

	if len(data) < minLen {
		return nil, ErrDataTooShort
	}

	if string(data[:magicLen]) != CryptoMagic {
		return nil, ErrMissingMagicHeader
	}

	salt := data[magicLen : magicLen+16]
	nonce := data[magicLen+16 : magicLen+16+12]
	ciphertext := data[magicLen+16+12:]

	key := DeriveKey(passphrase, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// EncryptString encrypts a string value and formats it as "GMANENC:<salt_hex>:<nonce_hex>:<ciphertext_hex>".
func EncryptString(plaintext, passphrase string) (string, error) {
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	key := DeriveKey(passphrase, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	return fmt.Sprintf("%s%s:%s:%s",
		EncryptedPrefix,
		hex.EncodeToString(salt),
		hex.EncodeToString(nonce),
		hex.EncodeToString(ciphertext),
	), nil
}

// DecryptString decrypts an encrypted string formatted with the "GMANENC:" prefix.
func DecryptString(encrypted, passphrase string) (string, error) {
	if !strings.HasPrefix(encrypted, EncryptedPrefix) {
		return "", ErrMissingMagicHeader
	}

	payload := strings.TrimPrefix(encrypted, EncryptedPrefix)

	parts := strings.Split(payload, ":")
	if len(parts) != 3 {
		return "", ErrInvalidEncryptedFormat
	}

	salt, err := hex.DecodeString(parts[0])
	if err != nil || len(salt) != 16 {
		return "", ErrInvalidSalt
	}

	nonce, err := hex.DecodeString(parts[1])
	if err != nil || len(nonce) != 12 {
		return "", ErrInvalidNonce
	}

	ciphertext, err := hex.DecodeString(parts[2])
	if err != nil {
		return "", ErrInvalidCiphertext
	}

	key := DeriveKey(passphrase, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}

// IsEncryptedString checks if the string contains the "GMANENC:" encryption prefix.
func IsEncryptedString(val string) bool {
	return strings.HasPrefix(val, EncryptedPrefix)
}

// GetDeviceID generates a deterministic Android-style Steam Guard Device ID from a SteamID64.
// Format: "android:XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX".
func GetDeviceID(steamID uint64) string {
	h := sha1.New() //nolint:gosec

	var idBuf [20]byte

	b := strconv.AppendUint(idBuf[:0], steamID, 10)
	h.Write(b)

	var hashBuf [20]byte

	sum := h.Sum(hashBuf[:0])

	var hexBuf [40]byte
	hex.Encode(hexBuf[:], sum)

	var result [44]byte
	copy(result[:8], "android:")
	copy(result[8:16], hexBuf[:8])
	result[16] = '-'
	copy(result[17:21], hexBuf[8:12])
	result[21] = '-'
	copy(result[22:26], hexBuf[12:16])
	result[26] = '-'
	copy(result[27:31], hexBuf[16:20])
	result[31] = '-'
	copy(result[32:44], hexBuf[20:32])

	return string(result[:])
}
