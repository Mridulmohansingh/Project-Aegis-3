// Package blueprint defines the Blueprint aggregate for test assembly constraints.
//
// A Blueprint specifies the rules for automated test assembly (ATA):
// chapter/topic coverage, difficulty distribution, cognitive level balance,
// time budget, exposure limits, and target test information function.
//
// The Paper Generation Engine uses these blueprints as constraints in its
// Mixed Integer Programming (MIP) formulation to produce psychometrically
// equivalent test forms.
package blueprint

import (
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

// ──────────────────────────────────────────────
//  Blueprint Aggregate
// ──────────────────────────────────────────────

// Blueprint is the aggregate root defining test assembly constraints.
type Blueprint struct {
	ID             uuid.UUID            `json:"id" db:"id"`
	OrganizationID uuid.UUID            `json:"organization_id" db:"organization_id"`
	Name           string               `json:"name" db:"name"`
	SubjectID      uuid.UUID            `json:"subject_id" db:"subject_id"`
	TotalItems     int                  `json:"total_items" db:"total_items"`
	Constraints    BlueprintConstraints `json:"constraints" db:"constraints"`
	Version        int                  `json:"version" db:"version"`
	CreatedAt      time.Time            `json:"created_at" db:"created_at"`
	CreatedBy      uuid.UUID            `json:"created_by" db:"created_by"`
	UpdatedAt      time.Time            `json:"updated_at" db:"updated_at"`
	UpdatedBy      uuid.UUID            `json:"updated_by" db:"updated_by"`
}

// BlueprintConstraints holds all constraint specifications for test assembly.
type BlueprintConstraints struct {
	// Chapters specifies the item distribution across chapters.
	Chapters []ChapterConstraint `json:"chapters"`
	// Topics specifies optional topic-level constraints within chapters.
	Topics []TopicConstraint `json:"topics,omitempty"`
	// Difficulty specifies the difficulty distribution targets.
	Difficulty DifficultyConstraint `json:"difficulty"`
	// CognitiveLevels specifies the Bloom's taxonomy level distribution.
	CognitiveLevels CognitiveLevelConstraint `json:"cognitive_levels"`
	// TimeBudgetSecs is the maximum total estimated solving time in seconds.
	TimeBudgetSecs int `json:"time_budget_secs"`
	// MaxExposureIndex is the maximum allowed exposure index for selected items.
	MaxExposureIndex float64 `json:"max_exposure_index"`
	// AnswerKeyBalance specifies constraints on answer key distribution.
	AnswerKeyBalance AnswerKeyConstraint `json:"answer_key_balance"`
	// InformationTargets specifies target test information function values at key theta points.
	InformationTargets []InformationTarget `json:"information_targets"`
	// EnemyItems lists item pairs that must not co-occur on the same form.
	// These are loaded dynamically from the item_enemies table.
}

// ChapterConstraint specifies how many items should come from a specific chapter.
type ChapterConstraint struct {
	ChapterID uuid.UUID `json:"chapter_id"`
	MinItems  int       `json:"min_items"`
	MaxItems  int       `json:"max_items"`
	// Weight is the relative importance of this chapter in the blueprint (should sum to 1.0).
	Weight float64 `json:"weight"`
}

// TopicConstraint specifies item requirements within a specific topic.
type TopicConstraint struct {
	TopicID  uuid.UUID `json:"topic_id"`
	MinItems int       `json:"min_items"`
	MaxItems int       `json:"max_items"`
}

// DifficultyConstraint specifies the target difficulty distribution.
type DifficultyConstraint struct {
	// TargetMeanB is the target mean of IRT difficulty parameters (logits).
	TargetMeanB float64 `json:"target_mean_b"`
	// TargetStdB is the target standard deviation of IRT difficulty parameters.
	TargetStdB float64 `json:"target_std_b"`
	// Distribution specifies the proportion of items at each difficulty level.
	Distribution DifficultyDistribution `json:"distribution"`
}

// DifficultyDistribution maps difficulty levels to target proportions (must sum to 1.0).
type DifficultyDistribution struct {
	Easy     float64 `json:"EASY"`
	Medium   float64 `json:"MEDIUM"`
	Hard     float64 `json:"HARD"`
	VeryHard float64 `json:"VERY_HARD"`
}

// Validate checks that the distribution sums to approximately 1.0.
func (d DifficultyDistribution) Validate() error {
	sum := d.Easy + d.Medium + d.Hard + d.VeryHard
	if math.Abs(sum-1.0) > 0.01 {
		return fmt.Errorf("difficulty distribution must sum to 1.0, got %.3f", sum)
	}
	return nil
}

// TargetCounts returns the target number of items for each difficulty level given a total.
func (d DifficultyDistribution) TargetCounts(total int) map[string]int {
	return map[string]int{
		"EASY":      int(math.Round(d.Easy * float64(total))),
		"MEDIUM":    int(math.Round(d.Medium * float64(total))),
		"HARD":      int(math.Round(d.Hard * float64(total))),
		"VERY_HARD": int(math.Round(d.VeryHard * float64(total))),
	}
}

// CognitiveLevelConstraint specifies the Bloom's taxonomy level distribution.
type CognitiveLevelConstraint struct {
	Remember   float64 `json:"REMEMBER"`
	Understand float64 `json:"UNDERSTAND"`
	Apply      float64 `json:"APPLY"`
	Analyze    float64 `json:"ANALYZE"`
	Evaluate   float64 `json:"EVALUATE"`
	Create     float64 `json:"CREATE"`
}

// Validate checks that the distribution sums to approximately 1.0.
func (c CognitiveLevelConstraint) Validate() error {
	sum := c.Remember + c.Understand + c.Apply + c.Analyze + c.Evaluate + c.Create
	if math.Abs(sum-1.0) > 0.01 {
		return fmt.Errorf("cognitive level distribution must sum to 1.0, got %.3f", sum)
	}
	return nil
}

// TargetCounts returns the target number of items for each cognitive level given a total.
func (c CognitiveLevelConstraint) TargetCounts(total int) map[string]int {
	return map[string]int{
		"REMEMBER":   int(math.Round(c.Remember * float64(total))),
		"UNDERSTAND": int(math.Round(c.Understand * float64(total))),
		"APPLY":      int(math.Round(c.Apply * float64(total))),
		"ANALYZE":    int(math.Round(c.Analyze * float64(total))),
		"EVALUATE":   int(math.Round(c.Evaluate * float64(total))),
		"CREATE":     int(math.Round(c.Create * float64(total))),
	}
}

// AnswerKeyConstraint prevents predictable answer key patterns.
type AnswerKeyConstraint struct {
	// MaxConsecutiveSame is the maximum number of consecutive questions with the same correct answer.
	MaxConsecutiveSame int `json:"max_consecutive_same"`
	// MaxAnyOptionPct is the maximum percentage any single option can be the correct answer.
	MaxAnyOptionPct float64 `json:"max_any_option_pct"`
}

// InformationTarget specifies a target for the Test Information Function.
// The TIF should meet or exceed the MinInfo value at the given theta point.
type InformationTarget struct {
	// Theta is the ability level on the IRT scale.
	Theta float64 `json:"theta"`
	// MinInfo is the minimum required test information at this theta.
	MinInfo float64 `json:"min_info"`
	// Weight is the importance of meeting this target in the objective function.
	Weight float64 `json:"weight"`
}

// ──────────────────────────────────────────────
//  Validation
// ──────────────────────────────────────────────

// Validate performs comprehensive validation of the blueprint constraints.
func (b *Blueprint) Validate() error {
	if b.TotalItems < 1 {
		return fmt.Errorf("total_items must be at least 1")
	}
	if b.Name == "" {
		return fmt.Errorf("blueprint name is required")
	}

	// Validate chapter constraints
	totalMinItems := 0
	totalMaxItems := 0
	totalWeight := 0.0
	for _, ch := range b.Constraints.Chapters {
		if ch.MinItems > ch.MaxItems {
			return fmt.Errorf("chapter %s: min_items (%d) > max_items (%d)", ch.ChapterID, ch.MinItems, ch.MaxItems)
		}
		totalMinItems += ch.MinItems
		totalMaxItems += ch.MaxItems
		totalWeight += ch.Weight
	}
	if totalMinItems > b.TotalItems {
		return fmt.Errorf("sum of chapter min_items (%d) exceeds total_items (%d)", totalMinItems, b.TotalItems)
	}
	if totalMaxItems < b.TotalItems {
		return fmt.Errorf("sum of chapter max_items (%d) is less than total_items (%d)", totalMaxItems, b.TotalItems)
	}
	if len(b.Constraints.Chapters) > 0 && math.Abs(totalWeight-1.0) > 0.01 {
		return fmt.Errorf("chapter weights must sum to 1.0, got %.3f", totalWeight)
	}

	// Validate difficulty distribution
	if err := b.Constraints.Difficulty.Distribution.Validate(); err != nil {
		return fmt.Errorf("difficulty constraint: %w", err)
	}

	// Validate cognitive level distribution
	if err := b.Constraints.CognitiveLevels.Validate(); err != nil {
		return fmt.Errorf("cognitive level constraint: %w", err)
	}

	// Validate time budget
	if b.Constraints.TimeBudgetSecs <= 0 {
		return fmt.Errorf("time_budget_secs must be positive")
	}

	// Validate exposure index
	if b.Constraints.MaxExposureIndex <= 0 || b.Constraints.MaxExposureIndex > 1.0 {
		return fmt.Errorf("max_exposure_index must be in (0, 1.0], got %.3f", b.Constraints.MaxExposureIndex)
	}

	// Validate information targets
	for i, t := range b.Constraints.InformationTargets {
		if t.MinInfo < 0 {
			return fmt.Errorf("information_target[%d]: min_info must be non-negative", i)
		}
		if t.Weight <= 0 {
			return fmt.Errorf("information_target[%d]: weight must be positive", i)
		}
	}

	return nil
}

// NewBlueprint creates a new Blueprint with the given specifications.
func NewBlueprint(
	orgID uuid.UUID,
	name string,
	subjectID uuid.UUID,
	totalItems int,
	constraints BlueprintConstraints,
	createdBy uuid.UUID,
) (*Blueprint, error) {
	now := time.Now().UTC()
	bp := &Blueprint{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           name,
		SubjectID:      subjectID,
		TotalItems:     totalItems,
		Constraints:    constraints,
		Version:        1,
		CreatedAt:      now,
		CreatedBy:      createdBy,
		UpdatedAt:      now,
		UpdatedBy:      createdBy,
	}
	if err := bp.Validate(); err != nil {
		return nil, err
	}
	return bp, nil
}

// ──────────────────────────────────────────────
//  Repository Interface
// ──────────────────────────────────────────────

// BlueprintRepository defines persistence operations for blueprints.
type BlueprintRepository interface {
	Create(blueprint *Blueprint) error
	GetByID(orgID, id uuid.UUID) (*Blueprint, error)
	Update(blueprint *Blueprint) error
	List(orgID uuid.UUID, cursor string, limit int) ([]*Blueprint, string, error)
	Delete(orgID, id uuid.UUID) error
}
