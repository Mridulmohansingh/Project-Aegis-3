package irt

import "math"

// Model3PL implements the Three-Parameter Logistic IRT model.
// P(θ) = c + (1-c) / (1 + exp(-a(θ-b)))
type Model3PL struct{}

// ItemParams holds the IRT parameters for a single item
type ItemParams struct {
	A float64 // Discrimination
	B float64 // Difficulty
	C float64 // Guessing
}

// Probability returns P(θ) for the 3PL model
func (m Model3PL) Probability(params ItemParams, theta float64) float64 {
	expTerm := math.Exp(-params.A * (theta - params.B))
	return params.C + (1.0-params.C)/(1.0+expTerm)
}

// Information returns the Fisher information I(θ) for a single item
// I(θ) = a² × (P-c)² × Q / ((1-c)² × P)
func (m Model3PL) Information(params ItemParams, theta float64) float64 {
	p := m.Probability(params, theta)
	q := 1.0 - p
	if p == 0.0 {
		return 0.0
	}
	num := params.A * params.A * math.Pow(p-params.C, 2) * q
	den := math.Pow(1.0-params.C, 2) * p
	return num / den
}

// TestInformation returns the sum of item informations at the given theta
func (m Model3PL) TestInformation(items []ItemParams, theta float64) float64 {
	info := 0.0
	for _, item := range items {
		info += m.Information(item, theta)
	}
	return info
}

// TestInformationFunction returns TIF values at multiple theta points
func (m Model3PL) TestInformationFunction(items []ItemParams, thetas []float64) map[float64]float64 {
	tif := make(map[float64]float64)
	for _, theta := range thetas {
		tif[theta] = m.TestInformation(items, theta)
	}
	return tif
}
