package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/aegis-platform/aegis/internal/domain/item"
	"github.com/aegis-platform/aegis/pkg/apperrors"
	"github.com/aegis-platform/aegis/pkg/pagination"
)

// ItemRepository implements item.ItemRepository using PostgreSQL.
type ItemRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewItemRepository creates a new PostgreSQL-backed item repository.
func NewItemRepository(pool *pgxpool.Pool, logger *zap.Logger) *ItemRepository {
	return &ItemRepository{
		pool:   pool,
		logger: logger.With(zap.String("component", "item_repository")),
	}
}

// Create persists a new item to the database.
func (r *ItemRepository) Create(ctx context.Context, i *item.Item) error {
	contentJSON, err := json.Marshal(i.Content)
	if err != nil {
		return apperrors.NewInternal("failed to marshal item content", err)
	}

	markingJSON, err := json.Marshal(i.MarkingScheme)
	if err != nil {
		return apperrors.NewInternal("failed to marshal marking scheme", err)
	}

	query := `
		INSERT INTO items (
			id, organization_id, external_id, item_type, status,
			subject_id, chapter_id, topic_id, sub_topic_id, learning_outcome_id,
			question_content, answer_key, solution, marking_scheme, estimated_time_secs,
			difficulty_level, cognitive_level,
			irt_a, irt_b, irt_c, irt_se_a, irt_se_b, irt_se_c, irt_info_at_0,
			calibration_sample, calibration_date,
			p_value, discrimination_idx, point_biserial, distractor_analysis,
			exposure_count, max_exposure, exposure_index, last_used_at, cooldown_until,
			primary_language, variant_group_id, parent_item_id,
			author_id, author_signature,
			tags, version, created_at, created_by, updated_at, updated_by
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15,
			$16, $17,
			$18, $19, $20, $21, $22, $23, $24,
			$25, $26,
			$27, $28, $29, $30,
			$31, $32, $33, $34, $35,
			$36, $37, $38,
			$39, $40,
			$41, $42, $43, $44, $45, $46
		)`

	var irtA, irtB, irtC, irtSEA, irtSEB, irtSEC, irtInfo *float64
	var calibSample *int
	var calibDate *time.Time
	var pVal, discIdx, ptBis *float64
	var distAnalysis []byte

	if i.IRTParams != nil {
		irtA = &i.IRTParams.A
		irtB = &i.IRTParams.B
		irtC = &i.IRTParams.C
		irtSEA = &i.IRTParams.SEA
		irtSEB = &i.IRTParams.SEB
		irtSEC = &i.IRTParams.SEC
		irtInfo = &i.IRTParams.InformationAtZero
		calibSample = &i.IRTParams.CalibrationSampleSize
		calibDate = i.IRTParams.CalibrationDate
	}

	if i.ClassicalStats != nil {
		pVal = &i.ClassicalStats.PValue
		discIdx = &i.ClassicalStats.DiscriminationIndex
		ptBis = &i.ClassicalStats.PointBiserial
		if i.ClassicalStats.DistractorAnalysis != nil {
			distAnalysis, _ = json.Marshal(i.ClassicalStats.DistractorAnalysis)
		}
	}

	_, err = r.pool.Exec(ctx, query,
		i.ID, i.OrganizationID, i.ExternalID, i.Type, i.Status,
		i.SubjectID, i.ChapterID, i.TopicID, i.SubTopicID, i.LearningOutcomeID,
		contentJSON, i.AnswerKey, i.Solution, markingJSON, i.EstimatedTimeSecs,
		nullableString(string(i.DifficultyLevel)), nullableString(string(i.CognitiveLevel)),
		irtA, irtB, irtC, irtSEA, irtSEB, irtSEC, irtInfo,
		calibSample, calibDate,
		pVal, discIdx, ptBis, distAnalysis,
		i.Exposure.ExposureCount, i.Exposure.MaxExposure, i.Exposure.ExposureIndex,
		i.Exposure.LastUsedAt, i.Exposure.CooldownUntil,
		i.PrimaryLanguage, i.VariantGroupID, i.ParentItemID,
		i.Approval.AuthorID, i.Approval.AuthorSignature,
		i.Tags, i.Version, i.CreatedAt, i.CreatedBy, i.UpdatedAt, i.UpdatedBy,
	)

	if err != nil {
		if strings.Contains(err.Error(), "uq_item_external") {
			return apperrors.NewConflict(fmt.Sprintf("item with external ID '%s' already exists", i.ExternalID))
		}
		return apperrors.NewInternal("failed to create item", err)
	}

	r.logger.Info("item created",
		zap.String("item_id", i.ID.String()),
		zap.String("external_id", i.ExternalID),
		zap.String("status", i.Status.String()),
	)

	return nil
}

