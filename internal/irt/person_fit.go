package irt

import (
	"errors"
	"math"
)

// PersonFit computes person-fit statistics to identify examinees
// with response patterns inconsistent with their estimated ability.
type PersonFit struct{}

// PersonFitResult holds person-fit statistics for one examinee
type PersonFitResult struct {
	CandidateID string
	Lz          float64 // Standardized log-likelihood (l_z)
	LzPValue    float64 // p-value for l_z
	Flagged     bool    // Whether the pattern is flagged as aberrant
	FlagReason  string  // e.g., "unexpected correct on hard items", "unexpected incorrect on easy items"
}

// ComputeLz computes the standardized log-likelihood person-fit statistic.
// l_z = (l_0 - E(l_0)) / √(Var(l_0))
// where l_0 = Σ[u_i ln P_i(θ) + (1-u_i) ln Q_i(θ)]
// Flagged if l_z < -2.0 (significant at p < 0.023)
func (pf *PersonFit) ComputeLz(items []ItemParams, responses []int, theta float64) (*PersonFitResult, error) {
	if len(items) != len(responses) {
		return nil, errors.New("items and responses length mismatch")
	}

	model := Model3PL{}
	var l0, el0, varl0 float64

	for i, item := range items {
		p := model.Probability(item, theta)
		q := 1.0 - p
		pSafe := math.Max(1e-10, p)
		qSafe := math.Max(1e-10, q)

		u := float64(responses[i])

		l0 += u*math.Log(pSafe) + (1.0-u)*math.Log(qSafe)
		el0 += p*math.Log(pSafe) + q*math.Log(qSafe)

		logRatio := math.Log(pSafe / qSafe)
		varl0 += p * q * logRatio * logRatio
	}

	if varl0 == 0 {
		return nil, errors.New("zero variance in expected log-likelihood")
	}

	lz := (l0 - el0) / math.Sqrt(varl0)

	// p-value from normal distribution (lower tail)
	pValue := 0.5 * (1.0 + math.Erf(lz/math.Sqrt(2.0)))

	flagged := lz < -2.0
	reason := ""
	if flagged {
		reason = "aberrant response pattern"
	}

	return &PersonFitResult{
		Lz:         lz,
		LzPValue:   pValue,
		Flagged:    flagged,
		FlagReason: reason,
	}, nil
}
