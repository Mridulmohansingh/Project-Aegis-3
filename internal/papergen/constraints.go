package papergen

import (
	"fmt"

	"github.com/aegis-platform/aegis/internal/domain/blueprint"
	"github.com/aegis-platform/aegis/internal/domain/item"
)

// BuildConstraints translates a blueprint into a MIPModel
func BuildConstraints(bp *blueprint.Blueprint, items []*item.Item, enemies []item.ItemEnemy) *MIPModel {
	m := &MIPModel{
		Variables:   make([]Variable, len(items)),
		Constraints: make([]Constraint, 0),
		Objective:   Minimize,
	}

	// 1. Create variables
	for i, itm := range items {
		m.Variables[i] = Variable{
			Index:    i,
			ItemID:   itm.ID,
			Lower:    0,
			Upper:    1,
			ObjCoeff: 0, // Set later based on TIF deviation
		}
	}

	// 2. Total items constraint: Σx_i = n_total
	totalConstraint := Constraint{
		Name:   "total_items",
		Type:   EQ,
		RHS:    float64(bp.TotalItems),
		Coeffs: make(map[int]float64),
	}
	for i := range m.Variables {
		totalConstraint.Coeffs[i] = 1.0
	}
	m.Constraints = append(m.Constraints, totalConstraint)

	// 3. Chapter coverage
	for _, cConstraint := range bp.Constraints.Chapters {
		chapterIDStr := cConstraint.ChapterID.String()
		minConstraint := Constraint{
			Name:   "chapter_min_" + chapterIDStr,
			Type:   GEQ,
			RHS:    float64(cConstraint.MinItems),
			Coeffs: make(map[int]float64),
		}
		maxConstraint := Constraint{
			Name:   "chapter_max_" + chapterIDStr,
			Type:   LEQ,
			RHS:    float64(cConstraint.MaxItems),
			Coeffs: make(map[int]float64),
		}
		for i, itm := range items {
			if itm.ChapterID == cConstraint.ChapterID {
				minConstraint.Coeffs[i] = 1.0
				maxConstraint.Coeffs[i] = 1.0
			}
		}
		m.Constraints = append(m.Constraints, minConstraint, maxConstraint)
	}

	// 4. Difficulty distribution
	diffTargets := bp.Constraints.Difficulty.Distribution.TargetCounts(bp.TotalItems)
	for levelStr, targetCount := range diffTargets {
		diffConstraint := Constraint{
			Name:   "difficulty_" + levelStr,
			Type:   EQ,
			RHS:    float64(targetCount),
			Coeffs: make(map[int]float64),
		}
		for i, itm := range items {
			if string(itm.DifficultyLevel) == levelStr {
				diffConstraint.Coeffs[i] = 1.0
			}
		}
		m.Constraints = append(m.Constraints, diffConstraint)
	}

	// 5. Cognitive levels distribution
	cogTargets := bp.Constraints.CognitiveLevels.TargetCounts(bp.TotalItems)
	for levelStr, targetCount := range cogTargets {
		cogConstraint := Constraint{
			Name:   "cognitive_" + levelStr,
			Type:   EQ,
			RHS:    float64(targetCount),
			Coeffs: make(map[int]float64),
		}
		for i, itm := range items {
			if string(itm.CognitiveLevel) == levelStr {
				cogConstraint.Coeffs[i] = 1.0
			}
		}
		m.Constraints = append(m.Constraints, cogConstraint)
	}

	// 6. Time budget: Σ(x_i * t_i) ≤ T_max
	timeConstraint := Constraint{
		Name:   "time_budget",
		Type:   LEQ,
		RHS:    float64(bp.Constraints.TimeBudgetSecs),
		Coeffs: make(map[int]float64),
	}
	for i, itm := range items {
		timeConstraint.Coeffs[i] = float64(itm.EstimatedTimeSecs)
	}
	m.Constraints = append(m.Constraints, timeConstraint)

	// 7. Enemy items: x_a + x_b ≤ 1 for each enemy pair
	for idx, enemy := range enemies {
		enemyConstraint := Constraint{
			Name:   fmt.Sprintf("enemy_%d", idx),
			Type:   LEQ,
			RHS:    1.0,
			Coeffs: make(map[int]float64),
		}
		for i, itm := range items {
			if itm.ID == enemy.ItemAID || itm.ID == enemy.ItemBID {
				enemyConstraint.Coeffs[i] = 1.0
			}
		}
		m.Constraints = append(m.Constraints, enemyConstraint)
	}

	// 8. Exposure control (force x_i = 0 for over-exposed items by altering upper bound)
	for i, itm := range items {
		if itm.Exposure.ExposureIndex > bp.Constraints.MaxExposureIndex {
			m.Variables[i].Upper = 0.0 // Pre-solve fixing
		}
	}

	return m
}
