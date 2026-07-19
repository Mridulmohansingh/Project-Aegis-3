-- Migration 001: Core schema — Organizations, Users, Roles
-- This establishes the multi-tenant foundation for AEGIS.
-- All subsequent tables reference organizations and users for RLS enforcement.

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ──────────────────────────────────────────────
--  Organizations (Tenants)
-- ──────────────────────────────────────────────
CREATE TABLE organizations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code        VARCHAR(50)  NOT NULL UNIQUE,       -- e.g., 'NTA', 'SSC', 'UPSC'
    name        VARCHAR(200) NOT NULL,
    description TEXT,
    status      VARCHAR(20)  NOT NULL DEFAULT 'ACTIVE'
                CHECK (status IN ('ACTIVE', 'SUSPENDED', 'DEACTIVATED')),
    config      JSONB        NOT NULL DEFAULT '{}',  -- Org-specific settings
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_organizations_code ON organizations (code);
CREATE INDEX idx_organizations_status ON organizations (status) WHERE status = 'ACTIVE';

-- ──────────────────────────────────────────────
--  Roles
-- ──────────────────────────────────────────────
CREATE TABLE roles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code        VARCHAR(50) NOT NULL UNIQUE,     -- e.g., 'ITEM_AUTHOR', 'REVIEWER', 'PSYCHOMETRICIAN'
    name        VARCHAR(100) NOT NULL,
    description TEXT,
    permissions JSONB NOT NULL DEFAULT '[]',      -- List of permission strings
    is_system   BOOLEAN NOT NULL DEFAULT FALSE,   -- System roles cannot be deleted
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed system roles
INSERT INTO roles (id, code, name, description, permissions, is_system) VALUES
    (gen_random_uuid(), 'SUPER_ADMIN',     'Super Administrator',   'Full system access',
     '["*"]', TRUE),
    (gen_random_uuid(), 'ORG_ADMIN',       'Organization Admin',    'Full access within organization',
     '["org:*"]', TRUE),
    (gen_random_uuid(), 'ITEM_AUTHOR',     'Item Author',           'Can create and edit draft items',
     '["item:create", "item:read_own", "item:update_own", "item:submit_review"]', TRUE),
    (gen_random_uuid(), 'REVIEWER',        'Content Reviewer',      'Can review items for content quality',
     '["item:read_assigned", "item:review"]', TRUE),
    (gen_random_uuid(), 'PSYCHOMETRICIAN', 'Psychometrician',       'Can calibrate items with IRT parameters',
     '["item:read_assigned", "item:calibrate", "item:read_stats"]', TRUE),
    (gen_random_uuid(), 'APPROVER',        'Final Approver',        'Can give final approval to items',
     '["item:read", "item:approve"]', TRUE),
    (gen_random_uuid(), 'EXAM_ADMIN',      'Exam Administrator',    'Can configure and schedule exams',
     '["exam:*", "blueprint:*", "paper:generate", "center:*"]', TRUE),
    (gen_random_uuid(), 'SCORER',          'Score Processor',       'Can trigger and verify scoring',
     '["score:process", "score:verify", "result:read"]', TRUE),
    (gen_random_uuid(), 'PROCTOR',         'Proctor',               'Can monitor live exams',
     '["exam:monitor", "incident:create", "incident:read"]', TRUE),
    (gen_random_uuid(), 'AUDITOR',         'Auditor',               'Read-only access to audit logs',
     '["audit:read"]', TRUE);

-- ──────────────────────────────────────────────
--  Users
-- ──────────────────────────────────────────────
CREATE TABLE users (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  UUID         NOT NULL REFERENCES organizations(id),
    email            VARCHAR(255) NOT NULL,
    display_name     VARCHAR(200) NOT NULL,
    -- Authentication is handled by Keycloak; we store the external ID
    external_auth_id VARCHAR(255),          -- Keycloak subject ID
    status           VARCHAR(20) NOT NULL DEFAULT 'ACTIVE'
                     CHECK (status IN ('ACTIVE', 'INACTIVE', 'LOCKED', 'PENDING_VERIFICATION')),
    mfa_enabled      BOOLEAN NOT NULL DEFAULT FALSE,
    last_login_at    TIMESTAMPTZ,
    login_count      INTEGER NOT NULL DEFAULT 0,
    failed_attempts  INTEGER NOT NULL DEFAULT 0,
    locked_until     TIMESTAMPTZ,
    metadata         JSONB NOT NULL DEFAULT '{}',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_user_email_org UNIQUE (organization_id, email)
);

CREATE INDEX idx_users_org ON users (organization_id) WHERE status = 'ACTIVE';
CREATE INDEX idx_users_external_auth ON users (external_auth_id) WHERE external_auth_id IS NOT NULL;
CREATE INDEX idx_users_email ON users (email);

-- ──────────────────────────────────────────────
--  User-Role Assignment
-- ──────────────────────────────────────────────
CREATE TABLE user_roles (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id     UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    -- Scoped assignments (e.g., reviewer for specific subjects)
    scope       JSONB NOT NULL DEFAULT '{}',   -- {"subject_ids": ["..."], "center_ids": ["..."]}
    granted_by  UUID NOT NULL REFERENCES users(id),
    granted_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ,                   -- Optional role expiry

    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX idx_user_roles_role ON user_roles (role_id);

-- ──────────────────────────────────────────────
--  Row-Level Security Policies
-- ──────────────────────────────────────────────
-- RLS ensures multi-tenant isolation at the database level.
-- Applications must SET app.current_organization_id before queries.

ALTER TABLE users ENABLE ROW LEVEL SECURITY;

CREATE POLICY users_org_isolation ON users
    USING (organization_id = current_setting('app.current_organization_id')::UUID);

-- ──────────────────────────────────────────────
--  Audit Trigger Function
-- ──────────────────────────────────────────────
-- Automatically updates updated_at on row modification.
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_organizations_updated_at
    BEFORE UPDATE ON organizations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
