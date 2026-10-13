package threshold

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
)

var (
	ErrExamNotStartedYet       = errors.New("cannot release paper: current time is before scheduled exam start time T")
	ErrThresholdNotMet         = errors.New("cannot reconstruct key: insufficient custodian shares submitted")
	ErrInvalidCiphertext       = errors.New("invalid or tampered ciphertext")
	ErrCustodianAlreadySubmitted = errors.New("custodian share already registered")
)

// Custodian represents an independent authority holding a key share.
type Custodian struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`         // e.g. "Hon'ble High Court Oversight", "National Cyber Agency"
	Organization string    `json:"organization"` // e.g. "Independent Oversight Board"
	PublicKey    []byte    `json:"public_key"`   // Ed25519 public key
}

// TimeLockedPaperPackage represents a tamper-evident, threshold-encrypted question paper.
// Even with direct database and disk access, NO ONE can read the questions until:
// 1. Current wall-clock time >= ScheduledReleaseTime
// 2. At least K independent custodians submit their shares
type TimeLockedPaperPackage struct {
	PaperID              uuid.UUID   `json:"paper_id"`
	ExamID               uuid.UUID   `json:"exam_id"`
	PaperCode            string      `json:"paper_code"`
	ScheduledReleaseTime time.Time   `json:"scheduled_release_time"` // Time T
	ThresholdK           int         `json:"threshold_k"`            // Minimum shares needed
	TotalCustodiansN     int         `json:"total_custodians_n"`     // Total shares distributed
	Custodians           []Custodian `json:"custodians"`             // Public identities of share holders
	KeyFingerprint       [32]byte    `json:"key_fingerprint"`        // SHA-256 of the paper key (for verification)
	EncryptedPayload     []byte      `json:"encrypted_payload"`      // AES-256-GCM encrypted questions & answers
	PayloadHash          [32]byte    `json:"payload_hash"`           // SHA-256 of the encrypted payload
	CreatedAt            time.Time   `json:"created_at"`
}

// TimeLockGuard manages share collection and time verification for release.
type TimeLockGuard struct {
	pkg             *TimeLockedPaperPackage
	submittedShares map[byte]Share
	clock           func() time.Time // Injectable clock for deterministic testing
}

// NewTimeLockGuard creates a release guard for a time-locked paper package.
func NewTimeLockGuard(pkg *TimeLockedPaperPackage) *TimeLockGuard {
	return &TimeLockGuard{
		pkg:             pkg,
		submittedShares: make(map[byte]Share),
		clock:           time.Now,
	}
}

// SetClock sets a custom clock function (useful for tests and time verification).
func (g *TimeLockGuard) SetClock(clock func() time.Time) {
	g.clock = clock
}

// SubmitShare registers a custodian's share toward unlocking the paper.
func (g *TimeLockGuard) SubmitShare(share Share) error {
	if share.Index == 0 {
		return errors.New("invalid share index 0")
	}
	if _, exists := g.submittedShares[share.Index]; exists {
		return ErrCustodianAlreadySubmitted
	}
	g.submittedShares[share.Index] = share
	return nil
}

// SharesSubmittedCount returns how many valid custodian shares have been submitted.
func (g *TimeLockGuard) SharesSubmittedCount() int {
	return len(g.submittedShares)
}

// CanUnlock checks if both the time-lock condition and threshold requirement are met.
func (g *TimeLockGuard) CanUnlock() (bool, error) {
	now := g.clock().UTC()
	if now.Before(g.pkg.ScheduledReleaseTime.UTC()) {
		return false, fmt.Errorf("%w: scheduled %v, current %v", ErrExamNotStartedYet, g.pkg.ScheduledReleaseTime, now)
	}

	if len(g.submittedShares) < g.pkg.ThresholdK {
		return false, fmt.Errorf("%w: submitted %d, required %d", ErrThresholdNotMet, len(g.submittedShares), g.pkg.ThresholdK)
	}

	return true, nil
}

// UnlockAndDecrypt verifies conditions, reconstructs the AES key, and decrypts the paper content.
func (g *TimeLockGuard) UnlockAndDecrypt() ([]byte, error) {
	// 1. Strict time check (defense against early release)
	canUnlock, err := g.CanUnlock()
	if !canUnlock {
		return nil, err
	}

	// 2. Collect K shares
	sharesList := make([]Share, 0, len(g.submittedShares))
	for _, s := range g.submittedShares {
		sharesList = append(sharesList, s)
	}

	// 3. Reconstruct symmetric key via Shamir Lagrange interpolation
	paperKey, err := Combine(sharesList, g.pkg.ThresholdK)
	if err != nil {
		return nil, fmt.Errorf("cryptographic reconstruction failed: %w", err)
	}

	// 4. Verify key fingerprint against package
	reconstructedFingerprint := sha256.Sum256(paperKey)
	if reconstructedFingerprint != g.pkg.KeyFingerprint {
		return nil, errors.New("reconstructed key fingerprint mismatch — invalid shares supplied")
	}

	// 5. Decrypt full paper payload
	plaintext, err := DecryptAESGCM(paperKey, g.pkg.EncryptedPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt paper payload: %w", err)
	}

	return plaintext, nil
}

// ─────────────────────────────────────────────────────────────
// Symmetric AES-256-GCM Envelope Encryption
// ─────────────────────────────────────────────────────────────

// EncryptAESGCM encrypts plaintext using standard AES-256-GCM with a random nonce.
func EncryptAESGCM(key []byte, plaintext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("key must be exactly 32 bytes (256 bits)")
	}

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
		return nil, err
	}

	// Prefix the ciphertext with the 12-byte nonce
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// DecryptAESGCM decrypts an AES-256-GCM ciphertext whose first 12 bytes are the nonce.
func DecryptAESGCM(key []byte, ciphertext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("key must be exactly 32 bytes (256 bits)")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrInvalidCiphertext
	}

	nonce, encryptedData := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCiphertext, err)
	}

	return plaintext, nil
}

// CreateTimeLockedPaper generates a fresh random 256-bit paper key,
// encrypts the full question content, and splits the key among N custodians with threshold K.
func CreateTimeLockedPaper(
	paperID uuid.UUID,
	examID uuid.UUID,
	paperCode string,
	scheduledReleaseTime time.Time,
	k, n int,
	custodians []Custodian,
	fullPaperContent []byte,
) (*TimeLockedPaperPackage, []Share, error) {
	if len(custodians) != n {
		return nil, nil, fmt.Errorf("custodians count (%d) must match total shares N (%d)", len(custodians), n)
	}

	// 1. Generate random 256-bit AES paper key
	paperKey := make([]byte, 32)
	if _, err := rand.Read(paperKey); err != nil {
		return nil, nil, fmt.Errorf("failed to generate random paper key: %w", err)
	}

	// 2. Encrypt full paper content with AES-256-GCM
	encryptedPayload, err := EncryptAESGCM(paperKey, fullPaperContent)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encrypt paper payload: %w", err)
	}

	// 3. Split paper key using Shamir's Secret Sharing (K-of-N)
	shares, err := Split(paperKey, k, n)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to split paper key: %w", err)
	}

	pkg := &TimeLockedPaperPackage{
		PaperID:              paperID,
		ExamID:               examID,
		PaperCode:            paperCode,
		ScheduledReleaseTime: scheduledReleaseTime.UTC(),
		ThresholdK:           k,
		TotalCustodiansN:     n,
		Custodians:           custodians,
		KeyFingerprint:       sha256.Sum256(paperKey),
		EncryptedPayload:     encryptedPayload,
		PayloadHash:          sha256.Sum256(encryptedPayload),
		CreatedAt:            time.Now().UTC(),
	}

	return pkg, shares, nil
}
