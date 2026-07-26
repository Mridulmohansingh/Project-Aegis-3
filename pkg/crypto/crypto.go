// Package crypto provides cryptographic primitives for the AEGIS platform.
//
// It implements:
//   - AES-256-GCM authenticated encryption (envelope encryption pattern)
//   - Ed25519 digital signatures for audit chain integrity
//   - SHA-256 hashing for content integrity verification
//   - Key management abstractions (HSM/Vault backends)
//
// All cryptographic operations use Go's standard crypto packages.
// Key material is never logged, serialized to JSON, or exposed in error messages.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"
)

// ──────────────────────────────────────────────
//  Key Management Interface
// ──────────────────────────────────────────────

// KeyManager abstracts key management operations. Implementations may use
// HSM, HashiCorp Vault, AWS KMS, or local key storage.
type KeyManager interface {
	// GenerateDataKey creates a new AES-256 data encryption key and returns
	// both the plaintext key and the encrypted (wrapped) version.
	// The key ID is used to retrieve the wrapping key for future decryption.
	GenerateDataKey(keyID string) (plaintext []byte, ciphertext []byte, err error)

	// DecryptDataKey unwraps an encrypted data key using the key specified by keyID.
	DecryptDataKey(keyID string, encryptedKey []byte) ([]byte, error)

	// GetSigningKey returns the Ed25519 private key for digital signatures.
	GetSigningKey(keyID string) (ed25519.PrivateKey, error)

	// GetVerificationKey returns the Ed25519 public key for signature verification.
	GetVerificationKey(keyID string) (ed25519.PublicKey, error)
}

// ──────────────────────────────────────────────
//  AES-256-GCM Encryption Service
// ──────────────────────────────────────────────

// EncryptionService provides AES-256-GCM authenticated encryption.
// It uses the envelope encryption pattern:
//  1. A Data Encryption Key (DEK) is generated per item/session.
//  2. The DEK is used to encrypt the data.
//  3. The DEK itself is encrypted by a Key Encryption Key (KEK) in the HSM/KMS.
//  4. The encrypted DEK is stored alongside the ciphertext.
type EncryptionService struct {
	keyManager KeyManager
	mu         sync.RWMutex
}

// NewEncryptionService creates a new EncryptionService with the given key manager.
func NewEncryptionService(km KeyManager) *EncryptionService {
	return &EncryptionService{keyManager: km}
}

// EncryptedBlob holds the encrypted data along with the key reference for decryption.
type EncryptedBlob struct {
	// Ciphertext is the AES-256-GCM encrypted data (nonce prepended).
	Ciphertext []byte `json:"-"` // Never serialize to JSON API responses
	// EncryptedDEK is the wrapped Data Encryption Key.
	EncryptedDEK []byte `json:"-"`
	// KeyID identifies the Key Encryption Key used to wrap the DEK.
	KeyID string `json:"-"`
	// Nonce size used (stored for future compatibility).
	NonceSize int `json:"-"`
}

// Encrypt encrypts plaintext using AES-256-GCM with a fresh DEK.
//
// The process:
//  1. Generate a new DEK via the key manager
//  2. Create AES-256-GCM cipher with the DEK
//  3. Generate a random 12-byte nonce
//  4. Encrypt with AEAD (nonce prepended to ciphertext)
//  5. Return ciphertext + encrypted DEK + key reference
//
// The additionalData parameter provides authenticated but unencrypted context
// (e.g., item ID) to prevent ciphertext transplant attacks.
func (s *EncryptionService) Encrypt(plaintext []byte, keyID string, additionalData []byte) (*EncryptedBlob, error) {
	if len(plaintext) == 0 {
		return nil, errors.New("plaintext cannot be empty")
	}

	// Generate a fresh DEK
	dekPlaintext, dekEncrypted, err := s.keyManager.GenerateDataKey(keyID)
	if err != nil {
		return nil, fmt.Errorf("generating data encryption key: %w", err)
	}
	// Immediately defer zeroing the plaintext DEK
	defer zeroBytes(dekPlaintext)

	// Create AES cipher
	block, err := aes.NewCipher(dekPlaintext)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	// Encrypt (nonce is prepended to ciphertext)
	ciphertext := gcm.Seal(nonce, nonce, plaintext, additionalData)

	return &EncryptedBlob{
		Ciphertext:   ciphertext,
		EncryptedDEK: dekEncrypted,
		KeyID:        keyID,
		NonceSize:    gcm.NonceSize(),
	}, nil
}

// Decrypt decrypts an EncryptedBlob using the key manager to unwrap the DEK.
func (s *EncryptionService) Decrypt(blob *EncryptedBlob, additionalData []byte) ([]byte, error) {
	if blob == nil || len(blob.Ciphertext) == 0 {
		return nil, errors.New("encrypted blob is empty")
	}

	// Unwrap the DEK
	dekPlaintext, err := s.keyManager.DecryptDataKey(blob.KeyID, blob.EncryptedDEK)
	if err != nil {
		return nil, fmt.Errorf("decrypting data encryption key: %w", err)
	}
	defer zeroBytes(dekPlaintext)

	// Create AES cipher
	block, err := aes.NewCipher(dekPlaintext)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	// Extract nonce from prepended ciphertext
	nonceSize := gcm.NonceSize()
	if len(blob.Ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce := blob.Ciphertext[:nonceSize]
	ciphertext := blob.Ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, additionalData)
	if err != nil {
		return nil, fmt.Errorf("decryption failed (data may be tampered): %w", err)
	}

	return plaintext, nil
}

