package irt

import (
	"errors"
	"math"
)

// MLEEstimator implements Maximum Likelihood Estimation of ability (θ).
// Uses Newton-Raphson iteration to find θ that maximizes the log-likelihood:
// L(θ) = Σ[u_i * ln P_i(θ) + (1-u_i) * ln(1 - P_i(θ))]
//
// Newton-Raphson update:
// θ_{t+1} = θ_t - L'(θ_t) / L''(θ_t)
type MLEEstimator struct {
	MaxIterations int
	Convergence   float64 // Convergence criterion (default 0.001)
	MinTheta      float64 // Lower bound (default -4.0)
	MaxTheta      float64 // Upper bound (default +4.0)
}

// EstimationResult holds the result of ability estimation
type EstimationResult struct {
	Theta         float64 // Estimated ability
	SE            float64 // Standard error of estimation
	Converged     bool
	Iterations    int
	LogLikelihood float64
}

// NewMLEEstimator creates an MLEEstimator with default values if zero values are provided.
func NewMLEEstimator(maxIterations int, convergence, minTheta, maxTheta float64) *MLEEstimator {
	if maxIterations == 0 {
		maxIterations = 50
	}
	if convergence == 0 {
		convergence = 0.001
	}
	if minTheta == 0 && maxTheta == 0 {
		minTheta = -4.0
		maxTheta = 4.0
	}
	return &MLEEstimator{
		MaxIterations: maxIterations,
		Convergence:   convergence,
		MinTheta:      minTheta,
		MaxTheta:      maxTheta,
	}
}

// Estimate computes the Maximum Likelihood Estimate of ability (θ) for a given response pattern.
func (e *MLEEstimator) Estimate(items []ItemParams, responses []int) (*EstimationResult, error) {
	if len(items) != len(responses) {
		return nil, errors.New("length of items and responses must match")
	}

	score := 0
	for _, r := range responses {
		score += r
	}

	// Edge case handling: all correct or all incorrect
	if score == 0 {
		return &EstimationResult{
			Theta:         e.MinTheta,
			SE:            math.Inf(1),
			Converged:     false,
			Iterations:    0,
			LogLikelihood: math.Inf(-1),
		}, nil
	} else if score == len(items) {
		return &EstimationResult{
			Theta:         e.MaxTheta,
			SE:            math.Inf(1),
			Converged:     false,
			Iterations:    0,
			LogLikelihood: math.Inf(-1),
		}, nil
	}

	model := Model3PL{}
	theta := 0.0 // Initial guess
	var ll float64
	converged := false
	iterations := 0

	for i := 0; i < e.MaxIterations; i++ {
		iterations++
		firstDerivative := 0.0
		secondDerivative := 0.0
		ll = 0.0

		for j, item := range items {
			p := model.Probability(item, theta)
			q := 1.0 - p
			u := float64(responses[j])

			// Avoid log(0)
			pSafe := math.Max(1e-10, p)
			qSafe := math.Max(1e-10, q)

			ll += u*math.Log(pSafe) + (1.0-u)*math.Log(qSafe)

			// First derivative of log-likelihood (D = 1 since we use standard 1.0)
			pStar := 1.0 / (1.0 + math.Exp(-item.A*(theta-item.B)))
			dp := item.A * (p - item.C) * (1.0 - pStar) // Correct derivative for 3PL
			if item.C == 0.0 {
				dp = item.A * p * q
			}

			w := dp / (p * q)
			firstDerivative += (u - p) * w

			// Second derivative (Fisher Information is negative of expected second derivative)
			// For MLE Newton-Raphson, we often use expected information to approximate second derivative.
			info := model.Information(item, theta)
			secondDerivative -= info
		}

		if secondDerivative == 0 {
			break
		}

		delta := firstDerivative / secondDerivative
		newTheta := theta - delta

		// Bound theta
		if newTheta > e.MaxTheta {
			newTheta = e.MaxTheta
		}
		if newTheta < e.MinTheta {
			newTheta = e.MinTheta
		}

		if math.Abs(newTheta-theta) < e.Convergence {
			theta = newTheta
			converged = true
			break
		}
		theta = newTheta
	}

	info := model.TestInformation(items, theta)
	se := math.Inf(1)
	if info > 0 {
		se = 1.0 / math.Sqrt(info)
	}

	return &EstimationResult{
		Theta:         theta,
		SE:            se,
		Converged:     converged,
		Iterations:    iterations,
		LogLikelihood: ll,
	}, nil
}

