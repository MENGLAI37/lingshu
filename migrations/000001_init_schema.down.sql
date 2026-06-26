-- ============================================================
-- 回滚初始数据库 Schema
-- Version: 000001
-- ============================================================

DROP TRIGGER IF EXISTS trg_evidence_hash ON evidence_records;
DROP FUNCTION IF EXISTS compute_evidence_hash();

DROP TRIGGER IF EXISTS update_sessions_updated_at ON sessions;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
DROP FUNCTION IF EXISTS update_updated_at_column();

DROP TABLE IF EXISTS evidence_records;
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS namespace_acls;
DROP TABLE IF EXISTS team_memberships;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS users;

DROP EXTENSION IF EXISTS "pgcrypto";
DROP EXTENSION IF EXISTS "uuid-ossp";
