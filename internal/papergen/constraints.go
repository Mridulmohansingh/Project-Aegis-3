package papergen

import (
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
	for chapterID, rules := range bp.ChapterConstraints {
		minConstraint := Constraint{
			Name:   "chapter_min_" + chapterID,
			Type:   GEQ,
			RHS:    float64(rules.Min),
			Coeffs: make(map[int]float64),
		}
		maxConstraint := Constraint{
			Name:   "chapter_max_" + chapterID,
			Type:   LEQ,
			RHS:    float64(rules.Max),
			Coeffs: make(map[int]float64),
		}
		for i, itm := range items {
			if itm.ChapterID == chapterID {
				minConstraint.Coeffs[i] = 1.0
				maxConstraint.Coeffs[i] = 1.0
			}
		}
		m.Constraints = append(m.Constraints, minConstraint, maxConstraint)
	}

	// 4. Difficulty distribution
	for diffLevel, rules := range bp.DifficultyConstraints {
		minConstraint := Constraint{
			Name:   "difficulty_min_" + diffLevel,
			Type:   GEQ,
			RHS:    float64(rules.Min),
			Coeffs: make(map[int]float64),
		}
		maxConstraint := Constraint{
			Name:   "difficulty_max_" + diffLevel,
			Type:   LEQ,
			RHS:    float64(rules.Max),
			Coeffs: make(map[int]float64),
		}
		for i, itm := range items {
			if itm.DifficultyLevel == diffLevel {
				minConstraint.Coeffs[i] = 1.0
				maxConstraint.Coeffs[i] = 1.0
			}
		}
		m.Constraints = append(m.Constraints, minConstraint, maxConstraint)
	}

	// 5. Cognitive levels
	for cogLevel, rules := range bp.CognitiveConstraints {
		minConstraint := Constraint{
			Name:   "cognitive_min_" + cogLevel,
			Type:   GEQ,
			RHS:    float64(rules.Min),
			Coeffs: make(map[int]float64),
		}
		maxConstraint := Constraint{
			Name:   "cognitive_max_" + cogLevel,
			Type:   LEQ,
			RHS:    float64(rules.Max),
			Coeffs: make(map[int]float64),
		}
		for i, itm := range items {
			if itm.CognitiveLevel == cogLevel {
				minConstraint.Coeffs[i] = 1.0
				maxConstraint.Coeffs[i] = 1.0
			}
		}
		m.Constraints = append(m.Constraints, minConstraint, maxConstraint)
	}

	// 6. Time budget: Σ(x_i * t_i) ≤ T_max
	timeConstraint := Constraint{
		Name:   "time_budget",
		Type:   LEQ,
		RHS:    float64(bp.MaxTimeSecs),
		Coeffs: make(map[int]float64),
	}
	for i, itm := range items {
		timeConstraint.Coeffs[i] = float64(itm.EstimatedTimeSecs)
	}
	m.Constraints = append(m.Constraints, timeConstraint)

	// 7. Enemy items: x_a + x_b ≤ 1 for each enemy pair
	for idx, enemy := range enemies {
		enemyConstraint := Constraint{
			Name:   "enemy_" + string(rune(idx)), // simplistic naming
			Type:   LEQ,
			RHS:    1.0,
			Coeffs: make(map[int]float64),
		}
		for i, itm := range items {
			if itm.ID == enemy.ItemID1 || itm.ID == enemy.ItemID2 {
				enemyConstraint.Coeffs[i] = 1.0
			}
		}
		m.Constraints = append(m.Constraints, enemyConstraint)
	}

	// 8. Exposure control (force x_i = 0 for over-exposed items by altering upper bound)
	for i, itm := range items {
		if itm.ExposureRate > bp.MaxExposureRate {
			m.Variables[i].Upper = 0.0 // Pre-solve fixing
		}
	}

	// 9. Answer key balance (simplistic approach: A, B, C, D roughly equal)
	// Usually items have a fixed correct option, but let's assume we map constraints
	// Here we skip strict answer key balance for brevity in this snippet.

	return m
}