// GetByID retrieves an item by its UUID within an organization.
func (r *ItemRepository) GetByID(ctx context.Context, orgID, id uuid.UUID) (*item.Item, error) {
	query := `
		SELECT
			id, organization_id, external_id, item_type, status,
			subject_id, chapter_id, topic_id, sub_topic_id, learning_outcome_id,
			question_content, answer_key, solution, marking_scheme, estimated_time_secs,
			difficulty_level, cognitive_level,
			irt_a, irt_b, irt_c, irt_se_a, irt_se_b, irt_se_c, irt_info_at_0,
			calibration_sample, calibration_date,
			p_value, discrimination_idx, point_biserial, distractor_analysis,
			exposure_count, max_exposure, exposure_index, last_used_at, cooldown_until,
			primary_language, variant_group_id, parent_item_id,
			author_id, author_signature,
			reviewer_id, reviewer_signature, reviewer_decision, reviewed_at,
			psychometrician_id, psychometrician_sig,
			approver_id, approver_signature, approved_at,
			tags, version, created_at, created_by, updated_at, updated_by, deleted_at
		FROM items
		WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL`

	return r.scanItem(ctx, query, id, orgID)
}

// GetByExternalID retrieves an item by its external identifier.
func (r *ItemRepository) GetByExternalID(ctx context.Context, orgID uuid.UUID, externalID string) (*item.Item, error) {
	query := `
		SELECT
			id, organization_id, external_id, item_type, status,
			subject_id, chapter_id, topic_id, sub_topic_id, learning_outcome_id,
			question_content, answer_key, solution, marking_scheme, estimated_time_secs,
			difficulty_level, cognitive_level,
			irt_a, irt_b, irt_c, irt_se_a, irt_se_b, irt_se_c, irt_info_at_0,
			calibration_sample, calibration_date,
			p_value, discrimination_idx, point_biserial, distractor_analysis,
			exposure_count, max_exposure, exposure_index, last_used_at, cooldown_until,
			primary_language, variant_group_id, parent_item_id,
			author_id, author_signature,
			reviewer_id, reviewer_signature, reviewer_decision, reviewed_at,
			psychometrician_id, psychometrician_sig,
			approver_id, approver_signature, approved_at,
			tags, version, created_at, created_by, updated_at, updated_by, deleted_at
		FROM items
		WHERE external_id = $1 AND organization_id = $2 AND deleted_at IS NULL`

	return r.scanItem(ctx, query, externalID, orgID)
}

