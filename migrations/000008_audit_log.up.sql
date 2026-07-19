-- Migration 008: Audit Log (Range-Partitioned, Append-Only)
-- Tamper-evident audit trail with Merkle chain integrity.
-- Partitioned by time for efficient queries and archival.

CREATE TABLE audit_log (
    id              BIGSERIAL,
    event_time      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    event_type      VARCHAR(50)  NOT NULL,       -- e.g., 'ITEM_CREATED', 'PAPER_GENERATED'
    actor_id        UUID         NOT NULL,
    actor_type      VARCHAR(20)  NOT NULL
                    CHECK (actor_type IN ('USER', 'SYSTEM', 'SERVICE')),
    actor_ip        INET,
    resource_type   VARCHAR(50)  NOT NULL,       -- e.g., 'item', 'paper', 'exam', 'response'
    resource_id     UUID         NOT NULL,
    organization_id UUID         NOT NULL,
    action          VARCHAR(50)  NOT NULL,       -- e.g., 'CREATE', 'UPDATE', 'TRANSITION'
    detail          JSONB,                        -- Action-specific details

    -- Merkle chain integrity
    previous_hash   BYTEA        NOT NULL,       -- SHA-256 of previous entry
    entry_hash      BYTEA        NOT NULL,       -- SHA-256(previous_hash || serialized_data)

    -- Periodic checkpoint (every 1000 entries)
    checkpoint_sig  BYTEA,                        -- Ed25519 signature of Merkle root

    PRIMARY KEY (event_time, id)
) PARTITION BY RANGE (event_time);

-- Create monthly partitions for the current year and next year
-- In production, use pg_partman for automated partition management
DO $$
DECLARE
    start_date DATE;
    end_date DATE;
    partition_name TEXT;
BEGIN
    FOR yr IN 2026..2027 LOOP
        FOR mo IN 1..12 LOOP
            start_date := make_date(yr, mo, 1);
            end_date := start_date + INTERVAL '1 month';
            partition_name := format('audit_log_y%sm%s', yr, lpad(mo::text, 2, '0'));

            EXECUTE format(
                'CREATE TABLE %I PARTITION OF audit_log FOR VALUES FROM (%L) TO (%L)',
                partition_name, start_date, end_date
            );
        END LOOP;
    END LOOP;
END$$;

-- Default partition for dates outside defined ranges
CREATE TABLE audit_log_default PARTITION OF audit_log DEFAULT;

-- Indexes
CREATE INDEX idx_audit_event_type ON audit_log (event_type, event_time DESC);
CREATE INDEX idx_audit_actor ON audit_log (actor_id, event_time DESC);
CREATE INDEX idx_audit_resource ON audit_log (resource_type, resource_id, event_time DESC);
CREATE INDEX idx_audit_org ON audit_log (organization_id, event_time DESC);
CREATE INDEX idx_audit_action ON audit_log (action, event_time DESC);

-- ──────────────────────────────────────────────
--  Prevent Modification of Audit Entries
-- ──────────────────────────────────────────────
-- This trigger prevents ANY update or delete on the audit log.
-- Even database administrators cannot modify historical entries.
-- Only the AEGIS superuser can disable this trigger for emergency recovery.

CREATE OR REPLACE FUNCTION prevent_audit_modification()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_log is append-only: UPDATE and DELETE operations are prohibited. '
                    'Audit integrity must be maintained at all times. '
                    'If this is an emergency, contact the security team.';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_audit_immutable
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_modification();

-- ──────────────────────────────────────────────
--  Audit Integrity Checkpoints
-- ──────────────────────────────────────────────
-- Stores periodic Merkle root checkpoints for efficient verification.
-- These can be cross-checked against external witnesses (e.g., timestamping service).
CREATE TABLE audit_checkpoints (
    id              BIGSERIAL PRIMARY KEY,
    checkpoint_time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    start_entry_id  BIGINT NOT NULL,
    end_entry_id    BIGINT NOT NULL,
    merkle_root     BYTEA NOT NULL,              -- SHA-256 root of checkpoint range
    signature       BYTEA NOT NULL,              -- Ed25519 signature of merkle_root
    signing_key_id  VARCHAR(100) NOT NULL,       -- Key ID used for signing
    entry_count     INTEGER NOT NULL,
    -- External witness (optional)
    witness_url     VARCHAR(500),                -- External timestamping service URL
    witness_receipt BYTEA                        -- Receipt from timestamping service
);

-- ──────────────────────────────────────────────
--  Revoke dangerous permissions on audit tables
-- ──────────────────────────────────────────────
-- These REVOKE statements should be run by the DBA after deployment:
--
-- REVOKE UPDATE, DELETE ON audit_log FROM aegis_app;
-- REVOKE UPDATE, DELETE ON audit_checkpoints FROM aegis_app;
-- GRANT INSERT, SELECT ON audit_log TO aegis_app;
-- GRANT INSERT, SELECT ON audit_checkpoints TO aegis_app;
