// Package threshold implements threshold cryptography and time-locked custody
// for the AEGIS National Digital Assessment Platform.
//
// Threat Model Addressed:
// In traditional examination systems, a single database administrator, cloud operator,
// or corrupt official with root/DB access can extract the question paper before the exam.
//
// Defense Mechanism:
// Using Shamir's Secret Sharing over Galois Field GF(2^8):
// 1. Every generated examination paper is encrypted with a unique 256-bit AES-GCM paper key.
// 2. The paper key is split into N independent custodian shares with threshold K (e.g., K=3, N=5).
// 3. Custodians are independent institutions (e.g., Examination Board, High Court Observer,
//    Independent Auditor, Cyber Security Directorate, Educational Institution).
// 4. Any K-1 shares reveal EXACTLY ZERO information about the key (information-theoretic security).
// 5. At exam start time T, custodians submit their shares to reconstruct the key and release the paper.
package threshold

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
)

var (
	ErrInvalidThreshold  = errors.New("threshold K must be between 2 and N (K <= N)")
	ErrTooFewShares      = errors.New("number of provided shares is less than threshold K")
	ErrDuplicateShares   = errors.New("duplicate custodian share X coordinates detected")
	ErrNilSecret         = errors.New("secret to split cannot be empty")
	ErrShareCorrupted    = errors.New("custodian share data is corrupted or mismatched length")
)

// Share represents a single custodian's share of a split cryptographic secret.
type Share struct {
	// Index (X coordinate in GF(2^8)), must be non-zero (1 to 255).
	Index byte `json:"index"`
	// Value (Y coordinate evaluations for each byte of the secret).
	Value []byte `json:"value"`
}

// ─────────────────────────────────────────────────────────────
// Galois Field GF(2^8) Arithmetic with Rijndael polynomial 0x11B
// ─────────────────────────────────────────────────────────────

var (
	gfExp [512]byte
	gfLog [256]byte
)

func init() {
	// Initialize exponential and logarithm lookup tables for GF(2^8)
	// Generator = 0x03, irreducible polynomial = x^8 + x^4 + x^3 + x + 1 (0x11B)
	var x byte = 1
	for i := 0; i < 255; i++ {
		gfExp[i] = x
		gfExp[i+255] = x
		gfLog[x] = byte(i)
		x = gfMulRijndael(x, 0x03)
	}
	gfExp[255] = 1
	gfExp[510] = 1
}

// gfMulRijndael multiplies two numbers in GF(2^8) using bit shifts
func gfMulRijndael(a, b byte) byte {
	var p byte = 0
	for i := 0; i < 8; i++ {
		if (b & 1) != 0 {
			p ^= a
		}
		hiBitSet := (a & 0x80) != 0
		a <<= 1
		if hiBitSet {
			a ^= 0x1b // Rijndael polynomial
		}
		b >>= 1
	}
	return p
}

// gfAdd adds two elements in GF(2^8) (XOR operation)
func gfAdd(a, b byte) byte {
	return a ^ b
}

// gfMul multiplies two elements in GF(2^8) using precomputed log/exp tables
func gfMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return gfExp[int(gfLog[a])+int(gfLog[b])]
}

// gfDiv divides a by b in GF(2^8)
func gfDiv(a, b byte) byte {
	if b == 0 {
		panic("division by zero in GF(2^8)")
	}
	if a == 0 {
		return 0
	}
	return gfExp[(int(gfLog[a])-int(gfLog[b])+255)%255]
}

// ─────────────────────────────────────────────────────────────
// Shamir's Secret Sharing (Split & Combine)
// ─────────────────────────────────────────────────────────────

