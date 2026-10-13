package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// PersistentFileKeyManager implements KeyManager by storing cryptographic keys
// securely on disk. Unlike LocalKeyManager, keys survive pod and process restarts.
//
// File permissions are strictly enforced to 0600 (owner read/write only).
type PersistentFileKeyManager struct {
	mu          sync.RWMutex
	baseDir     string
	masterKey   []byte // 32-byte master KEK
	signingKeys map[string]ed25519.PrivateKey
}

// NewPersistentFileKeyManager creates or loads a persistent key manager from baseDir.
// If keys do not exist, they are generated cryptographically and saved to disk.
func NewPersistentFileKeyManager(baseDir string) (*PersistentFileKeyManager, error) {
	if baseDir == "" {
		baseDir = "./.aegis_keys"
	}

	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return nil, fmt.Errorf("creating key directory: %w", err)
	}

	km := &PersistentFileKeyManager{
		baseDir:     baseDir,
		signingKeys: make(map[string]ed25519.PrivateKey),
	}

	// 1. Load or initialize master Key Encryption Key (KEK)
	masterKeyPath := filepath.Join(baseDir, "master.kek")
	if _, err := os.Stat(masterKeyPath); os.IsNotExist(err) {
		masterKey := make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, masterKey); err != nil {
			return nil, fmt.Errorf("generating master key: %w", err)
		}
		if err := os.WriteFile(masterKeyPath, []byte(hex.EncodeToString(masterKey)), 0600); err != nil {
			return nil, fmt.Errorf("saving master key: %w", err)
		}
		km.masterKey = masterKey
	} else {
		data, err := os.ReadFile(masterKeyPath)
		if err != nil {
			return nil, fmt.Errorf("reading master key: %w", err)
		}
		key, err := hex.DecodeString(string(data))
		if err != nil || len(key) != 32 {
			return nil, errors.New("corrupted master key file")
		}
		km.masterKey = key
	}

	return km, nil
}

// GenerateDataKey generates a random 32-byte DEK and wraps it with the persistent master key.
func (km *PersistentFileKeyManager) GenerateDataKey(keyID string) ([]byte, []byte, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	dek := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return nil, nil, fmt.Errorf("generating DEK: %w", err)
	}

	wrappedDEK, err := EncryptWithKey(dek, km.masterKey, []byte(keyID))
	if err != nil {
		return nil, nil, fmt.Errorf("wrapping DEK: %w", err)
	}

	return dek, wrappedDEK, nil
}

// DecryptDataKey unwraps an encrypted DEK using the persistent master key.
func (km *PersistentFileKeyManager) DecryptDataKey(keyID string, encryptedKey []byte) ([]byte, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	return DecryptWithKey(encryptedKey, km.masterKey, []byte(keyID))
}

// GetSigningKey retrieves or creates a persistent Ed25519 signing key.
func (km *PersistentFileKeyManager) GetSigningKey(keyID string) (ed25519.PrivateKey, error) {
	km.mu.Lock()
	defer km.mu.Unlock()

	if key, ok := km.signingKeys[keyID]; ok {
		return key, nil
	}

	keyFile := filepath.Join(km.baseDir, fmt.Sprintf("%s.signing.key", keyID))
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		// Generate fresh Ed25519 keypair
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generating Ed25519 key: %w", err)
		}

		// Save private key (0600)
		if err := os.WriteFile(keyFile, []byte(hex.EncodeToString(priv)), 0600); err != nil {
			return nil, fmt.Errorf("saving signing key: %w", err)
		}

		// Save public key for verification (0644)
		pubFile := filepath.Join(km.baseDir, fmt.Sprintf("%s.pub.key", keyID))
		_ = os.WriteFile(pubFile, []byte(hex.EncodeToString(pub)), 0644)

		km.signingKeys[keyID] = priv
		return priv, nil
	}

	// Load existing private key
	data, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("reading signing key: %w", err)
	}

	privBytes, err := hex.DecodeString(string(data))
	if err != nil || len(privBytes) != ed25519.PrivateKeySize {
		return nil, errors.New("corrupted signing key file")
	}

	priv := ed25519.PrivateKey(privBytes)
	km.signingKeys[keyID] = priv
	return priv, nil
}

// GetVerificationKey returns the public verification key for a given signing keyID.
func (km *PersistentFileKeyManager) GetVerificationKey(keyID string) (ed25519.PublicKey, error) {
	priv, err := km.GetSigningKey(keyID)
	if err != nil {
		return nil, err
	}
	return priv.Public().(ed25519.PublicKey), nil
}
