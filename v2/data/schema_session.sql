-- GoSQLPage v2.1 - Session Database Schema
-- Each session.db is autonomous and contains working copies

-- Session metadata
CREATE TABLE IF NOT EXISTS _session_meta (
    session_id      TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL,
    user_type       TEXT NOT NULL DEFAULT 'human', -- human, bot, system
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    last_activity   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    base_snapshot   TEXT NOT NULL,                 -- hash of content.db at creation
    schema_version  INTEGER NOT NULL,              -- version of schema.db at creation
    schema_hash     TEXT NOT NULL,                 -- hash of schema.db
    status          TEXT NOT NULL DEFAULT 'active' -- active, submitted, merged, abandoned, conflict
);

-- Change journal - tracks all modifications
CREATE TABLE IF NOT EXISTS _changes (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    operation       TEXT NOT NULL,                 -- insert, update, delete, link, unlink
    block_id        TEXT NOT NULL,
    field           TEXT,                          -- null for full block ops, field name for partial
    before          TEXT,                          -- JSON snapshot or value before
    after           TEXT,                          -- JSON snapshot or value after
    merged          INTEGER NOT NULL DEFAULT 0     -- 0 = pending, 1 = merged
);

-- Structural dependencies for conflict detection
CREATE TABLE IF NOT EXISTS _structural_deps (
    block_id        TEXT PRIMARY KEY,
    depends_on      TEXT NOT NULL,                 -- JSON: [parent_id, refs.to_id, ...]
    snapshot_hashes TEXT NOT NULL                  -- JSON: {dep_id: hash, ...}
);

-- Working copy of blocks (same schema as content.db + _dirty flag)
CREATE TABLE IF NOT EXISTS blocks (
    id              TEXT PRIMARY KEY,
    parent_id       TEXT,
    type            TEXT NOT NULL,
    content         TEXT NOT NULL DEFAULT '',
    content_html    TEXT,
    position        TEXT NOT NULL DEFAULT 'a',
    hash            TEXT NOT NULL,
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    created_by      TEXT NOT NULL,
    published       INTEGER NOT NULL DEFAULT 0,
    deleted_at      TEXT,
    _dirty          INTEGER NOT NULL DEFAULT 0,    -- 0 = clean, 1 = modified locally
    _source         TEXT NOT NULL DEFAULT 'new'   -- 'new' = created in session, 'copy' = copied from content.db
);

-- Working copy of refs
CREATE TABLE IF NOT EXISTS refs (
    from_id         TEXT NOT NULL,
    to_id           TEXT NOT NULL,
    type            TEXT NOT NULL,
    anchor          TEXT,
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    created_by      TEXT NOT NULL,
    _dirty          INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (from_id, to_id, type)
);

-- Working copy of attrs
CREATE TABLE IF NOT EXISTS attrs (
    block_id        TEXT NOT NULL,
    name            TEXT NOT NULL,
    value           TEXT NOT NULL,
    _dirty          INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (block_id, name)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_session_blocks_dirty ON blocks(_dirty);
CREATE INDEX IF NOT EXISTS idx_session_blocks_parent ON blocks(parent_id);
CREATE INDEX IF NOT EXISTS idx_session_refs_dirty ON refs(_dirty);
CREATE INDEX IF NOT EXISTS idx_session_attrs_dirty ON attrs(_dirty);
CREATE INDEX IF NOT EXISTS idx_changes_block ON _changes(block_id);
CREATE INDEX IF NOT EXISTS idx_changes_merged ON _changes(merged);
