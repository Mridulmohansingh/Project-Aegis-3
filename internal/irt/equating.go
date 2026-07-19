package irt

import (
	"errors"
	"math"
)

// TrueScoreEquater implements IRT true-score equating across test forms.
// Given θ estimated on Form X, compute the equivalent true score on Form Y.
//
// True score on form: τ(θ) = Σ P_i(θ) for all items i on the form
// Equating: find θ_Y such that τ_X(θ) = τ_Y(θ_Y)
type TrueScoreEquater struct {
	ReferenceForm []ItemParams // Items on the reference (target) form
	AlternateForm []ItemParams // Items on the alternate (source) form
}

// EquatingResult holds the equating table
type EquatingResult struct {
	// TrueScoreMap maps alternate form true scores to reference form true scores
	TrueScoreMap map[float64]float64
	// ScaledScoreMap maps raw scores on alternate to scaled scores
	ScaledScoreMap map[int]int
}

// Equate generates an equating table that links scores on the alternate form to the reference form.
func (e *TrueScoreEquater) Equate() (*EquatingResult, error) {
	if len(e.ReferenceForm) == 0 || len(e.AlternateForm) == 0 {
		return nil, errors.New("forms cannot be empty")
	}

	model := Model3PL{}
	
	// Generate theta grid from -4 to +4 in steps of 0.01
	thetaMin := -4.0
	thetaMax := 4.0
	step := 0.01
	nPoints := int(math.Round((thetaMax - thetaMin) / step)) + 1

	trueScoreMap := make(map[float64]float64)
	
	// Precompute true scores on both forms across theta grid
	for i := 0; i < nPoints; i++ {
		theta := thetaMin + float64(i)*step
		
		tsRef := 0.0
		for _, item := range e.ReferenceForm {
			tsRef += model.Probability(item, theta)
		}
		
		tsAlt := 0.0
		for _, item := range e.AlternateForm {
			tsAlt += model.Probability(item, theta)
		}
		
		// Map the alternate form true score directly to reference form true score at the same theta
		trueScoreMap[tsAlt] = tsRef
	}

	// For integer raw scores on alternate form, interpolate reference score
	scaledScoreMap := make(map[int]int)
	maxRawScore := len(e.AlternateForm)
	
	for raw := 0; raw <= maxRawScore; raw++ {
		rawFloat := float64(raw)
		
		// Find closest true scores on alternate form
		var bestBelow, bestAbove float64
		var refBelow, refAbove float64
		var minDiffBelow, minDiffAbove = math.MaxFloat64, math.MaxFloat64
		
		for tsAlt, tsRef := range trueScoreMap {
			if tsAlt <= rawFloat {
				if rawFloat-tsAlt < minDiffBelow {
					minDiffBelow = rawFloat - tsAlt
					bestBelow = tsAlt
					refBelow = tsRef
				}
			}
			if tsAlt >= rawFloat {
				if tsAlt-rawFloat < minDiffAbove {
					minDiffAbove = tsAlt - rawFloat
					bestAbove = tsAlt
					refAbove = tsRef
				}
			}
		}
		
		// Interpolate
		var interpolatedRef float64
		if bestAbove == bestBelow {
			interpolatedRef = refBelow
		} else {
			fraction := (rawFloat - bestBelow) / (bestAbove - bestBelow)
			interpolatedRef = refBelow + fraction*(refAbove-refBelow)
		}
		
		scaledScoreMap[raw] = int(math.Round(interpolatedRef))
	}

	return &EquatingResult{
		TrueScoreMap:   trueScoreMap,
		ScaledScoreMap: scaledScoreMap,
	}, nil
}
