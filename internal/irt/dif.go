package irt

import (
	"errors"
	"math"
)

// MantelHaenszel implements the Mantel-Haenszel statistic for DIF detection.
// It tests whether item performance differs between reference and focal groups
// (e.g., language groups, gender groups) after controlling for ability.
type MantelHaenszel struct{}

// DIFResult holds the result of a DIF analysis for a single item
type DIFResult struct {
	ItemID      string
	MHChiSquare float64 // Mantel-Haenszel chi-square statistic
	MHOddsRatio float64 // Common odds ratio (α_MH)
	MHDelta     float64 // Δ_MH = -2.35 × ln(α_MH) (ETS delta scale)
	PValue      float64 // p-value from chi-square distribution
	DIFCategory string  // "A" (negligible), "B" (moderate), "C" (large)
	Flagged     bool    // Whether the item is flagged for review
}

// DetectDIF analyzes DIF using Mantel-Haenszel for a set of items.
// referenceResponses and focalResponses are matrices where each row is
// a candidate and each column is an item (0 = incorrect, 1 = correct).
// totalScores are the total scores used for ability matching.
func (mh *MantelHaenszel) DetectDIF(
	referenceResponses [][]int,
	focalResponses [][]int,
	referenceTotalScores []int,
	focalTotalScores []int,
) ([]DIFResult, error) {

	if len(referenceResponses) != len(referenceTotalScores) {
		return nil, errors.New("reference responses and scores length mismatch")
	}
	if len(focalResponses) != len(focalTotalScores) {
		return nil, errors.New("focal responses and scores length mismatch")
	}
	if len(referenceResponses) == 0 || len(focalResponses) == 0 {
		return nil, errors.New("empty response matrices")
	}

	numItems := len(referenceResponses[0])
	results := make([]DIFResult, numItems)

	// Determine max score for stratification
	maxScore := 0
	for _, s := range referenceTotalScores {
		if s > maxScore {
			maxScore = s
		}
	}
	for _, s := range focalTotalScores {
		if s > maxScore {
			maxScore = s
		}
	}

	for j := 0; j < numItems; j++ {
		var numAlpha, denAlpha float64
		var mhNumerator, mhVar float64

		for k := 0; k <= maxScore; k++ {
			// A_k = reference correct, B_k = reference incorrect
			// C_k = focal correct, D_k = focal incorrect
			Ak, Bk, Ck, Dk := 0.0, 0.0, 0.0, 0.0

			for i, score := range referenceTotalScores {
				if score == k {
					if referenceResponses[i][j] == 1 {
						Ak++
					} else {
						Bk++
					}
				}
			}

			for i, score := range focalTotalScores {
				if score == k {
					if focalResponses[i][j] == 1 {
						Ck++
					} else {
						Dk++
					}
				}
			}

			Nk := Ak + Bk + Ck + Dk
			if Nk == 0 {
				continue
			}

			nR := Ak + Bk // total in reference in stratum k
			nF := Ck + Dk // total in focal in stratum k
			m1 := Ak + Ck // total correct in stratum k
			m0 := Bk + Dk // total incorrect in stratum k

			numAlpha += (Ak * Dk) / Nk
			denAlpha += (Bk * Ck) / Nk

			expectedAk := (nR * m1) / Nk
			varAk := (nR * nF * m1 * m0) / (Nk * Nk * (Nk - 1.0))
			if Nk > 1 {
				mhNumerator += Ak - expectedAk
				mhVar += varAk
			}
		}

		alphaMH := 1.0
		if denAlpha > 0 {
			alphaMH = numAlpha / denAlpha
		}

		deltaMH := -2.35 * math.Log(alphaMH)

		chiSquareMH := 0.0
		if mhVar > 0 {
			// Continuity correction
			absNum := math.Abs(mhNumerator) - 0.5
			if absNum < 0 {
				absNum = 0
			}
			chiSquareMH = (absNum * absNum) / mhVar
		}

		// Calculate p-value for 1 degree of freedom chi-square
		pValue := 1.0 - chiSquareCDF(chiSquareMH)

		// Classification
		absDelta := math.Abs(deltaMH)
		category := "A"
		flagged := false

		significant := pValue < 0.05
		if absDelta >= 1.5 && significant {
			category = "C"
			flagged = true
		} else if absDelta >= 1.0 && significant {
			category = "B"
			flagged = true
		}

		results[j] = DIFResult{
			MHChiSquare: chiSquareMH,
			MHOddsRatio: alphaMH,
			MHDelta:     deltaMH,
			PValue:      pValue,
			DIFCategory: category,
			Flagged:     flagged,
		}
	}

	return results, nil
}

// chiSquareCDF computes the CDF of the chi-square distribution with 1 DOF.
// Used for calculating p-values.
func chiSquareCDF(x float64) float64 {
	if x <= 0 {
		return 0
	}
	return math.Erf(math.Sqrt(x / 2.0))
}
