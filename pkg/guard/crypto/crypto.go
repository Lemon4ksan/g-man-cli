// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package crypto provides encryption utilities for the g-man CLI.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// CryptoMagic is the magic header prefix for encrypted files.
const CryptoMagic = "GMANENC\x00"

// EncryptedPrefix is the prefix used for encrypted values inside .env.
const EncryptedPrefix = "GMANENC:"

// DeriveKey derives a 32-byte key from a passphrase and a salt using 100,000 rounds of SHA-256.
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

	// Combine: Magic + Salt + Nonce + Ciphertext
	out := make([]byte, 0, len(CryptoMagic)+len(salt)+len(nonce)+len(ciphertext))
	out = append(out, CryptoMagic...)
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)

	return out, nil
}

// DecryptData decrypts ciphertext bytes using AES-256-GCM.
func DecryptData(data []byte, passphrase string) ([]byte, error) {
	magicLen := len(CryptoMagic)
	if len(data) < magicLen+16+12+16 {
		return nil, errors.New("invalid or corrupted encrypted data: file too short")
	}

	if string(data[:magicLen]) != CryptoMagic {
		return nil, errors.New("invalid encrypted data: missing GMANENC magic header")
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
		return nil, errors.New("decryption failed: invalid passphrase or corrupted data")
	}

	return plaintext, nil
}

// EncryptString encrypts a string and formats it as "GMANENC:<salt_hex>:<nonce_hex>:<ciphertext_hex>".
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

// DecryptString decrypts a formatted encrypted string.
func DecryptString(encrypted, passphrase string) (string, error) {
	if !strings.HasPrefix(encrypted, EncryptedPrefix) {
		return "", errors.New("invalid encrypted string: missing prefix")
	}

	payload := strings.TrimPrefix(encrypted, EncryptedPrefix)

	parts := strings.Split(payload, ":")
	if len(parts) != 3 {
		return "", errors.New("invalid encrypted string format: must have 3 parts")
	}

	salt, err := hex.DecodeString(parts[0])
	if err != nil || len(salt) != 16 {
		return "", errors.New("invalid salt encoding or size")
	}

	nonce, err := hex.DecodeString(parts[1])
	if err != nil || len(nonce) != 12 {
		return "", errors.New("invalid nonce encoding or size")
	}

	ciphertext, err := hex.DecodeString(parts[2])
	if err != nil {
		return "", errors.New("invalid ciphertext encoding")
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
		return "", errors.New("decryption failed: invalid passphrase or corrupted data")
	}

	return string(plaintext), nil
}

// IsEncryptedString checks if the string starts with the GMANENC: prefix.
func IsEncryptedString(val string) bool {
	return strings.HasPrefix(val, EncryptedPrefix)
}
