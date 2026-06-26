-- ============================================================
-- 运维 AI Agent 初始数据库 Schema
-- PostgreSQL 15+
-- Version: 000001
-- ============================================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- 1. 用户与多租户
CREATE TABLE users (
    user_id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username        VARCHAR(64) NOT NULL UNIQUE,
    email           VARCHAR(255) NOT NULL UNIQUE,
    role            VARCHAR(16) NOT NULL DEFAULT 'viewer' CHECK (role IN ('admin', 'operator', 'viewer')),
    oidc_claims     JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email ON users(email);

CREATE TABLE teams (
    team_id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name                VARCHAR(128) NOT NULL UNIQUE,
    allowed_namespaces  JSONB NOT NULL DEFAULT '[]',
    denied_namespaces   JSONB NOT NULL DEFAULT '[]',
    llm_budget_monthly  BIGINT NOT NULL DEFAULT 5000000,
    default_config      JSONB NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE team_memberships (
    membership_id   UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    team_id         UUID NOT NULL REFERENCES teams(team_id) ON DELETE CASCADE,
    role            VARCHAR(16) NOT NULL DEFAULT 'member' CHECK (role IN ('admin', 'member', 'viewer')),
    joined_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, team_id)
);
CREATE INDEX idx_team_memberships_team ON team_memberships(team_id);
CREATE INDEX idx_team_memberships_user ON team_memberships(user_id);

CREATE TABLE namespace_acls (
    acl_id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    team_id             UUID NOT NULL REFERENCES teams(team_id) ON DELETE CASCADE,
    namespace_pattern   VARCHAR(255) NOT NULL,
    access_type         VARCHAR(8) NOT NULL CHECK (access_type IN ('allow', 'deny')),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(team_id, namespace_pattern)
);
CREATE INDEX idx_namespace_acls_team ON namespace_acls(team_id);

-- 2. 会话管理
CREATE TABLE sessions (
    session_id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    parent_session_id   UUID NULL REFERENCES sessions(session_id),
    cluster             VARCHAR(128) NOT NULL DEFAULT 'default',
    namespace           VARCHAR(128) NOT NULL DEFAULT 'default',
    environment         VARCHAR(32) NOT NULL DEFAULT 'production',
    status              VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'crashed', 'completed', 'exported', 'imported')),
    user_id             UUID REFERENCES users(user_id),
    team_id             UUID REFERENCES teams(team_id),
    incident_id         UUID,
    conversation_history JSONB NOT NULL DEFAULT '[]',
    tool_call_history   JSONB NOT NULL DEFAULT '[]',
    metadata            JSONB NOT NULL DEFAULT '{}',
    cost_usd_milli      BIGINT NOT NULL DEFAULT 0,
    token_budget_used   BIGINT NOT NULL DEFAULT 0,
    token_budget_limit  BIGINT NOT NULL DEFAULT 100000,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at          TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '24 hours')
);
CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_team ON sessions(team_id);
CREATE INDEX idx_sessions_status ON sessions(status);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);
CREATE INDEX idx_sessions_parent ON sessions(parent_session_id);

-- 3. 审计与证据链
CREATE TABLE audit_events (
    event_id            BIGSERIAL PRIMARY KEY,
    session_id          UUID REFERENCES sessions(session_id),
    user_id             UUID REFERENCES users(user_id),
    cluster             VARCHAR(128) NOT NULL,
    namespace           VARCHAR(128) NOT NULL,
    action              VARCHAR(64) NOT NULL,
    tool_name           VARCHAR(128),
    risk_level          VARCHAR(4) NOT NULL CHECK (risk_level IN ('L0', 'L1', 'L2', 'L3', 'L4')),
    target              JSONB NOT NULL DEFAULT '{}',
    pre_check           JSONB NOT NULL DEFAULT '{}',
    impact_analysis     JSONB NOT NULL DEFAULT '{}',
    result              JSONB NOT NULL DEFAULT '{}',
    rollback_info       JSONB,
    approval            JSONB,
    evidence_chain_hash VARCHAR(64),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_audit_events_session ON audit_events(session_id);
CREATE INDEX idx_audit_events_user ON audit_events(user_id);
CREATE INDEX idx_audit_events_created ON audit_events(created_at);
CREATE INDEX idx_audit_events_cluster_ns ON audit_events(cluster, namespace);
CREATE INDEX idx_audit_events_risk ON audit_events(risk_level);
CREATE INDEX idx_audit_events_action ON audit_events(action);

CREATE TABLE evidence_records (
    evidence_id     BIGSERIAL PRIMARY KEY,
    event_id        BIGINT NOT NULL REFERENCES audit_events(event_id) ON DELETE CASCADE,
    evidence_type   VARCHAR(32) NOT NULL CHECK (evidence_type IN (
        'decision_context', 'system_state', 'resource_snapshot',
        'metrics_query', 'risk_assessment', 'approval'
    )),
    content         JSONB NOT NULL,
    content_hash    VARCHAR(64) NOT NULL,
    prev_hash       VARCHAR(64),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_evidence_event ON evidence_records(event_id);
CREATE INDEX idx_evidence_type ON evidence_records(evidence_type);
CREATE INDEX idx_evidence_created ON evidence_records(created_at);

-- 触发器: 更新时间戳
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_sessions_updated_at BEFORE UPDATE ON sessions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 触发器: 审计事件哈希链
CREATE OR REPLACE FUNCTION compute_evidence_hash()
RETURNS TRIGGER AS $$
DECLARE
    prev_hash_val VARCHAR(64);
BEGIN
    SELECT content_hash INTO prev_hash_val
    FROM evidence_records
    WHERE event_id = NEW.event_id AND evidence_id < NEW.evidence_id
    ORDER BY evidence_id DESC LIMIT 1;

    NEW.prev_hash := COALESCE(prev_hash_val, '');
    NEW.content_hash := encode(digest(NEW.content::text || NEW.prev_hash, 'sha256'), 'hex');
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER trg_evidence_hash BEFORE INSERT ON evidence_records
    FOR EACH ROW EXECUTE FUNCTION compute_evidence_hash();
