-- Migration 004: Blueprints, Exams, Papers
-- Test assembly blueprints, exam configurations, and generated paper records.

-- ──────────────────────────────────────────────
--  Blueprints
-- ──────────────────────────────────────────────
CREATE TABLE blueprints (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID         NOT NULL REFERENCES organizations(id),
    name            VARCHAR(200) NOT NULL,
    subject_id      UUID         NOT NULL REFERENCES subjects(id),
    total_items     INTEGER      NOT NULL CHECK (total_items > 0),
    constraints     JSONB        NOT NULL,   -- Full constraint specification
    version         INTEGER      NOT NULL DEFAULT 1,
    status          VARCHAR(20)  NOT NULL DEFAULT 'DRAFT'
                    CHECK (status IN ('DRAFT', 'ACTIVE', 'ARCHIVED')),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_by      UUID         NOT NULL REFERENCES users(id),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_by      UUID         NOT NULL REFERENCES users(id),

    CONSTRAINT uq_blueprint_name UNIQUE (organization_id, name)
);

CREATE INDEX idx_blueprints_org ON blueprints (organization_id, status);
CREATE INDEX idx_blueprints_subject ON blueprints (subject_id);

-- ──────────────────────────────────────────────
--  Exams
-- ──────────────────────────────────────────────
CREATE TABLE exams (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     UUID         NOT NULL REFERENCES organizations(id),
    exam_code           VARCHAR(50)  NOT NULL,
    exam_name           VARCHAR(200) NOT NULL,
    exam_type           VARCHAR(20)  NOT NULL
                        CHECK (exam_type IN ('FIXED_FORM', 'LINEAR_ON_FLY', 'CAT', 'MULTI_STAGE')),
    status              VARCHAR(20)  NOT NULL DEFAULT 'DRAFT'
                        CHECK (status IN ('DRAFT', 'CONFIGURED', 'PAPERS_GENERATED',
                               'SCHEDULED', 'ACTIVE', 'COMPLETED', 'CANCELLED', 'ARCHIVED')),
    -- Exam parameters
    total_marks         INTEGER      NOT NULL CHECK (total_marks > 0),
    total_questions     INTEGER      NOT NULL CHECK (total_questions > 0),
    duration_minutes    INTEGER      NOT NULL CHECK (duration_minutes > 0),
    negative_marking    BOOLEAN      NOT NULL DEFAULT FALSE,
    -- Structure
    sections            JSONB        NOT NULL DEFAULT '[]',
    blueprint_id        UUID         NOT NULL REFERENCES blueprints(id),
    -- Scheduling
    scheduling          JSONB        NOT NULL DEFAULT '{}',
    -- Configuration
    config              JSONB        NOT NULL DEFAULT '{}',
    -- Metadata
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_by          UUID         NOT NULL REFERENCES users(id),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_by          UUID         NOT NULL REFERENCES users(id),

    CONSTRAINT uq_exam_code UNIQUE (organization_id, exam_code)
);

CREATE INDEX idx_exams_org_status ON exams (organization_id, status);
CREATE INDEX idx_exams_blueprint ON exams (blueprint_id);
CREATE INDEX idx_exams_scheduling ON exams USING GIN (scheduling jsonb_path_ops);

-- ──────────────────────────────────────────────
--  Papers (Generated Test Forms)
-- ──────────────────────────────────────────────
CREATE TABLE papers (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    exam_id             UUID         NOT NULL REFERENCES exams(id),
    paper_code          VARCHAR(50)  NOT NULL,
    form_number         INTEGER      NOT NULL CHECK (form_number > 0),
    -- Encrypted item list (only HSM can decrypt)
    encrypted_item_ids  BYTEA        NOT NULL,
    encrypted_key_id    VARCHAR(100) NOT NULL,    -- HSM/KMS key reference
    item_sequence_hash  BYTEA        NOT NULL,    -- SHA-256 of ordered item IDs
    item_count          INTEGER      NOT NULL CHECK (item_count > 0),

    -- Psychometric profile
    mean_difficulty     DOUBLE PRECISION,
    std_difficulty      DOUBLE PRECISION,
    test_information    JSONB,                     -- {"0.0": 12.5, "1.0": 8.3, ...}
    reliability_est     DOUBLE PRECISION,
    total_time_est_secs INTEGER,
    difficulty_dist     JSONB,                     -- {"EASY": 15, "MEDIUM": 30, ...}
    cognitive_dist      JSONB,                     -- {"REMEMBER": 5, "APPLY": 20, ...}
    chapter_dist        JSONB,                     -- {"chapterId": count, ...}

    -- Generation metadata
    generation_log      JSONB        NOT NULL,
    generated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    generated_by        VARCHAR(50)  NOT NULL DEFAULT 'SYSTEM',
    digital_signature   BYTEA        NOT NULL,     -- Ed25519 signature

    CONSTRAINT uq_paper_form UNIQUE (exam_id, form_number)
);

CREATE INDEX idx_papers_exam ON papers (exam_id);

-- ──────────────────────────────────────────────
--  Paper Generation Jobs (Async tracking)
-- ──────────────────────────────────────────────
CREATE TABLE paper_generation_jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    exam_id         UUID         NOT NULL REFERENCES exams(id),
    status          VARCHAR(20)  NOT NULL DEFAULT 'PENDING'
                    CHECK (status IN ('PENDING', 'RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED')),
    form_count      INTEGER      NOT NULL CHECK (form_count > 0),
    forms_completed INTEGER      NOT NULL DEFAULT 0,
    error_message   TEXT,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    requested_by    UUID         NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_paper_gen_jobs_exam ON paper_generation_jobs (exam_id);
CREATE INDEX idx_paper_gen_jobs_status ON paper_generation_jobs (status)
    WHERE status IN ('PENDING', 'RUNNING');

-- RLS
ALTER TABLE blueprints ENABLE ROW LEVEL SECURITY;
CREATE POLICY blueprints_org_isolation ON blueprints
    USING (organization_id = current_setting('app.current_organization_id')::UUID);

ALTER TABLE exams ENABLE ROW LEVEL SECURITY;
CREATE POLICY exams_org_isolation ON exams
    USING (organization_id = current_setting('app.current_organization_id')::UUID);

-- Triggers
CREATE TRIGGER trg_blueprints_updated_at BEFORE UPDATE ON blueprints
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER trg_exams_updated_at BEFORE UPDATE ON exams
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