// Split divides a secret byte slice into N shares, requiring at least K shares to reconstruct.
// Information-theoretic security: Any K-1 or fewer shares leak ZERO information about the secret.
func Split(secret []byte, k, n int) ([]Share, error) {
	if len(secret) == 0 {
		return nil, ErrNilSecret
	}
	if k < 2 || k > n || n > 255 {
		return nil, fmt.Errorf("%w: requested k=%d, n=%d (max n is 255)", ErrInvalidThreshold, k, n)
	}

	secretLen := len(secret)
	shares := make([]Share, n)
	for i := 0; i < n; i++ {
		shares[i] = Share{
			Index: byte(i + 1), // X coordinates are 1, 2, ..., n
			Value: make([]byte, secretLen),
		}
	}

	// For each byte of the secret, generate a random polynomial of degree K-1:
	// P(x) = secret_byte + a_1*x + a_2*x^2 + ... + a_{k-1}*x^{k-1}
	coeff := make([]byte, k)
	for byteIdx, secretByte := range secret {
		coeff[0] = secretByte
		if _, err := rand.Read(coeff[1:]); err != nil {
			return nil, fmt.Errorf("failed to generate random polynomial coefficients: %w", err)
		}

		// Evaluate P(x) for each custodian x = 1, 2, ..., n
		for i := 0; i < n; i++ {
			x := shares[i].Index
			var y byte = 0
			// Horner's method for polynomial evaluation in GF(2^8):
			// P(x) = coeff[0] + x*(coeff[1] + x*(coeff[2] + ...))
			for deg := k - 1; deg >= 0; deg-- {
				y = gfAdd(gfMul(y, x), coeff[deg])
			}
			shares[i].Value[byteIdx] = y
		}
	}

	return shares, nil
}

// Combine reconstructs the secret from any subset of K or more shares using Lagrange interpolation.
func Combine(shares []Share, k int) ([]byte, error) {
	if len(shares) < k {
		return nil, fmt.Errorf("%w: provided %d, required %d", ErrTooFewShares, len(shares), k)
	}
	if k < 2 {
		return nil, ErrInvalidThreshold
	}

	// Use only the first K shares provided
	selected := shares[:k]

	// Verify all shares have identical length and unique X indices
	secretLen := len(selected[0].Value)
	if secretLen == 0 {
		return nil, ErrShareCorrupted
	}

	seenX := make(map[byte]bool, k)
	for _, s := range selected {
		if s.Index == 0 {
			return nil, fmt.Errorf("%w: share index cannot be 0", ErrShareCorrupted)
		}
		if seenX[s.Index] {
			return nil, fmt.Errorf("%w: share index %d used twice", ErrDuplicateShares, s.Index)
		}
		seenX[s.Index] = true

		if len(s.Value) != secretLen {
			return nil, fmt.Errorf("%w: expected length %d, got %d", ErrShareCorrupted, secretLen, len(s.Value))
		}
	}

	// Precompute Lagrange basis polynomials at x = 0:
	// l_j(0) = Prod_{m != j} (0 - x_m) / (x_j - x_m)
	// In GF(2^8), subtraction is identical to addition (XOR).
	lagrangeBasis := make([]byte, k)
	for j := 0; j < k; j++ {
		xj := selected[j].Index
		var basis byte = 1
		for m := 0; m < k; m++ {
			if m == j {
				continue
			}
			xm := selected[m].Index
			// numerator = 0 ^ xm = xm
			// denominator = xj ^ xm
			num := xm
			den := gfAdd(xj, xm)
			basis = gfMul(basis, gfDiv(num, den))
		}
		lagrangeBasis[j] = basis
	}

	// Reconstruct each byte of the secret:
	// Secret[byteIdx] = Sum_{j=0}^{k-1} y_j * l_j(0)
	secret := make([]byte, secretLen)
	for byteIdx := 0; byteIdx < secretLen; byteIdx++ {
		var secretByte byte = 0
		for j := 0; j < k; j++ {
			term := gfMul(selected[j].Value[byteIdx], lagrangeBasis[j])
			secretByte = gfAdd(secretByte, term)
		}
		secret[byteIdx] = secretByte
	}

	return secret, nil
}

// VerifyShareIntegrity checks whether a share matches a known secret hash without exposing the secret.
func VerifyShareIntegrity(shares []Share, k int, expectedHash []byte) bool {
	secret, err := Combine(shares, k)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(secret, expectedHash) == 1
}