// EAPEstimator implements Expected A Posteriori estimation.
// Uses numerical integration with a normal prior:
// θ_EAP = ∫θ × L(θ|u) × g(θ) dθ / ∫L(θ|u) × g(θ) dθ
// where g(θ) is the prior distribution N(μ, σ²)
type EAPEstimator struct {
	PriorMean  float64 // Default 0.0
	PriorSD    float64 // Default 1.0
	QuadPoints int     // Number of quadrature points (default 61)
	MinTheta   float64 // Default -4.0
	MaxTheta   float64 // Default 4.0
}

// NewEAPEstimator creates an EAPEstimator with default values.
func NewEAPEstimator(mean, sd float64, points int, minTheta, maxTheta float64) *EAPEstimator {
	if points == 0 {
		points = 61
	}
	if sd == 0 {
		sd = 1.0
	}
	if minTheta == 0 && maxTheta == 0 {
		minTheta = -4.0
		maxTheta = 4.0
	}
	return &EAPEstimator{
		PriorMean:  mean,
		PriorSD:    sd,
		QuadPoints: points,
		MinTheta:   minTheta,
		MaxTheta:   maxTheta,
	}
}

// Estimate computes the Expected A Posteriori Estimate of ability (θ).
func (e *EAPEstimator) Estimate(items []ItemParams, responses []int) (*EstimationResult, error) {
	if len(items) != len(responses) {
		return nil, errors.New("length of items and responses must match")
	}

	model := Model3PL{}
	step := (e.MaxTheta - e.MinTheta) / float64(e.QuadPoints-1)

	var numSum float64
	var denSum float64

	for i := 0; i < e.QuadPoints; i++ {
		theta := e.MinTheta + float64(i)*step
		
		// Prior probability density
		z := (theta - e.PriorMean) / e.PriorSD
		prior := math.Exp(-0.5*z*z) / (e.PriorSD * math.Sqrt(2*math.Pi))

		// Likelihood
		likelihood := 1.0
		for j, item := range items {
			p := model.Probability(item, theta)
			if responses[j] == 1 {
				likelihood *= p
			} else {
				likelihood *= (1.0 - p)
			}
		}

		posteriorUnnormalized := likelihood * prior
		numSum += theta * posteriorUnnormalized
		denSum += posteriorUnnormalized
	}

	if denSum == 0.0 {
		return nil, errors.New("zero probability of response pattern")
	}

	thetaEst := numSum / denSum

	// Compute posterior variance (and SE)
	var varSum float64
	for i := 0; i < e.QuadPoints; i++ {
		theta := e.MinTheta + float64(i)*step
		z := (theta - e.PriorMean) / e.PriorSD
		prior := math.Exp(-0.5*z*z) / (e.PriorSD * math.Sqrt(2*math.Pi))
		
		likelihood := 1.0
		for j, item := range items {
			p := model.Probability(item, theta)
			if responses[j] == 1 {
				likelihood *= p
			} else {
				likelihood *= (1.0 - p)
			}
		}

		posteriorUnnormalized := likelihood * prior
		diff := theta - thetaEst
		varSum += diff * diff * posteriorUnnormalized
	}

	se := math.Sqrt(varSum / denSum)

	// Compute LogLikelihood at estimated theta
	ll := 0.0
	for j, item := range items {
		p := model.Probability(item, thetaEst)
		q := 1.0 - p
		pSafe := math.Max(1e-10, p)
		qSafe := math.Max(1e-10, q)
		if responses[j] == 1 {
			ll += math.Log(pSafe)
		} else {
			ll += math.Log(qSafe)
		}
	}

	return &EstimationResult{
		Theta:         thetaEst,
		SE:            se,
		Converged:     true, // EAP always converges
		Iterations:    1,
		LogLikelihood: ll,
	}, nil
}
