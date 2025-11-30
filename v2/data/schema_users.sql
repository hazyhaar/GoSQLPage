-- GoSQLPage v2.1 - Users Database Schema
-- Contains authentication and permissions

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id              TEXT PRIMARY KEY,
    type            TEXT NOT NULL DEFAULT 'human', -- human, bot, system
    username        TEXT NOT NULL UNIQUE,
    email           TEXT,
    password_hash   TEXT,                          -- nullable for bots
    config          TEXT,                          -- JSON: preferences, bot settings
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    last_login      TEXT,
    status          TEXT NOT NULL DEFAULT 'active' -- active, suspended, deleted
);

-- Permissions table
CREATE TABLE IF NOT EXISTS permissions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         TEXT NOT NULL,                 -- FK users, or * for all
    scope           TEXT NOT NULL,                 -- global, tenant, document, block, type
    scope_id        TEXT,                          -- nullable if global
    action          TEXT NOT NULL,                 -- read, edit, delete, publish, merge, admin
    granted         INTEGER NOT NULL DEFAULT 1,   -- 1 = allow, 0 = deny
    granted_by      TEXT,                          -- FK users
    granted_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    expires_at      TEXT,                          -- nullable, for temporary permissions
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- API keys for bots and external access
CREATE TABLE IF NOT EXISTS api_keys (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id),
    name            TEXT NOT NULL,
    key_hash        TEXT NOT NULL,                 -- hashed API key
    scopes          TEXT,                          -- JSON: allowed scopes
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    last_used       TEXT,
    expires_at      TEXT,
    revoked_at      TEXT
);

-- Sessions for HTTP auth
CREATE TABLE IF NOT EXISTS http_sessions (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id),
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    expires_at      TEXT NOT NULL,
    ip_address      TEXT,
    user_agent      TEXT
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_permissions_user ON permissions(user_id, scope, action);
CREATE INDEX IF NOT EXISTS idx_permissions_scope ON permissions(scope, scope_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_user ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_http_sessions_user ON http_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_http_sessions_expires ON http_sessions(expires_at);

-- Default system user
INSERT OR IGNORE INTO users (id, type, username, status) VALUES
    ('system', 'system', 'system', 'active');

-- Default admin user (password should be changed on first login)
INSERT OR IGNORE INTO users (id, type, username, email, status) VALUES
    ('admin', 'human', 'admin', 'admin@localhost', 'active');

-- Grant admin all permissions
INSERT OR IGNORE INTO permissions (user_id, scope, action, granted_by) VALUES
    ('admin', 'global', 'admin', 'system'),
    ('admin', 'global', 'read', 'system'),
    ('admin', 'global', 'edit', 'system'),
    ('admin', 'global', 'delete', 'system'),
    ('admin', 'global', 'publish', 'system'),
    ('admin', 'global', 'merge', 'system');

-- Grant all users read permission by default
INSERT OR IGNORE INTO permissions (user_id, scope, action, granted_by) VALUES
    ('*', 'global', 'read', 'system');
