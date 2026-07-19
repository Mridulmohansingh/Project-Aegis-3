-- Migration 007: Scores and Results

-- ──────────────────────────────────────────────
--  Individual Scores
-- ──────────────────────────────────────────────
CREATE TABLE scores (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    exam_id             UUID         NOT NULL REFERENCES exams(id),
    candidate_id        UUID         NOT NULL REFERENCES candidates(id),
    paper_id            UUID         NOT NULL REFERENCES papers(id),
    organization_id     UUID         NOT NULL REFERENCES organizations(id),

    -- Raw scoring
    raw_score           DOUBLE PRECISION NOT NULL,
    max_score           DOUBLE PRECISION NOT NULL,
    correct_count       INTEGER NOT NULL,
    incorrect_count     INTEGER NOT NULL,
    unanswered_count    INTEGER NOT NULL,
    total_items         INTEGER NOT NULL,

    -- IRT scoring
    theta_estimate      DOUBLE PRECISION,       -- MLE/EAP ability estimate
    theta_se            DOUBLE PRECISION,       -- Standard error of theta
    estimation_method   VARCHAR(10)             -- 'MLE', 'EAP', 'MAP'
                        CHECK (estimation_method IN ('MLE', 'EAP', 'MAP')),
    theta_converged     BOOLEAN,

    -- Scaled/Equated scores
    scaled_score        DOUBLE PRECISION,       -- Equated to reference form
    percentile          DOUBLE PRECISION,       -- Percentile rank (0-100)
    normalized_score    DOUBLE PRECISION,       -- z-score or T-score

    -- Person-fit statistics
    person_fit_lz       DOUBLE PRECISION,       -- Standardized log-likelihood
    person_fit_flag     BOOLEAN DEFAULT FALSE,
    person_fit_reason   TEXT,

    -- Section-wise breakdown
    section_scores      JSONB NOT NULL DEFAULT '[]',
    -- [{"section": "Physics", "raw": 40, "correct": 10, "incorrect": 2, "unanswered": 3}]

    -- Scoring metadata
    scored_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    scoring_version     VARCHAR(50) NOT NULL,    -- Version of scoring algorithm
    digital_signature   BYTEA NOT NULL,          -- Ed25519 signature over score data

    CONSTRAINT uq_score_candidate_exam UNIQUE (exam_id, candidate_id)
);

CREATE INDEX idx_scores_exam ON scores (exam_id);
CREATE INDEX idx_scores_candidate ON scores (candidate_id);
CREATE INDEX idx_scores_percentile ON scores (exam_id, percentile DESC);
CREATE INDEX idx_scores_person_fit ON scores (exam_id)
    WHERE person_fit_flag = TRUE;

-- ──────────────────────────────────────────────
--  Published Results
-- ──────────────────────────────────────────────
CREATE TABLE results (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    exam_id             UUID         NOT NULL REFERENCES exams(id),
    candidate_id        UUID         NOT NULL REFERENCES candidates(id),
    score_id            UUID         NOT NULL REFERENCES scores(id),
    organization_id     UUID         NOT NULL REFERENCES organizations(id),

    -- Result outcome
    rank                INTEGER,                -- Overall rank
    category_rank       INTEGER,                -- Rank within category
    qualified           BOOLEAN NOT NULL,
    qualification_score DOUBLE PRECISION,       -- Cut-off score used

    -- Verifiable credential
    credential_id       VARCHAR(200),           -- W3C Verifiable Credential ID
    credential_hash     BYTEA,                  -- SHA-256 of credential document

    -- Publication
    published_at        TIMESTAMPTZ,
    published_by        UUID REFERENCES users(id),
    status              VARCHAR(20) NOT NULL DEFAULT 'DRAFT'
                        CHECK (status IN ('DRAFT', 'PROVISIONAL', 'FINAL', 'CHALLENGED', 'REVISED')),

    -- Metadata
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_result_candidate_exam UNIQUE (exam_id, candidate_id)
);

CREATE INDEX idx_results_exam_rank ON results (exam_id, rank) WHERE rank IS NOT NULL;
CREATE INDEX idx_results_status ON results (exam_id, status);

-- ──────────────────────────────────────────────
--  DIF Analysis Results
-- ──────────────────────────────────────────────
CREATE TABLE dif_results (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    exam_id         UUID         NOT NULL REFERENCES exams(id),
    item_id         UUID         NOT NULL REFERENCES items(id),
    -- DIF parameters
    reference_group VARCHAR(50)  NOT NULL,       -- e.g., 'MALE', 'ENGLISH'
    focal_group     VARCHAR(50)  NOT NULL,       -- e.g., 'FEMALE', 'HINDI'
    grouping_var    VARCHAR(50)  NOT NULL,       -- e.g., 'GENDER', 'LANGUAGE'
    -- Mantel-Haenszel statistics
    mh_chi_square   DOUBLE PRECISION NOT NULL,
    mh_odds_ratio   DOUBLE PRECISION NOT NULL,
    mh_delta        DOUBLE PRECISION NOT NULL,   -- Δ_MH = -2.35 × ln(α_MH)
    p_value         DOUBLE PRECISION NOT NULL,
    dif_category    CHAR(1) NOT NULL CHECK (dif_category IN ('A', 'B', 'C')),
    flagged         BOOLEAN NOT NULL DEFAULT FALSE,
    -- Metadata
    analysis_date   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    analysis_version VARCHAR(50) NOT NULL
);

CREATE INDEX idx_dif_exam_item ON dif_results (exam_id, item_id);
CREATE INDEX idx_dif_flagged ON dif_results (exam_id) WHERE flagged = TRUE;

-- ──────────────────────────────────────────────
--  Exam Statistics (Aggregate)
-- ──────────────────────────────────────────────
CREATE TABLE exam_statistics (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    exam_id         UUID         NOT NULL REFERENCES exams(id) UNIQUE,
    organization_id UUID         NOT NULL REFERENCES organizations(id),

    -- Descriptive stats
    total_appeared  INTEGER NOT NULL,
    total_qualified INTEGER NOT NULL,
    mean_raw_score  DOUBLE PRECISION NOT NULL,
    median_raw_score DOUBLE PRECISION NOT NULL,
    std_raw_score   DOUBLE PRECISION NOT NULL,
    min_raw_score   DOUBLE PRECISION NOT NULL,
    max_raw_score   DOUBLE PRECISION NOT NULL,

    -- IRT stats
    mean_theta      DOUBLE PRECISION,
    std_theta       DOUBLE PRECISION,

    -- Reliability
    cronbach_alpha  DOUBLE PRECISION,
    marginal_reliability DOUBLE PRECISION,

    -- Score distribution (histogram)
    score_distribution JSONB,           -- [{"range": "0-10", "count": 1234}, ...]
    percentile_table   JSONB,           -- {"50": 65.3, "75": 78.2, "90": 89.1, ...}

    -- Cut-offs
    cut_offs        JSONB,              -- {"GENERAL": 45.0, "OBC": 40.0, "SC": 35.0, ...}

    -- Metadata
    computed_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version         VARCHAR(50) NOT NULL
);

-- RLS
ALTER TABLE scores ENABLE ROW LEVEL SECURITY;
CREATE POLICY scores_org_isolation ON scores
    USING (organization_id = current_setting('app.current_organization_id')::UUID);

ALTER TABLE results ENABLE ROW LEVEL SECURITY;
CREATE POLICY results_org_isolation ON results
    USING (organization_id = current_setting('app.current_organization_id')::UUID);

-- Triggers
CREATE TRIGGER trg_results_updated_at BEFORE UPDATE ON results
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
