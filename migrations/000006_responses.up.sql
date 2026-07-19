-- Migration 006: Responses (Hash-Partitioned for Scalability)
-- Designed to handle billions of response records across millions of exams.
-- Hash-partitioned by exam_id for even distribution across partitions.

-- ──────────────────────────────────────────────
--  Exam Sessions
-- ──────────────────────────────────────────────
CREATE TABLE exam_sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    exam_id         UUID         NOT NULL REFERENCES exams(id),
    candidate_id    UUID         NOT NULL REFERENCES candidates(id),
    paper_id        UUID         NOT NULL REFERENCES papers(id),
    center_id       UUID         REFERENCES centers(id),
    -- Session lifecycle
    status          VARCHAR(20)  NOT NULL DEFAULT 'INITIALIZED'
                    CHECK (status IN ('INITIALIZED', 'AUTHENTICATED', 'IN_PROGRESS',
                           'PAUSED', 'COMPLETED', 'TERMINATED', 'TIMED_OUT')),
    -- Timing (server-authoritative via NTP)
    scheduled_start TIMESTAMPTZ  NOT NULL,
    actual_start    TIMESTAMPTZ,
    actual_end      TIMESTAMPTZ,
    remaining_secs  INTEGER,          -- Server-computed remaining time
    pause_count     SMALLINT NOT NULL DEFAULT 0,
    total_pause_secs INTEGER NOT NULL DEFAULT 0,
    -- Security
    client_ip       INET,
    user_agent      VARCHAR(500),
    device_fingerprint BYTEA,         -- Hashed device info
    session_token_hash BYTEA NOT NULL, -- SHA-256 of session token (token never stored)
    -- Integrity
    total_responses INTEGER NOT NULL DEFAULT 0,
    last_sequence   BIGINT NOT NULL DEFAULT 0,
    -- Metadata
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_session_candidate_exam UNIQUE (exam_id, candidate_id)
);

CREATE INDEX idx_sessions_exam ON exam_sessions (exam_id, status);
CREATE INDEX idx_sessions_candidate ON exam_sessions (candidate_id);
CREATE INDEX idx_sessions_active ON exam_sessions (exam_id)
    WHERE status IN ('INITIALIZED', 'AUTHENTICATED', 'IN_PROGRESS', 'PAUSED');

-- ──────────────────────────────────────────────
--  Responses (Hash-Partitioned)
-- ──────────────────────────────────────────────
CREATE TABLE responses (
    id                  UUID NOT NULL DEFAULT gen_random_uuid(),
    exam_id             UUID NOT NULL,              -- Partition key
    candidate_id        UUID NOT NULL,
    session_id          UUID NOT NULL,
    paper_id            UUID NOT NULL,
    item_id             UUID NOT NULL,
    section_index       SMALLINT NOT NULL,
    question_index      SMALLINT NOT NULL,

    -- Response data
    selected_option     SMALLINT,                   -- For MCQ single (0-indexed)
    selected_options    SMALLINT[],                  -- For MCQ multi-select
    integer_answer      INTEGER,                     -- For integer-type
    text_answer         BYTEA,                       -- For descriptive (AES-256-GCM encrypted)
    is_marked           BOOLEAN NOT NULL DEFAULT FALSE,
    is_visited          BOOLEAN NOT NULL DEFAULT FALSE,
    visit_count         SMALLINT NOT NULL DEFAULT 0,
    time_spent_ms       INTEGER NOT NULL DEFAULT 0,

    -- Response timeline
    first_response_at   TIMESTAMPTZ,
    last_modified_at    TIMESTAMPTZ,
    response_changes    SMALLINT NOT NULL DEFAULT 0,

    -- Integrity verification
    client_timestamp    TIMESTAMPTZ NOT NULL,
    server_timestamp    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    client_hash         BYTEA NOT NULL,              -- HMAC-SHA256 of response data
    sequence_number     BIGINT NOT NULL,              -- Monotonic per session

    PRIMARY KEY (exam_id, id)
) PARTITION BY HASH (exam_id);

-- Create 64 hash partitions for even distribution
DO $$
BEGIN
    FOR i IN 0..63 LOOP
        EXECUTE format(
            'CREATE TABLE responses_p%s PARTITION OF responses FOR VALUES WITH (MODULUS 64, REMAINDER %s)',
            i, i
        );
    END LOOP;
END$$;

-- Indexes on the partitioned response table
CREATE INDEX idx_responses_candidate ON responses (exam_id, candidate_id);
CREATE INDEX idx_responses_session ON responses (session_id);
CREATE INDEX idx_responses_item ON responses (exam_id, item_id);
CREATE INDEX idx_responses_sequence ON responses (session_id, sequence_number);

-- ──────────────────────────────────────────────
--  Response Sync Queue (Offline Recovery)
-- ──────────────────────────────────────────────
CREATE TABLE response_sync_queue (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id      UUID NOT NULL,
    exam_id         UUID NOT NULL,
    candidate_id    UUID NOT NULL,
    -- Encrypted batch of responses from offline mode
    encrypted_batch BYTEA NOT NULL,
    batch_hash      BYTEA NOT NULL,              -- SHA-256 of plaintext batch
    batch_size      INTEGER NOT NULL,
    -- Processing
    status          VARCHAR(20) NOT NULL DEFAULT 'PENDING'
                    CHECK (status IN ('PENDING', 'PROCESSING', 'COMPLETED', 'FAILED', 'DUPLICATE')),
    processed_at    TIMESTAMPTZ,
    error_message   TEXT,
    -- Metadata
    client_timestamp TIMESTAMPTZ NOT NULL,
    server_received  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sync_queue_session ON response_sync_queue (session_id);
CREATE INDEX idx_sync_queue_pending ON response_sync_queue (status)
    WHERE status = 'PENDING';

-- Triggers
CREATE TRIGGER trg_sessions_updated_at BEFORE UPDATE ON exam_sessions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
