package threshold_test

import (
	"bytes"
	"crypto/rand"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aegis-platform/aegis/pkg/crypto/threshold"
)

// TestShamirSplitAndCombine tests basic Shamir threshold reconstruction
func TestShamirSplitAndCombine(t *testing.T) {
	secret := []byte("AEGIS-TOP-SECRET-PAPER-KEY-32BYTE!") // 32 bytes
	k := 3
	n := 5

	shares, err := threshold.Split(secret, k, n)
	require.NoError(t, err)
	require.Len(t, shares, n)

	// Combine using exactly K shares (e.g. shares 0, 2, 4)
	subset := []threshold.Share{shares[0], shares[2], shares[4]}
	recovered, err := threshold.Combine(subset, k)
	require.NoError(t, err)
	assert.Equal(t, secret, recovered)

	// Combine using another distinct subset of K shares (shares 1, 3, 4)
	subset2 := []threshold.Share{shares[1], shares[3], shares[4]}
	recovered2, err := threshold.Combine(subset2, k)
	require.NoError(t, err)
	assert.Equal(t, secret, recovered2)

	// Combine using all N shares
	recoveredAll, err := threshold.Combine(shares, k)
	require.NoError(t, err)
	assert.Equal(t, secret, recoveredAll)
}

// TestShamirKMinus1CannotReconstruct proves mathematically that K-1 shares CANNOT recover the secret
func TestShamirKMinus1CannotReconstruct(t *testing.T) {
	secret := []byte("SUPER-SENSITIVE-NATIONAL-KEY-32B")
	k := 4
	n := 7

	shares, err := threshold.Split(secret, k, n)
	require.NoError(t, err)

	// Provide only K-1 shares (3 shares instead of 4)
	kMinus1Shares := shares[:k-1]
	_, err = threshold.Combine(kMinus1Shares, k)
	require.Error(t, err)
	assert.ErrorIs(t, err, threshold.ErrTooFewShares, "Attempting to combine K-1 shares must fail with ErrTooFewShares")
}

// TestPaperCannotBeDecryptedBeforeT proves that even with ALL custodian shares,
// the question paper mathematically cannot be unlocked before exam start time T
func TestPaperCannotBeDecryptedBeforeT(t *testing.T) {
	scheduledT := time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC) // Exam scheduled for 9:00 AM UTC
	k := 3
	n := 5

	custodians := []threshold.Custodian{
		{ID: uuid.New(), Name: "Chief Examination Commissioner", Organization: "Exam Board"},
		{ID: uuid.New(), Name: "High Court Judicial Observer", Organization: "Judiciary"},
		{ID: uuid.New(), Name: "National Cyber Security Officer", Organization: "Cyber Directorate"},
		{ID: uuid.New(), Name: "Independent University Vice-Chancellor", Organization: "Academia"},
		{ID: uuid.New(), Name: "Lead Technical Auditor", Organization: "Audit Firm"},
	}

	fullQuestionPaperJSON := []byte(`{
		"paper_code": "UPSC-2026-SET-A",
		"instructions": "All 100 questions are compulsory.",
		"questions": [
			{"id": "q1", "stem": "Which fundamental right is guaranteed under Article 21?", "answer": "Right to Life"},
			{"id": "q2", "stem": "What is the capital of India?", "answer": "New Delhi"}
		]
	}`)

	pkg, shares, err := threshold.CreateTimeLockedPaper(
		uuid.New(),
		uuid.New(),
		"UPSC-2026-SET-A",
		scheduledT,
		k, n,
		custodians,
		fullQuestionPaperJSON,
	)
	require.NoError(t, err)
	require.NotNil(t, pkg)
	require.Len(t, shares, n)

	guard := threshold.NewTimeLockGuard(pkg)

	// Simulate all 5 custodians submitting their shares BEFORE the exam (e.g. at 8:30 AM, 30 mins before)
	guard.SetClock(func() time.Time {
		return time.Date(2026, 11, 15, 8, 30, 0, 0, time.UTC)
	})

	for _, share := range shares {
		err := guard.SubmitShare(share)
		require.NoError(t, err)
	}

	// Verify that unlock FAILS even though all shares are present
	decryptedPayload, err := guard.UnlockAndDecrypt()
	require.Error(t, err)
	assert.Nil(t, decryptedPayload)
	assert.ErrorIs(t, err, threshold.ErrExamNotStartedYet, "Release guard must reject decryption before time T")
}

