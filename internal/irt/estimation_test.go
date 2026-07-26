package irt_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/aegis-platform/aegis/internal/irt"
	"github.com/stretchr/testify/assert"
)

func generateTestItems(n int) []irt.ItemParams {
	items := make([]irt.ItemParams, n)
	for i := 0; i < n; i++ {
		// varying difficulty from -2 to 2
		b := -2.0 + float64(i)*4.0/float64(n-1)
		items[i] = irt.ItemParams{A: 1.0, B: b, C: 0.2}
	}
	return items
}

func generateResponses(items []irt.ItemParams, theta float64) []int {
	model := irt.Model3PL{}
	responses := make([]int, len(items))
	rng := rand.New(rand.NewSource(13))
	for i, item := range items {
		p := model.Probability(item, theta)
		if rng.Float64() < p {
			responses[i] = 1
		} else {
			responses[i] = 0
		}
	}
	return responses
}

func TestMLEEstimation(t *testing.T) {
	items := generateTestItems(30)
	trueTheta := 1.0
	responses := generateResponses(items, trueTheta)

	estimator := irt.NewMLEEstimator(50, 0.001, -4.0, 4.0)
	res, err := estimator.Estimate(items, responses)

	assert.NoError(t, err)
	assert.True(t, res.Converged)
	assert.InDelta(t, trueTheta, res.Theta, 0.3)
}

func TestMLEAllCorrect(t *testing.T) {
	items := generateTestItems(10)
	responses := []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}

	estimator := irt.NewMLEEstimator(50, 0.001, -4.0, 4.0)
	res, err := estimator.Estimate(items, responses)

	assert.NoError(t, err)
	assert.False(t, res.Converged)
	assert.Equal(t, 4.0, res.Theta)
	assert.True(t, math.IsInf(res.SE, 1))
}

func TestMLEAllIncorrect(t *testing.T) {
	items := generateTestItems(10)
	responses := []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	estimator := irt.NewMLEEstimator(50, 0.001, -4.0, 4.0)
	res, err := estimator.Estimate(items, responses)

	assert.NoError(t, err)
	assert.False(t, res.Converged)
	assert.Equal(t, -4.0, res.Theta)
	assert.True(t, math.IsInf(res.SE, 1))
}

func TestEAPEstimation(t *testing.T) {
	items := generateTestItems(30)
	trueTheta := 1.0
	responses := generateResponses(items, trueTheta)

	estimator := irt.NewEAPEstimator(0.0, 1.0, 61, -4.0, 4.0)
	res, err := estimator.Estimate(items, responses)

	assert.NoError(t, err)
	assert.True(t, res.Converged)
	assert.InDelta(t, trueTheta, res.Theta, 0.4)
}

func TestEAPPriorEffect(t *testing.T) {
	items := generateTestItems(30)
	trueTheta := 2.0
	responses := generateResponses(items, trueTheta)

	est1 := irt.NewEAPEstimator(0.0, 1.0, 61, -4.0, 4.0)
	res1, _ := est1.Estimate(items, responses)

	est2 := irt.NewEAPEstimator(2.0, 1.0, 61, -4.0, 4.0)
	res2, _ := est2.Estimate(items, responses)

	// Prior at 2.0 should result in an estimate closer to 2.0 than prior at 0.0
	diff1 := math.Abs(res1.Theta - 2.0)
	diff2 := math.Abs(res2.Theta - 2.0)
	assert.True(t, diff2 < diff1)
}

func TestConvergence(t *testing.T) {
	items := generateTestItems(30)
	responses := generateResponses(items, 1.0)

	estimator := irt.NewMLEEstimator(1, 0.001, -4.0, 4.0)
	res, _ := estimator.Estimate(items, responses)

	assert.False(t, res.Converged)
	assert.Equal(t, 1, res.Iterations)
}

func TestStandardError(t *testing.T) {
	items10 := generateTestItems(10)
	responses10 := generateResponses(items10, 0.0)

	items50 := generateTestItems(50)
	responses50 := generateResponses(items50, 0.0)

	estimator := irt.NewMLEEstimator(50, 0.001, -4.0, 4.0)
	res10, _ := estimator.Estimate(items10, responses10)
	res50, _ := estimator.Estimate(items50, responses50)

	// SE should be smaller with more items
	assert.True(t, res50.SE < res10.SE)
}
