-- GoSQLPage v2.1 - Content Database Schema
-- Contains published blocks, relations, and vectors

-- Main blocks table
CREATE TABLE IF NOT EXISTS blocks (
    id              TEXT PRIMARY KEY,           -- nanoid or uuid
    parent_id       TEXT REFERENCES blocks(id), -- nullable for root blocks
    type            TEXT NOT NULL,              -- paragraph, heading, code, claim, question, etc.
    content         TEXT NOT NULL DEFAULT '',   -- markdown source
    content_html    TEXT,                       -- rendered HTML (cache)
    position        TEXT NOT NULL DEFAULT 'a',  -- fractional indexing for ordering
    hash            TEXT NOT NULL,              -- SHA256 of content for dedup/diff
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    created_by      TEXT NOT NULL,              -- user_id
    published       INTEGER NOT NULL DEFAULT 0, -- 0 = draft, 1 = public
    deleted_at      TEXT                        -- soft delete timestamp
);

-- Relations between blocks
CREATE TABLE IF NOT EXISTS refs (
    from_id         TEXT NOT NULL REFERENCES blocks(id),
    to_id           TEXT NOT NULL REFERENCES blocks(id),
    type            TEXT NOT NULL,              -- cites, refutes, extends, depends, parent_of, etc.
    anchor          TEXT,                       -- position in source block (optional)
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    created_by      TEXT NOT NULL,
    PRIMARY KEY (from_id, to_id, type)
);

-- Custom attributes for blocks
CREATE TABLE IF NOT EXISTS attrs (
    block_id        TEXT NOT NULL REFERENCES blocks(id),
    name            TEXT NOT NULL,
    value           TEXT NOT NULL,
    PRIMARY KEY (block_id, name)
);

-- Full-text search index
CREATE VIRTUAL TABLE IF NOT EXISTS blocks_fts USING fts5(
    id,
    type,
    content,
    content='blocks',
    content_rowid='rowid'
);

-- Triggers to keep FTS in sync
CREATE TRIGGER IF NOT EXISTS blocks_fts_insert AFTER INSERT ON blocks BEGIN
    INSERT INTO blocks_fts(id, type, content) VALUES (new.id, new.type, new.content);
END;

CREATE TRIGGER IF NOT EXISTS blocks_fts_delete AFTER DELETE ON blocks BEGIN
    INSERT INTO blocks_fts(blocks_fts, id, type, content) VALUES('delete', old.id, old.type, old.content);
END;

CREATE TRIGGER IF NOT EXISTS blocks_fts_update AFTER UPDATE ON blocks BEGIN
    INSERT INTO blocks_fts(blocks_fts, id, type, content) VALUES('delete', old.id, old.type, old.content);
    INSERT INTO blocks_fts(id, type, content) VALUES (new.id, new.type, new.content);
END;

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_blocks_parent_position ON blocks(parent_id, position);
CREATE INDEX IF NOT EXISTS idx_blocks_type ON blocks(type);
CREATE INDEX IF NOT EXISTS idx_blocks_updated ON blocks(updated_at);
CREATE INDEX IF NOT EXISTS idx_blocks_hash ON blocks(hash);
CREATE INDEX IF NOT EXISTS idx_blocks_published ON blocks(published) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_refs_to ON refs(to_id);
CREATE INDEX IF NOT EXISTS idx_attrs_name_value ON attrs(name, value);

-- Vector embeddings for RAG (optional)
CREATE TABLE IF NOT EXISTS blocks_vectors (
    block_id        TEXT PRIMARY KEY REFERENCES blocks(id),
    vector          BLOB NOT NULL,              -- float32[]
    model_version   TEXT NOT NULL,
    updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