// EncryptWithKey encrypts plaintext directly with a provided AES-256 key.
// Use this for scenarios where key management is handled externally.
func EncryptWithKey(plaintext, key, additionalData []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be exactly 32 bytes (AES-256), got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	return gcm.Seal(nonce, nonce, plaintext, additionalData), nil
}

// DecryptWithKey decrypts ciphertext directly with a provided AES-256 key.
func DecryptWithKey(ciphertext, key, additionalData []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be exactly 32 bytes (AES-256), got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce := ciphertext[:nonceSize]
	ct := ciphertext[nonceSize:]

	return gcm.Open(nil, nonce, ct, additionalData)
}

// ──────────────────────────────────────────────
//  Digital Signatures (Ed25519)
// ──────────────────────────────────────────────

// SigningService provides Ed25519 digital signature operations.
type SigningService struct {
	keyManager KeyManager
}

// NewSigningService creates a new SigningService.
func NewSigningService(km KeyManager) *SigningService {
	return &SigningService{keyManager: km}
}

// Sign creates an Ed25519 signature over the given data.
func (s *SigningService) Sign(data []byte, keyID string) ([]byte, error) {
	privateKey, err := s.keyManager.GetSigningKey(keyID)
	if err != nil {
		return nil, fmt.Errorf("getting signing key: %w", err)
	}
	return ed25519.Sign(privateKey, data), nil
}

// Verify checks an Ed25519 signature against the given data.
func (s *SigningService) Verify(data, signature []byte, keyID string) (bool, error) {
	publicKey, err := s.keyManager.GetVerificationKey(keyID)
	if err != nil {
		return false, fmt.Errorf("getting verification key: %w", err)
	}
	return ed25519.Verify(publicKey, data, signature), nil
}

// ──────────────────────────────────────────────
//  Hashing
// ──────────────────────────────────────────────

// SHA256Hash computes the SHA-256 hash of the given data.
func SHA256Hash(data []byte) [32]byte {
	return sha256.Sum256(data)
}

// SHA256HashHex computes the SHA-256 hash and returns it as a hex string.
func SHA256HashHex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// SHA256Chain computes SHA-256(previousHash || data) for Merkle chain linking.
func SHA256Chain(previousHash, data []byte) [32]byte {
	combined := make([]byte, 0, len(previousHash)+len(data))
	combined = append(combined, previousHash...)
	combined = append(combined, data...)
	return sha256.Sum256(combined)
}

// ──────────────────────────────────────────────
//  Local Key Manager (Development/Testing)
// ──────────────────────────────────────────────

// LocalKeyManager is a key manager that stores keys in memory.
// FOR DEVELOPMENT AND TESTING ONLY. Never use in production.
// Production must use HSM-backed or Vault-backed key managers.
type LocalKeyManager struct {
	mu          sync.RWMutex
	masterKey   []byte // 32-byte master KEK
	signingKeys map[string]ed25519.PrivateKey
}

// NewLocalKeyManager creates a local key manager with a random master key.
// FOR DEVELOPMENT AND TESTING ONLY.
func NewLocalKeyManager() (*LocalKeyManager, error) {
	masterKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, masterKey); err != nil {
		return nil, fmt.Errorf("generating master key: %w", err)
	}

	return &LocalKeyManager{
		masterKey:   masterKey,
		signingKeys: make(map[string]ed25519.PrivateKey),
	}, nil
}

// GenerateDataKey generates a random 32-byte DEK and wraps it with the master key.
func (km *LocalKeyManager) GenerateDataKey(keyID string) ([]byte, []byte, error) {
	dek := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return nil, nil, fmt.Errorf("generating DEK: %w", err)
	}

	// Wrap DEK with master key using AES-256-GCM
	wrappedDEK, err := EncryptWithKey(dek, km.masterKey, []byte(keyID))
	if err != nil {
		return nil, nil, fmt.Errorf("wrapping DEK: %w", err)
	}

	return dek, wrappedDEK, nil
}

// DecryptDataKey unwraps an encrypted DEK using the master key.
func (km *LocalKeyManager) DecryptDataKey(keyID string, encryptedKey []byte) ([]byte, error) {
	return DecryptWithKey(encryptedKey, km.masterKey, []byte(keyID))
}

// GetSigningKey returns or generates an Ed25519 signing key.
func (km *LocalKeyManager) GetSigningKey(keyID string) (ed25519.PrivateKey, error) {
	km.mu.Lock()
	defer km.mu.Unlock()

	if key, ok := km.signingKeys[keyID]; ok {
		return key, nil
	}

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating Ed25519 key: %w", err)
	}

	km.signingKeys[keyID] = privateKey
	return privateKey, nil
}

// GetVerificationKey returns the public key corresponding to a signing key.
func (km *LocalKeyManager) GetVerificationKey(keyID string) (ed25519.PublicKey, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	privateKey, ok := km.signingKeys[keyID]
	if !ok {
		return nil, fmt.Errorf("signing key '%s' not found", keyID)
	}

	return privateKey.Public().(ed25519.PublicKey), nil
}

// ──────────────────────────────────────────────
//  Utilities
// ──────────────────────────────────────────────

// zeroBytes securely zeroes a byte slice to prevent key material from lingering in memory.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// GenerateRandomBytes returns n cryptographically random bytes.
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, fmt.Errorf("generating random bytes: %w", err)
	}
	return b, nil
}
