-- GoSQLPage v2.1 - Audit Database Schema
-- Contains logs of all mutations for traceability

-- Audit configuration
CREATE TABLE IF NOT EXISTS audit_config (
    key             TEXT PRIMARY KEY,
    value           TEXT NOT NULL
);

-- Default audit configuration
INSERT OR IGNORE INTO audit_config (key, value) VALUES
    ('store_content', 'false'),                    -- store full content in logs
    ('store_content_types', '["code","definition","procedure"]'), -- types to store content for
    ('retention_days', '90'),                      -- how long to keep logs
    ('archive_after_days', '30');                  -- when to move to archive

-- Main audit log
CREATE TABLE IF NOT EXISTS audit_log (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    user_id         TEXT NOT NULL,
    user_type       TEXT NOT NULL,                 -- human, bot, system
    session_id      TEXT,                          -- editing session that produced this change
    operation       TEXT NOT NULL,                 -- insert, update, delete, merge, publish
    block_id        TEXT NOT NULL,
    block_type      TEXT NOT NULL,
    before_hash     TEXT,                          -- hash before change
    after_hash      TEXT,                          -- hash after change
    before_content  TEXT,                          -- content before (if config allows)
    after_content   TEXT,                          -- content after (if config allows)
    diff            TEXT,                          -- JSON diff (compact alternative)
    metadata        TEXT                           -- JSON: IP, user-agent, etc.
);

-- Merge log - tracks merge operations
CREATE TABLE IF NOT EXISTS merge_log (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    session_id      TEXT NOT NULL,
    user_id         TEXT NOT NULL,
    status          TEXT NOT NULL,                 -- success, conflict, failed
    blocks_inserted INTEGER NOT NULL DEFAULT 0,
    blocks_updated  INTEGER NOT NULL DEFAULT 0,
    blocks_deleted  INTEGER NOT NULL DEFAULT 0,
    duration_ms     INTEGER,
    error_message   TEXT,
    metadata        TEXT                           -- JSON: additional details
);

-- Indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_audit_block ON audit_log(block_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_log(user_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_time ON audit_log(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_operation ON audit_log(operation, timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_session ON audit_log(session_id);
CREATE INDEX IF NOT EXISTS idx_merge_time ON merge_log(timestamp);
CREATE INDEX IF NOT EXISTS idx_merge_user ON merge_log(user_id);
CREATE INDEX IF NOT EXISTS idx_merge_session ON merge_log(session_id);
