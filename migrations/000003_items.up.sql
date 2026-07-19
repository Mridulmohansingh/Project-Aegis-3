-- Migration 003: Items, Item Versions, Item Enemies, Item Translations
-- The core question bank tables with full psychometric metadata,
-- IRT parameters, exposure control, and cryptographic approval chain.

CREATE TABLE items (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     UUID NOT NULL REFERENCES organizations(id),
    external_id         VARCHAR(50) NOT NULL,
    item_type           VARCHAR(20) NOT NULL
                        CHECK (item_type IN ('MCQ_SINGLE', 'MCQ_MULTI', 'INTEGER',
                               'DESCRIPTIVE', 'MATCHING', 'ASSERTION_REASON', 'CASE_BASED')),
    status              VARCHAR(20) NOT NULL DEFAULT 'DRAFT'
                        CHECK (status IN ('DRAFT', 'REVIEW', 'CALIBRATION', 'PILOT',
                               'ACTIVE', 'SUSPENDED', 'RETIRED')),

    -- Taxonomy references
    subject_id          UUID NOT NULL REFERENCES subjects(id),
    chapter_id          UUID NOT NULL REFERENCES chapters(id),
    topic_id            UUID NOT NULL REFERENCES topics(id),
    sub_topic_id        UUID REFERENCES sub_topics(id),
    learning_outcome_id UUID REFERENCES learning_outcomes(id),

    -- Content (question_content is JSONB; answer_key and solution are encrypted)
    question_content    JSONB NOT NULL,
    answer_key          BYTEA NOT NULL,     -- AES-256-GCM encrypted
    solution            BYTEA,              -- AES-256-GCM encrypted (optional)
    marking_scheme      JSONB NOT NULL,     -- {"correct": 4, "incorrect": -1, "unanswered": 0}
    estimated_time_secs INTEGER NOT NULL DEFAULT 120,

    -- Classification
    difficulty_level    VARCHAR(10)
                        CHECK (difficulty_level IN ('EASY', 'MEDIUM', 'HARD', 'VERY_HARD')),
    cognitive_level     VARCHAR(20)
                        CHECK (cognitive_level IN ('REMEMBER', 'UNDERSTAND', 'APPLY',
                               'ANALYZE', 'EVALUATE', 'CREATE')),

    -- IRT Parameters (3-Parameter Logistic Model)
    irt_a               DOUBLE PRECISION,   -- Discrimination [0.5, 2.5]
    irt_b               DOUBLE PRECISION,   -- Difficulty [-3.0, +3.0] logits
    irt_c               DOUBLE PRECISION,   -- Guessing [0.0, 0.35]
    irt_se_a            DOUBLE PRECISION,   -- SE of discrimination
    irt_se_b            DOUBLE PRECISION,   -- SE of difficulty
    irt_se_c            DOUBLE PRECISION,   -- SE of guessing
    irt_info_at_0       DOUBLE PRECISION,   -- Fisher information at θ=0
    calibration_sample  INTEGER,            -- Calibration sample size
    calibration_date    TIMESTAMPTZ,

    -- Classical Test Theory statistics
    p_value             DOUBLE PRECISION,   -- Proportion correct
    discrimination_idx  DOUBLE PRECISION,   -- Upper-lower 27% discrimination
    point_biserial      DOUBLE PRECISION,   -- Point-biserial correlation
    distractor_analysis JSONB,              -- {"A": 0.25, "B": 0.40, ...}

    -- Exposure control
    exposure_count      INTEGER NOT NULL DEFAULT 0,
    max_exposure        INTEGER NOT NULL DEFAULT 50,
    exposure_index      DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    last_used_at        TIMESTAMPTZ,
    cooldown_until      TIMESTAMPTZ,

    -- Language & variants
    primary_language    VARCHAR(10) NOT NULL DEFAULT 'en',
    variant_group_id    UUID,                   -- Links isomorphic items
    parent_item_id      UUID REFERENCES items(id),

    -- Approval chain (digital signatures)
    author_id           UUID NOT NULL REFERENCES users(id),
    author_signature    BYTEA,
    reviewer_id         UUID REFERENCES users(id),
    reviewer_signature  BYTEA,
    reviewer_decision   VARCHAR(10)
                        CHECK (reviewer_decision IN ('APPROVED', 'REJECTED', 'REVISION')),
    reviewed_at         TIMESTAMPTZ,
    psychometrician_id  UUID REFERENCES users(id),
    psychometrician_sig BYTEA,
    approver_id         UUID REFERENCES users(id),
    approver_signature  BYTEA,
    approved_at         TIMESTAMPTZ,

    -- Metadata
    tags                TEXT[],
    version             INTEGER NOT NULL DEFAULT 1,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by          UUID NOT NULL REFERENCES users(id),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by          UUID NOT NULL REFERENCES users(id),
    deleted_at          TIMESTAMPTZ,

    CONSTRAINT uq_item_external UNIQUE (organization_id, external_id),
    -- Ensure IRT parameters are either all NULL or all set
    CONSTRAINT chk_irt_params CHECK (
        (irt_a IS NULL AND irt_b IS NULL AND irt_c IS NULL) OR
        (irt_a IS NOT NULL AND irt_b IS NOT NULL AND irt_c IS NOT NULL)
    )
);

