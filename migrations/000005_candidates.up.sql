-- Migration 005: Candidates and Registrations

CREATE TABLE candidates (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     UUID         NOT NULL REFERENCES organizations(id),
    -- Identity (PII — encrypted at application layer)
    candidate_id_number VARCHAR(100) NOT NULL,    -- Application number / roll number
    full_name_encrypted BYTEA        NOT NULL,    -- AES-256-GCM
    email_hash          BYTEA        NOT NULL,    -- SHA-256 for lookup
    phone_hash          BYTEA,                    -- SHA-256 for lookup
    date_of_birth_encrypted BYTEA,                -- AES-256-GCM
    -- Identity verification
    aadhaar_token       VARCHAR(100),             -- Tokenized Aadhaar (never store actual number)
    identity_verified   BOOLEAN NOT NULL DEFAULT FALSE,
    verification_method VARCHAR(50),              -- 'AADHAAR', 'DIGILOCKER', 'MANUAL'
    verified_at         TIMESTAMPTZ,
    -- Category & accommodations
    category            VARCHAR(20) CHECK (category IN ('GENERAL', 'OBC', 'SC', 'ST', 'EWS', 'PH')),
    pwd_type            VARCHAR(50),              -- Type of disability
    accommodation       JSONB DEFAULT '{}',       -- {"extra_time_pct": 33, "scribe": true, ...}
    -- Authentication
    auth_credential_id  VARCHAR(255),             -- FIDO2 credential ID
    -- Metadata
    status              VARCHAR(20) NOT NULL DEFAULT 'ACTIVE'
                        CHECK (status IN ('ACTIVE', 'SUSPENDED', 'BLOCKED')),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_candidate_id UNIQUE (organization_id, candidate_id_number)
);

-- Lookup indexes (on hashed values for privacy-preserving search)
CREATE INDEX idx_candidates_email_hash ON candidates (email_hash);
CREATE INDEX idx_candidates_phone_hash ON candidates (phone_hash);
CREATE INDEX idx_candidates_aadhaar ON candidates (aadhaar_token) WHERE aadhaar_token IS NOT NULL;
CREATE INDEX idx_candidates_org ON candidates (organization_id, status);

-- ──────────────────────────────────────────────
--  Exam Registrations
-- ──────────────────────────────────────────────
CREATE TABLE registrations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    candidate_id    UUID         NOT NULL REFERENCES candidates(id),
    exam_id         UUID         NOT NULL REFERENCES exams(id),
    organization_id UUID         NOT NULL REFERENCES organizations(id),
    -- Registration details
    status          VARCHAR(20)  NOT NULL DEFAULT 'REGISTERED'
                    CHECK (status IN ('REGISTERED', 'ADMIT_CARD_GENERATED',
                           'APPEARED', 'ABSENT', 'CANCELLED', 'DISQUALIFIED')),
    -- Center allocation
    center_id       UUID,               -- Allocated exam center
    shift_id        UUID,               -- Allocated shift/session
    seat_number     VARCHAR(20),
    -- Admit card
    admit_card_url  VARCHAR(500),
    admit_card_hash BYTEA,              -- SHA-256 of admit card PDF
    qr_code_data    BYTEA,              -- Encrypted QR code payload
    -- Fees
    fee_paid        BOOLEAN NOT NULL DEFAULT FALSE,
    fee_amount      DECIMAL(10,2),
    payment_ref     VARCHAR(100),
    -- Metadata
    registered_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_registration UNIQUE (candidate_id, exam_id)
);

CREATE INDEX idx_registrations_exam ON registrations (exam_id, status);
CREATE INDEX idx_registrations_center ON registrations (center_id) WHERE center_id IS NOT NULL;
CREATE INDEX idx_registrations_candidate ON registrations (candidate_id);

-- ──────────────────────────────────────────────
--  Exam Centers
-- ──────────────────────────────────────────────
CREATE TABLE centers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID         NOT NULL REFERENCES organizations(id),
    center_code     VARCHAR(50)  NOT NULL,
    name            VARCHAR(200) NOT NULL,
    address         JSONB        NOT NULL,       -- Structured address
    city            VARCHAR(100) NOT NULL,
    state           VARCHAR(100) NOT NULL,
    pin_code        VARCHAR(10)  NOT NULL,
    -- Coordinates for geographic allocation
    latitude        DOUBLE PRECISION,
    longitude       DOUBLE PRECISION,
    -- Capacity
    total_capacity  INTEGER NOT NULL CHECK (total_capacity > 0),
    computer_count  INTEGER NOT NULL CHECK (computer_count > 0),
    -- Infrastructure
    internet_type   VARCHAR(20) CHECK (internet_type IN ('FIBER', 'BROADBAND', 'SATELLITE', 'NONE')),
    backup_power    BOOLEAN NOT NULL DEFAULT FALSE,
    cctv_enabled    BOOLEAN NOT NULL DEFAULT FALSE,
    -- Status
    status          VARCHAR(20) NOT NULL DEFAULT 'ACTIVE'
                    CHECK (status IN ('ACTIVE', 'INACTIVE', 'UNDER_INSPECTION', 'BLACKLISTED')),
    last_inspection TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_center_code UNIQUE (organization_id, center_code)
);

CREATE INDEX idx_centers_org ON centers (organization_id, status);
CREATE INDEX idx_centers_geo ON centers (latitude, longitude) WHERE status = 'ACTIVE';
CREATE INDEX idx_centers_city ON centers (state, city) WHERE status = 'ACTIVE';

-- RLS
ALTER TABLE candidates ENABLE ROW LEVEL SECURITY;
CREATE POLICY candidates_org_isolation ON candidates
    USING (organization_id = current_setting('app.current_organization_id')::UUID);

ALTER TABLE registrations ENABLE ROW LEVEL SECURITY;
CREATE POLICY registrations_org_isolation ON registrations
    USING (organization_id = current_setting('app.current_organization_id')::UUID);

ALTER TABLE centers ENABLE ROW LEVEL SECURITY;
CREATE POLICY centers_org_isolation ON centers
    USING (organization_id = current_setting('app.current_organization_id')::UUID);

-- Triggers
CREATE TRIGGER trg_candidates_updated_at BEFORE UPDATE ON candidates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER trg_registrations_updated_at BEFORE UPDATE ON registrations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER trg_centers_updated_at BEFORE UPDATE ON centers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
