package crypto_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aegis-platform/aegis/pkg/crypto"
)

func TestPersistentKeyManagerSurvivesRestarts(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "aegis_key_test")
	defer os.RemoveAll(tempDir)

	// Phase 1: Initialize Key Manager, generate DEK and encrypt
	km1, err := crypto.NewPersistentFileKeyManager(tempDir)
	require.NoError(t, err)

	keyID := "exam-key-2026"
	plaintextDEK, encryptedDEK, err := km1.GenerateDataKey(keyID)
	require.NoError(t, err)

	signingKeyID := "audit-signer"
	sigKey1, err := km1.GetSigningKey(signingKeyID)
	require.NoError(t, err)
	pubKey1, err := km1.GetVerificationKey(signingKeyID)
	require.NoError(t, err)

	// Phase 2: Simulate complete server reboot / pod recreation by creating a fresh instance from same directory
	km2, err := crypto.NewPersistentFileKeyManager(tempDir)
	require.NoError(t, err)

	// Verify that km2 can still decrypt the DEK created by km1
	decryptedDEK, err := km2.DecryptDataKey(keyID, encryptedDEK)
	require.NoError(t, err)
	assert.Equal(t, plaintextDEK, decryptedDEK, "Persistent Key Manager must decrypt data encrypted prior to restart")

	// Verify signing keys are identical across restarts
	sigKey2, err := km2.GetSigningKey(signingKeyID)
	require.NoError(t, err)
	pubKey2, err := km2.GetVerificationKey(signingKeyID)
	require.NoError(t, err)

	assert.Equal(t, []byte(sigKey1), []byte(sigKey2))
	assert.Equal(t, []byte(pubKey1), []byte(pubKey2))
}