-- Optimized indexes for paper generation queries
CREATE INDEX idx_items_active_pool ON items (organization_id, subject_id, status, difficulty_level)
    WHERE status = 'ACTIVE' AND deleted_at IS NULL;

CREATE INDEX idx_items_topic_coverage ON items (topic_id, sub_topic_id, cognitive_level)
    WHERE status = 'ACTIVE' AND deleted_at IS NULL;

CREATE INDEX idx_items_exposure ON items (exposure_index, last_used_at)
    WHERE status = 'ACTIVE' AND deleted_at IS NULL;

CREATE INDEX idx_items_irt ON items (irt_b, irt_a)
    WHERE status = 'ACTIVE' AND irt_b IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX idx_items_variant_group ON items (variant_group_id)
    WHERE variant_group_id IS NOT NULL;

CREATE INDEX idx_items_chapter ON items (chapter_id, status)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_items_author ON items (author_id, status)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_items_status ON items (organization_id, status)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_items_tags ON items USING GIN (tags)
    WHERE deleted_at IS NULL;

-- Full-text search on question content
CREATE INDEX idx_items_content_search ON items USING GIN (question_content jsonb_path_ops)
    WHERE deleted_at IS NULL;

-- ──────────────────────────────────────────────
--  Item Version History (Immutable)
-- ──────────────────────────────────────────────

CREATE TABLE item_versions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id         UUID NOT NULL REFERENCES items(id),
    version         INTEGER NOT NULL,
    change_type     VARCHAR(20) NOT NULL
                    CHECK (change_type IN ('CONTENT', 'METADATA', 'STATUS', 'IRT_UPDATE')),
    previous_data   JSONB NOT NULL,
    new_data        JSONB NOT NULL,
    changed_by      UUID NOT NULL REFERENCES users(id),
    changed_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    change_reason   TEXT,
    digital_sig     BYTEA NOT NULL,     -- Ed25519 signature over (item_id || version || data)

    CONSTRAINT uq_item_version UNIQUE (item_id, version)
);

CREATE INDEX idx_item_versions_item ON item_versions (item_id, version);

-- Prevent updates and deletes on version history
-- This is enforced via a trigger
CREATE OR REPLACE FUNCTION prevent_version_modification()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'item_versions is append-only: modifications are not allowed';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_item_versions_immutable
    BEFORE UPDATE OR DELETE ON item_versions
    FOR EACH ROW EXECUTE FUNCTION prevent_version_modification();

-- ──────────────────────────────────────────────
--  Item Enemy Pairs
-- ──────────────────────────────────────────────

CREATE TABLE item_enemies (
    item_a_id       UUID NOT NULL REFERENCES items(id),
    item_b_id       UUID NOT NULL REFERENCES items(id),
    reason          TEXT NOT NULL,
    created_by      UUID NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (item_a_id, item_b_id),
    CHECK (item_a_id < item_b_id)    -- Canonical ordering
);

CREATE INDEX idx_item_enemies_b ON item_enemies (item_b_id);

-- ──────────────────────────────────────────────
--  Item Translations
-- ──────────────────────────────────────────────

CREATE TABLE item_translations (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id             UUID NOT NULL REFERENCES items(id),
    language            VARCHAR(10) NOT NULL,   -- ISO 639-1 codes
    question_content    JSONB NOT NULL,
    translation_status  VARCHAR(20) NOT NULL DEFAULT 'DRAFT'
                        CHECK (translation_status IN ('DRAFT', 'REVIEW', 'VERIFIED', 'FLAGGED')),
    translator_id       UUID REFERENCES users(id),
    verifier_id         UUID REFERENCES users(id),
    verified_at         TIMESTAMPTZ,
    dif_flag            BOOLEAN DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_item_lang UNIQUE (item_id, language)
);

CREATE INDEX idx_item_translations_item ON item_translations (item_id);
CREATE INDEX idx_item_translations_lang ON item_translations (language, translation_status);

-- RLS
ALTER TABLE items ENABLE ROW LEVEL SECURITY;
CREATE POLICY items_org_isolation ON items
    USING (organization_id = current_setting('app.current_organization_id')::UUID);

-- Auto-update triggers
CREATE TRIGGER trg_items_updated_at BEFORE UPDATE ON items
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER trg_item_translations_updated_at BEFORE UPDATE ON item_translations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
