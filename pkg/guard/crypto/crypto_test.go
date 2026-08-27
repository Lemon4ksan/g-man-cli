// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by aBSD-style
// license that can be found in the LICENSE file.

package crypto_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-cli/pkg/guard/crypto"
)

func TestDeriveKey(t *testing.T) {
	key1 := crypto.DeriveKey("password", []byte("salt123456789012"))
	key2 := crypto.DeriveKey("password", []byte("salt123456789012"))
	key3 := crypto.DeriveKey("password", []byte("salt999999999999"))
	key4 := crypto.DeriveKey("different", []byte("salt123456789012"))

	assert.Equal(t, key1, key2)
	assert.NotEqual(t, key1, key3)
	assert.NotEqual(t, key1, key4)
	assert.Len(t, key1, 32)
}

func TestEncryptDecryptData(t *testing.T) {
	passphrase := "my-secret-passphrase"
	plaintext := []byte("hello world this is a test string!")

	encrypted, err := crypto.EncryptData(plaintext, passphrase)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)

	decrypted, err := crypto.DecryptData(encrypted, passphrase)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)

	_, err = crypto.DecryptData(encrypted, "wrong-passphrase")
	assert.Error(t, err)

	_, err = crypto.DecryptData([]byte("too short"), passphrase)
	assert.Error(t, err)

	badMagic := make([]byte, len(encrypted))
	copy(badMagic, encrypted)
	badMagic[0] = 'X'
	_, err = crypto.DecryptData(badMagic, passphrase)
	assert.Error(t, err)
}

func TestEncryptDecryptString(t *testing.T) {
	passphrase := "string-passphrase"
	plaintext := "secret message here"

	encrypted, err := crypto.EncryptString(plaintext, passphrase)
	require.NoError(t, err)
	assert.True(t, crypto.IsEncryptedString(encrypted))

	decrypted, err := crypto.DecryptString(encrypted, passphrase)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)

	_, err = crypto.DecryptString(encrypted, "wrong")
	assert.Error(t, err)

	assert.False(t, crypto.IsEncryptedString("plain text"))

	_, err = crypto.DecryptString("plain text", passphrase)
	assert.Error(t, err)

	_, err = crypto.DecryptString("GMANENC:part1", passphrase)
	assert.Error(t, err)

	// Invalid salt/nonce sizes (valid hex but wrong length)
	_, err = crypto.DecryptString("GMANENC:aabbcc:aabbccddeeff001122334455:cipher", passphrase)
	assert.Error(t, err)

	_, err = crypto.DecryptString("GMANENC:aabbccddeeff00112233445566778899:aabbcc:cipher", passphrase)
	assert.Error(t, err)

	// Invalid hex parts
	_, err = crypto.DecryptString("GMANENC:invalidhex:nonce:cipher", passphrase)
	assert.Error(t, err)

	_, err = crypto.DecryptString("GMANENC:aabbccddeeff00112233445566778899:invalidnonce:cipher", passphrase)
	assert.Error(t, err)

	_, err = crypto.DecryptString(
		"GMANENC:aabbccddeeff00112233445566778899:aabbccddeeff001122334455:invalidcipher",
		passphrase,
	)
	assert.Error(t, err)
}
