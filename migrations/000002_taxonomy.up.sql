-- Migration 002: Taxonomy tables
-- Hierarchical classification system for assessment items:
-- Subject → Chapter → Topic → Sub-Topic → Learning Outcome

CREATE TABLE subjects (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID         NOT NULL REFERENCES organizations(id),
    code            VARCHAR(50)  NOT NULL,
    name            VARCHAR(200) NOT NULL,
    description     TEXT,
    status          VARCHAR(20)  NOT NULL DEFAULT 'ACTIVE'
                    CHECK (status IN ('ACTIVE', 'INACTIVE')),
    display_order   INTEGER      NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_subject_code UNIQUE (organization_id, code)
);

CREATE TABLE chapters (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id      UUID         NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    organization_id UUID         NOT NULL REFERENCES organizations(id),
    code            VARCHAR(50)  NOT NULL,
    name            VARCHAR(200) NOT NULL,
    description     TEXT,
    display_order   INTEGER      NOT NULL DEFAULT 0,
    weightage       DOUBLE PRECISION NOT NULL DEFAULT 0.0,  -- Relative weight in exam blueprint
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_chapter_code UNIQUE (subject_id, code)
);

CREATE INDEX idx_chapters_subject ON chapters (subject_id);

CREATE TABLE topics (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chapter_id      UUID         NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
    organization_id UUID         NOT NULL REFERENCES organizations(id),
    code            VARCHAR(50)  NOT NULL,
    name            VARCHAR(200) NOT NULL,
    description     TEXT,
    display_order   INTEGER      NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_topic_code UNIQUE (chapter_id, code)
);

CREATE INDEX idx_topics_chapter ON topics (chapter_id);

CREATE TABLE sub_topics (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    topic_id        UUID         NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    organization_id UUID         NOT NULL REFERENCES organizations(id),
    code            VARCHAR(50)  NOT NULL,
    name            VARCHAR(200) NOT NULL,
    description     TEXT,
    display_order   INTEGER      NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_sub_topic_code UNIQUE (topic_id, code)
);

CREATE INDEX idx_sub_topics_topic ON sub_topics (topic_id);

CREATE TABLE learning_outcomes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID         NOT NULL REFERENCES organizations(id),
    code            VARCHAR(50)  NOT NULL,
    description     TEXT         NOT NULL,
    -- A learning outcome can be linked to multiple taxonomy levels
    subject_id      UUID         REFERENCES subjects(id),
    chapter_id      UUID         REFERENCES chapters(id),
    topic_id        UUID         REFERENCES topics(id),
    cognitive_level VARCHAR(20)  CHECK (cognitive_level IN
                    ('REMEMBER', 'UNDERSTAND', 'APPLY', 'ANALYZE', 'EVALUATE', 'CREATE')),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_lo_code UNIQUE (organization_id, code)
);

CREATE INDEX idx_lo_subject ON learning_outcomes (subject_id) WHERE subject_id IS NOT NULL;
CREATE INDEX idx_lo_chapter ON learning_outcomes (chapter_id) WHERE chapter_id IS NOT NULL;

-- Enable RLS on taxonomy tables
ALTER TABLE subjects ENABLE ROW LEVEL SECURITY;
CREATE POLICY subjects_org_isolation ON subjects
    USING (organization_id = current_setting('app.current_organization_id')::UUID);

ALTER TABLE chapters ENABLE ROW LEVEL SECURITY;
CREATE POLICY chapters_org_isolation ON chapters
    USING (organization_id = current_setting('app.current_organization_id')::UUID);

ALTER TABLE topics ENABLE ROW LEVEL SECURITY;
CREATE POLICY topics_org_isolation ON topics
    USING (organization_id = current_setting('app.current_organization_id')::UUID);

ALTER TABLE sub_topics ENABLE ROW LEVEL SECURITY;
CREATE POLICY sub_topics_org_isolation ON sub_topics
    USING (organization_id = current_setting('app.current_organization_id')::UUID);

ALTER TABLE learning_outcomes ENABLE ROW LEVEL SECURITY;
CREATE POLICY lo_org_isolation ON learning_outcomes
    USING (organization_id = current_setting('app.current_organization_id')::UUID);

-- Auto-update triggers
CREATE TRIGGER trg_subjects_updated_at BEFORE UPDATE ON subjects
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER trg_chapters_updated_at BEFORE UPDATE ON chapters
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER trg_topics_updated_at BEFORE UPDATE ON topics
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER trg_sub_topics_updated_at BEFORE UPDATE ON sub_topics
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER trg_lo_updated_at BEFORE UPDATE ON learning_outcomes
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