// TestPaperCannotBeDecryptedWithLessThanK proves that at exam time T,
// having fewer than K shares (e.g. only 2 of 3) rejects decryption
func TestPaperCannotBeDecryptedWithLessThanK(t *testing.T) {
	scheduledT := time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)
	k := 3
	n := 5

	custodians := make([]threshold.Custodian, n)
	for i := 0; i < n; i++ {
		custodians[i] = threshold.Custodian{ID: uuid.New(), Name: "Custodian"}
	}

	secretContent := []byte("CONFIDENTIAL QUESTIONS")
	pkg, shares, err := threshold.CreateTimeLockedPaper(
		uuid.New(), uuid.New(), "PAPER-B", scheduledT, k, n, custodians, secretContent,
	)
	require.NoError(t, err)

	guard := threshold.NewTimeLockGuard(pkg)

	// Clock is AT exam time T (9:01 AM)
	guard.SetClock(func() time.Time {
		return time.Date(2026, 11, 15, 9, 1, 0, 0, time.UTC)
	})

	// Only 2 custodians submit shares (K=3 needed)
	require.NoError(t, guard.SubmitShare(shares[0]))
	require.NoError(t, guard.SubmitShare(shares[1]))

	decrypted, err := guard.UnlockAndDecrypt()
	require.Error(t, err)
	assert.Nil(t, decrypted)
	assert.ErrorIs(t, err, threshold.ErrThresholdNotMet)
}

// TestPaperDecryptionSuccessAtTWithKShares proves that at time >= T,
// having K shares decrypts the exact full paper content
func TestPaperDecryptionSuccessAtTWithKShares(t *testing.T) {
	scheduledT := time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)
	k := 3
	n := 5

	custodians := make([]threshold.Custodian, n)
	for i := 0; i < n; i++ {
		custodians[i] = threshold.Custodian{ID: uuid.New(), Name: "Custodian"}
	}

	expectedContent := []byte(`{"sections":[{"name":"Physics","questions":[{"id":"p1","text":"Calculate gravity"}]}]}`)

	pkg, shares, err := threshold.CreateTimeLockedPaper(
		uuid.New(), uuid.New(), "PAPER-FINAL", scheduledT, k, n, custodians, expectedContent,
	)
	require.NoError(t, err)

	guard := threshold.NewTimeLockGuard(pkg)

	// Clock is AT exam time T
	guard.SetClock(func() time.Time {
		return time.Date(2026, 11, 15, 9, 0, 1, 0, time.UTC)
	})

	// Submit exactly K shares (e.g. share 1, 2, 4)
	require.NoError(t, guard.SubmitShare(shares[1]))
	require.NoError(t, guard.SubmitShare(shares[2]))
	require.NoError(t, guard.SubmitShare(shares[4]))

	decrypted, err := guard.UnlockAndDecrypt()
	require.NoError(t, err)
	assert.True(t, bytes.Equal(expectedContent, decrypted), "Decrypted paper payload must match original content byte-for-byte")
}

// TestTamperedCiphertextDetected proves that modifying any byte of the encrypted payload is rejected
func TestTamperedCiphertextDetected(t *testing.T) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)

	plaintext := []byte("Question text here")
	ciphertext, err := threshold.EncryptAESGCM(key, plaintext)
	require.NoError(t, err)

	// Flip a bit in the ciphertext
	ciphertext[len(ciphertext)-1] ^= 0x01

	_, err = threshold.DecryptAESGCM(key, ciphertext)
	require.Error(t, err, "Decryption must fail when ciphertext is modified")
	assert.ErrorIs(t, err, threshold.ErrInvalidCiphertext)
}