// Update persists changes to an existing item with optimistic locking.
func (r *ItemRepository) Update(ctx context.Context, i *item.Item) error {
	contentJSON, _ := json.Marshal(i.Content)
	markingJSON, _ := json.Marshal(i.MarkingScheme)

	query := `
		UPDATE items SET
			status = $3, question_content = $4, answer_key = $5, solution = $6,
			marking_scheme = $7, estimated_time_secs = $8,
			difficulty_level = $9, cognitive_level = $10,
			irt_a = $11, irt_b = $12, irt_c = $13,
			irt_se_a = $14, irt_se_b = $15, irt_se_c = $16, irt_info_at_0 = $17,
			calibration_sample = $18, calibration_date = $19,
			exposure_count = $20, exposure_index = $21, last_used_at = $22, cooldown_until = $23,
			reviewer_id = $24, reviewer_signature = $25, reviewer_decision = $26, reviewed_at = $27,
			psychometrician_id = $28, psychometrician_sig = $29,
			approver_id = $30, approver_signature = $31, approved_at = $32,
			tags = $33, version = $34, updated_at = $35, updated_by = $36
		WHERE id = $1 AND organization_id = $2 AND version = $37 AND deleted_at IS NULL`

	var irtA, irtB, irtC, irtSEA, irtSEB, irtSEC, irtInfo *float64
	var calibSample *int
	var calibDate *time.Time

	if i.IRTParams != nil {
		irtA = &i.IRTParams.A
		irtB = &i.IRTParams.B
		irtC = &i.IRTParams.C
		irtSEA = &i.IRTParams.SEA
		irtSEB = &i.IRTParams.SEB
		irtSEC = &i.IRTParams.SEC
		irtInfo = &i.IRTParams.InformationAtZero
		calibSample = &i.IRTParams.CalibrationSampleSize
		calibDate = i.IRTParams.CalibrationDate
	}

	tag, err := r.pool.Exec(ctx, query,
		i.ID, i.OrganizationID,
		i.Status, contentJSON, i.AnswerKey, i.Solution,
		markingJSON, i.EstimatedTimeSecs,
		nullableString(string(i.DifficultyLevel)), nullableString(string(i.CognitiveLevel)),
		irtA, irtB, irtC, irtSEA, irtSEB, irtSEC, irtInfo,
		calibSample, calibDate,
		i.Exposure.ExposureCount, i.Exposure.ExposureIndex,
		i.Exposure.LastUsedAt, i.Exposure.CooldownUntil,
		i.Approval.ReviewerID, i.Approval.ReviewerSignature,
		nullableString(string(i.Approval.ReviewerDecision)), i.Approval.ReviewedAt,
		i.Approval.PsychometricianID, i.Approval.PsychometricianSig,
		i.Approval.ApproverID, i.Approval.ApproverSignature, i.Approval.ApprovedAt,
		i.Tags, i.Version, i.UpdatedAt, i.UpdatedBy,
		i.Version-1, // Optimistic lock: expect previous version
	)

	if err != nil {
		return apperrors.NewInternal("failed to update item", err)
	}

	if tag.RowsAffected() == 0 {
		return apperrors.NewConflict("item was modified by another process (version conflict)")
	}

	return nil
}

// SoftDelete marks an item as deleted.
func (r *ItemRepository) SoftDelete(ctx context.Context, orgID, id, deletedBy uuid.UUID) error {
	query := `
		UPDATE items
		SET deleted_at = NOW(), updated_by = $3, updated_at = NOW()
		WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL`

	tag, err := r.pool.Exec(ctx, query, id, orgID, deletedBy)
	if err != nil {
		return apperrors.NewInternal("failed to soft-delete item", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("Item", id)
	}
	return nil
}

// List retrieves a filtered, paginated list of items.
func (r *ItemRepository) List(ctx context.Context, filter item.ItemFilter, cursor string, limit int) ([]*item.Item, string, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	conditions = append(conditions, "deleted_at IS NULL")

	if filter.OrganizationID != nil {
		conditions = append(conditions, fmt.Sprintf("organization_id = $%d", argIdx))
		args = append(args, *filter.OrganizationID)
		argIdx++
	}
	if filter.SubjectID != nil {
		conditions = append(conditions, fmt.Sprintf("subject_id = $%d", argIdx))
		args = append(args, *filter.SubjectID)
		argIdx++
	}
	if filter.ChapterID != nil {
		conditions = append(conditions, fmt.Sprintf("chapter_id = $%d", argIdx))
		args = append(args, *filter.ChapterID)
		argIdx++
	}
	if filter.TopicID != nil {
		conditions = append(conditions, fmt.Sprintf("topic_id = $%d", argIdx))
		args = append(args, *filter.TopicID)
		argIdx++
	}
	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, string(*filter.Status))
		argIdx++
	}
	if filter.DifficultyLevel != nil {
		conditions = append(conditions, fmt.Sprintf("difficulty_level = $%d", argIdx))
		args = append(args, string(*filter.DifficultyLevel))
		argIdx++
	}
	if filter.CognitiveLevel != nil {
		conditions = append(conditions, fmt.Sprintf("cognitive_level = $%d", argIdx))
		args = append(args, string(*filter.CognitiveLevel))
		argIdx++
	}
	if filter.ItemType != nil {
		conditions = append(conditions, fmt.Sprintf("item_type = $%d", argIdx))
		args = append(args, string(*filter.ItemType))
		argIdx++
	}
	if filter.HasIRTParams != nil && *filter.HasIRTParams {
		conditions = append(conditions, "irt_a IS NOT NULL")
	}
	if filter.MaxExposureIndex != nil {
		conditions = append(conditions, fmt.Sprintf("exposure_index <= $%d", argIdx))
		args = append(args, *filter.MaxExposureIndex)
		argIdx++
	}

	// Cursor-based pagination
	if cursor != "" {
		cursorData, err := pagination.DecodeCursor(cursor)
		if err != nil {
			return nil, "", apperrors.NewBadRequest("invalid cursor: " + err.Error())
		}
		conditions = append(conditions, fmt.Sprintf("(created_at, id) < ($%d, $%d)", argIdx, argIdx+1))
		args = append(args, cursorData.CreatedAt, cursorData.ID)
		argIdx += 2
	}

	whereClause := "WHERE " + strings.Join(conditions, " AND ")

	query := fmt.Sprintf(`
		SELECT
			id, organization_id, external_id, item_type, status,
			subject_id, chapter_id, topic_id, sub_topic_id, learning_outcome_id,
			question_content, answer_key, solution, marking_scheme, estimated_time_secs,
			difficulty_level, cognitive_level,
			irt_a, irt_b, irt_c, irt_se_a, irt_se_b, irt_se_c, irt_info_at_0,
			calibration_sample, calibration_date,
			p_value, discrimination_idx, point_biserial, distractor_analysis,
			exposure_count, max_exposure, exposure_index, last_used_at, cooldown_until,
			primary_language, variant_group_id, parent_item_id,
			author_id, author_signature,
			reviewer_id, reviewer_signature, reviewer_decision, reviewed_at,
			psychometrician_id, psychometrician_sig,
			approver_id, approver_signature, approved_at,
			tags, version, created_at, created_by, updated_at, updated_by, deleted_at
		FROM items
		%s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d`, whereClause, argIdx)

	args = append(args, limit+1) // Fetch one extra to detect hasMore

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", apperrors.NewInternal("failed to list items", err)
	}
	defer rows.Close()

	var items []*item.Item
	for rows.Next() {
		itm, err := r.scanItemFromRow(rows)
		if err != nil {
			return nil, "", err
		}
		items = append(items, itm)
	}

	if err := rows.Err(); err != nil {
		return nil, "", apperrors.NewInternal("error reading item rows", err)
	}

	// Compute next cursor
	var nextCursor string
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
		last := items[len(items)-1]
		nextCursor = pagination.EncodeCursor(pagination.CursorData{
			ID:        last.ID.String(),
			CreatedAt: last.CreatedAt.Format(time.RFC3339Nano),
		})
	}

	return items, nextCursor, nil
}

// GetEligibleForPaperGeneration returns active items matching paper generation criteria.
func (r *ItemRepository) GetEligibleForPaperGeneration(ctx context.Context, orgID, subjectID uuid.UUID, maxExposureIndex float64) ([]*item.Item, error) {
	query := `
		SELECT
			id, organization_id, external_id, item_type, status,
			subject_id, chapter_id, topic_id, sub_topic_id, learning_outcome_id,
			question_content, answer_key, solution, marking_scheme, estimated_time_secs,
			difficulty_level, cognitive_level,
			irt_a, irt_b, irt_c, irt_se_a, irt_se_b, irt_se_c, irt_info_at_0,
			calibration_sample, calibration_date,
			p_value, discrimination_idx, point_biserial, distractor_analysis,
			exposure_count, max_exposure, exposure_index, last_used_at, cooldown_until,
			primary_language, variant_group_id, parent_item_id,
			author_id, author_signature,
			reviewer_id, reviewer_signature, reviewer_decision, reviewed_at,
			psychometrician_id, psychometrician_sig,
			approver_id, approver_signature, approved_at,
			tags, version, created_at, created_by, updated_at, updated_by, deleted_at
		FROM items
		WHERE organization_id = $1
			AND subject_id = $2
			AND status = 'ACTIVE'
			AND deleted_at IS NULL
			AND irt_a IS NOT NULL
			AND exposure_index <= $3
			AND (cooldown_until IS NULL OR cooldown_until < NOW())
		ORDER BY exposure_index ASC, irt_info_at_0 DESC`

	rows, err := r.pool.Query(ctx, query, orgID, subjectID, maxExposureIndex)
	if err != nil {
		return nil, apperrors.NewInternal("failed to query eligible items", err)
	}
	defer rows.Close()

	var items []*item.Item
	for rows.Next() {
		itm, err := r.scanItemFromRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, itm)
	}

	return items, rows.Err()
}

// GetEnemies returns all enemy pairs involving the given item.
func (r *ItemRepository) GetEnemies(ctx context.Context, itemID uuid.UUID) ([]item.ItemEnemy, error) {
	query := `
		SELECT item_a_id, item_b_id, reason, created_by, created_at
		FROM item_enemies
		WHERE item_a_id = $1 OR item_b_id = $1`

	rows, err := r.pool.Query(ctx, query, itemID)
	if err != nil {
		return nil, apperrors.NewInternal("failed to query item enemies", err)
	}
	defer rows.Close()

	var enemies []item.ItemEnemy
	for rows.Next() {
		var e item.ItemEnemy
		if err := rows.Scan(&e.ItemAID, &e.ItemBID, &e.Reason, &e.CreatedBy, &e.CreatedAt); err != nil {
			return nil, apperrors.NewInternal("failed to scan enemy row", err)
		}
		enemies = append(enemies, e)
	}

	return enemies, rows.Err()
}

// CreateVersion records an item version change.
func (r *ItemRepository) CreateVersion(ctx context.Context, version *item.ItemVersion) error {
	query := `
		INSERT INTO item_versions (id, item_id, version, change_type, previous_data, new_data, changed_by, changed_at, change_reason, digital_sig)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := r.pool.Exec(ctx, query,
		version.ID, version.ItemID, version.Version, version.ChangeType,
		version.PreviousData, version.NewData,
		version.ChangedBy, version.ChangedAt, version.ChangeReason, version.DigitalSig,
	)
	if err != nil {
		return apperrors.NewInternal("failed to create item version", err)
	}
	return nil
}

// GetVersionHistory returns the version history of an item.
func (r *ItemRepository) GetVersionHistory(ctx context.Context, itemID uuid.UUID) ([]item.ItemVersion, error) {
	query := `
		SELECT id, item_id, version, change_type, previous_data, new_data, changed_by, changed_at, change_reason, digital_sig
		FROM item_versions
		WHERE item_id = $1
		ORDER BY version ASC`

	rows, err := r.pool.Query(ctx, query, itemID)
	if err != nil {
		return nil, apperrors.NewInternal("failed to query version history", err)
	}
	defer rows.Close()

	var versions []item.ItemVersion
	for rows.Next() {
		var v item.ItemVersion
		if err := rows.Scan(
			&v.ID, &v.ItemID, &v.Version, &v.ChangeType,
			&v.PreviousData, &v.NewData,
			&v.ChangedBy, &v.ChangedAt, &v.ChangeReason, &v.DigitalSig,
		); err != nil {
			return nil, apperrors.NewInternal("failed to scan version row", err)
		}
		versions = append(versions, v)
	}

	return versions, rows.Err()
}

// ──────────────────────────────────────────────
//  Internal Helpers
// ──────────────────────────────────────────────

// scanItem executes a query and scans a single item result.
func (r *ItemRepository) scanItem(ctx context.Context, query string, args ...interface{}) (*item.Item, error) {
	row := r.pool.QueryRow(ctx, query, args...)
	i, err := r.scanItemFromQueryRow(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("Item", args[0])
		}
		return nil, apperrors.NewInternal("failed to scan item", err)
	}
	return i, nil
}

// scanItemFromRow scans an item from a pgx.Rows row.
func (r *ItemRepository) scanItemFromRow(rows pgx.Rows) (*item.Item, error) {
	var i item.Item
	var contentJSON, markingJSON, distAnalysis []byte
	var diffLevel, cogLevel, revDecision *string
	var irtA, irtB, irtC, irtSEA, irtSEB, irtSEC, irtInfo *float64
	var calibSample *int
	var calibDate *time.Time
	var pVal, discIdx, ptBis *float64

	err := rows.Scan(
		&i.ID, &i.OrganizationID, &i.ExternalID, &i.Type, &i.Status,
		&i.SubjectID, &i.ChapterID, &i.TopicID, &i.SubTopicID, &i.LearningOutcomeID,
		&contentJSON, &i.AnswerKey, &i.Solution, &markingJSON, &i.EstimatedTimeSecs,
		&diffLevel, &cogLevel,
		&irtA, &irtB, &irtC, &irtSEA, &irtSEB, &irtSEC, &irtInfo,
		&calibSample, &calibDate,
		&pVal, &discIdx, &ptBis, &distAnalysis,
		&i.Exposure.ExposureCount, &i.Exposure.MaxExposure, &i.Exposure.ExposureIndex,
		&i.Exposure.LastUsedAt, &i.Exposure.CooldownUntil,
		&i.PrimaryLanguage, &i.VariantGroupID, &i.ParentItemID,
		&i.Approval.AuthorID, &i.Approval.AuthorSignature,
		&i.Approval.ReviewerID, &i.Approval.ReviewerSignature, &revDecision, &i.Approval.ReviewedAt,
		&i.Approval.PsychometricianID, &i.Approval.PsychometricianSig,
		&i.Approval.ApproverID, &i.Approval.ApproverSignature, &i.Approval.ApprovedAt,
		&i.Tags, &i.Version, &i.CreatedAt, &i.CreatedBy, &i.UpdatedAt, &i.UpdatedBy, &i.DeletedAt,
	)
	if err != nil {
		return nil, apperrors.NewInternal("failed to scan item row", err)
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(contentJSON, &i.Content); err != nil {
		return nil, apperrors.NewInternal("failed to unmarshal content", err)
	}
	if err := json.Unmarshal(markingJSON, &i.MarkingScheme); err != nil {
		return nil, apperrors.NewInternal("failed to unmarshal marking scheme", err)
	}

	// Map nullable fields
	if diffLevel != nil {
		i.DifficultyLevel = item.DifficultyLevel(*diffLevel)
	}
	if cogLevel != nil {
		i.CognitiveLevel = item.CognitiveLevel(*cogLevel)
	}
	if revDecision != nil {
		i.Approval.ReviewerDecision = item.ReviewDecision(*revDecision)
	}

	// Reconstruct IRT parameters
	if irtA != nil {
		i.IRTParams = &item.IRTParameters{
			A: *irtA, B: *irtB, C: *irtC,
			SEA: deref(irtSEA), SEB: deref(irtSEB), SEC: deref(irtSEC),
			InformationAtZero: deref(irtInfo),
		}
		if calibSample != nil {
			i.IRTParams.CalibrationSampleSize = *calibSample
		}
		i.IRTParams.CalibrationDate = calibDate
	}

	// Reconstruct classical stats
	if pVal != nil {
		i.ClassicalStats = &item.ClassicalStats{
			PValue:             *pVal,
			DiscriminationIndex: deref(&discIdx),
			PointBiserial:       deref(&ptBis),
		}
		if distAnalysis != nil {
			json.Unmarshal(distAnalysis, &i.ClassicalStats.DistractorAnalysis)
		}
	}

	return &i, nil
}

// scanItemFromQueryRow scans an item from a pgx.Row (single result).
func (r *ItemRepository) scanItemFromQueryRow(row pgx.Row) (*item.Item, error) {
	var i item.Item
	var contentJSON, markingJSON, distAnalysis []byte
	var diffLevel, cogLevel, revDecision *string
	var irtA, irtB, irtC, irtSEA, irtSEB, irtSEC, irtInfo *float64
	var calibSample *int
	var calibDate *time.Time
	var pVal, discIdx, ptBis *float64

	err := row.Scan(
		&i.ID, &i.OrganizationID, &i.ExternalID, &i.Type, &i.Status,
		&i.SubjectID, &i.ChapterID, &i.TopicID, &i.SubTopicID, &i.LearningOutcomeID,
		&contentJSON, &i.AnswerKey, &i.Solution, &markingJSON, &i.EstimatedTimeSecs,
		&diffLevel, &cogLevel,
		&irtA, &irtB, &irtC, &irtSEA, &irtSEB, &irtSEC, &irtInfo,
		&calibSample, &calibDate,
		&pVal, &discIdx, &ptBis, &distAnalysis,
		&i.Exposure.ExposureCount, &i.Exposure.MaxExposure, &i.Exposure.ExposureIndex,
		&i.Exposure.LastUsedAt, &i.Exposure.CooldownUntil,
		&i.PrimaryLanguage, &i.VariantGroupID, &i.ParentItemID,
		&i.Approval.AuthorID, &i.Approval.AuthorSignature,
		&i.Approval.ReviewerID, &i.Approval.ReviewerSignature, &revDecision, &i.Approval.ReviewedAt,
		&i.Approval.PsychometricianID, &i.Approval.PsychometricianSig,
		&i.Approval.ApproverID, &i.Approval.ApproverSignature, &i.Approval.ApprovedAt,
		&i.Tags, &i.Version, &i.CreatedAt, &i.CreatedBy, &i.UpdatedAt, &i.UpdatedBy, &i.DeletedAt,
	)
	if err != nil {
		return nil, err
	}

	// Unmarshal and map (same as scanItemFromRow)
	json.Unmarshal(contentJSON, &i.Content)
	json.Unmarshal(markingJSON, &i.MarkingScheme)

	if diffLevel != nil {
		i.DifficultyLevel = item.DifficultyLevel(*diffLevel)
	}
	if cogLevel != nil {
		i.CognitiveLevel = item.CognitiveLevel(*cogLevel)
	}
	if revDecision != nil {
		i.Approval.ReviewerDecision = item.ReviewDecision(*revDecision)
	}
	if irtA != nil {
		i.IRTParams = &item.IRTParameters{
			A: *irtA, B: *irtB, C: *irtC,
			SEA: deref(irtSEA), SEB: deref(irtSEB), SEC: deref(irtSEC),
			InformationAtZero: deref(irtInfo),
		}
		if calibSample != nil {
			i.IRTParams.CalibrationSampleSize = *calibSample
		}
		i.IRTParams.CalibrationDate = calibDate
	}
	if pVal != nil {
		i.ClassicalStats = &item.ClassicalStats{
			PValue: *pVal, DiscriminationIndex: deref(&discIdx), PointBiserial: deref(&ptBis),
		}
		if distAnalysis != nil {
			json.Unmarshal(distAnalysis, &i.ClassicalStats.DistractorAnalysis)
		}
	}

	return &i, nil
}

// nullableString converts an empty string to nil for nullable DB columns.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// deref safely dereferences a *float64, returning 0 if nil.
func deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}
