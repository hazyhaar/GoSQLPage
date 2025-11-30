-- GoSQLPage v2.1 - Schema Database
-- Contains block type definitions and validation rules

-- Block type definitions
CREATE TABLE IF NOT EXISTS block_types (
    name                TEXT PRIMARY KEY,
    label               TEXT NOT NULL,          -- display label
    icon                TEXT,                   -- emoji or CSS class
    schema              TEXT,                   -- JSON: required fields, validations
    render_template     TEXT,                   -- HTML template or component name
    allowed_children    TEXT,                   -- JSON: allowed child types
    allowed_parents     TEXT,                   -- JSON: allowed parent types
    category            TEXT NOT NULL DEFAULT 'content', -- content, discussion, knowledge, task, bot, system
    version             INTEGER NOT NULL DEFAULT 1
);

-- Schema version tracking
CREATE TABLE IF NOT EXISTS schema_version (
    id                  INTEGER PRIMARY KEY CHECK (id = 1),
    version             INTEGER NOT NULL DEFAULT 1,
    hash                TEXT NOT NULL,          -- hash of all type definitions
    updated_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Relation type definitions
CREATE TABLE IF NOT EXISTS relation_types (
    name                TEXT PRIMARY KEY,
    label               TEXT NOT NULL,
    inverse             TEXT,                   -- inverse relation name
    from_types          TEXT,                   -- JSON: allowed source types
    to_types            TEXT,                   -- JSON: allowed target types
    symmetric           INTEGER NOT NULL DEFAULT 0
);

-- Validation rules
CREATE TABLE IF NOT EXISTS validation_rules (
    id                  TEXT PRIMARY KEY,
    target              TEXT NOT NULL,          -- block_type or * for all
    rule                TEXT NOT NULL,          -- SQL expression or JSON schema
    message             TEXT NOT NULL,          -- error message if invalid
    severity            TEXT NOT NULL DEFAULT 'error' -- error, warning, info
);

-- Initialize schema version
INSERT OR IGNORE INTO schema_version (id, version, hash) VALUES (1, 1, 'initial');

-- Default block types (Category: Content)
INSERT OR IGNORE INTO block_types (name, label, icon, category, allowed_children, allowed_parents) VALUES
    ('document', 'Document', '📄', 'content', '["heading","paragraph","list","code","table","embed","quote"]', '[]'),
    ('heading', 'Heading', '📌', 'content', '[]', '["document"]'),
    ('paragraph', 'Paragraph', '📝', 'content', '[]', '["document","list_item","quote"]'),
    ('list', 'List', '📋', 'content', '["list_item"]', '["document","list_item"]'),
    ('list_item', 'List Item', '•', 'content', '["paragraph","list"]', '["list"]'),
    ('code', 'Code Block', '💻', 'content', '[]', '["document"]'),
    ('table', 'Table', '📊', 'content', '["table_row"]', '["document"]'),
    ('quote', 'Quote', '💬', 'content', '["paragraph"]', '["document"]'),
    ('embed', 'Embed', '🔗', 'content', '[]', '["document"]');

-- Category: Discussion
INSERT OR IGNORE INTO block_types (name, label, icon, category, schema) VALUES
    ('question', 'Question', '❓', 'discussion', '{"attrs":{"status":{"type":"string","enum":["open","answered","closed"],"default":"open"}}}'),
    ('answer', 'Answer', '✅', 'discussion', '{"attrs":{"accepted":{"type":"boolean","default":false},"score":{"type":"integer","default":0}}}'),
    ('claim', 'Claim', '💡', 'discussion', '{"attrs":{"confidence":{"type":"integer","min":0,"max":100}}}'),
    ('counter', 'Counter-argument', '⚔️', 'discussion', '{"attrs":{"target_claim":{"type":"string"}}}'),
    ('evidence', 'Evidence', '📚', 'discussion', '{"attrs":{"url":{"type":"string"},"citation":{"type":"string"}}}');

-- Category: Knowledge
INSERT OR IGNORE INTO block_types (name, label, icon, category) VALUES
    ('definition', 'Definition', '📖', 'knowledge'),
    ('procedure', 'Procedure', '📋', 'knowledge'),
    ('decision', 'Decision', '⚖️', 'knowledge'),
    ('lesson', 'Lesson Learned', '🎓', 'knowledge');

-- Category: Tasks
INSERT OR IGNORE INTO block_types (name, label, icon, category, schema) VALUES
    ('task', 'Task', '☑️', 'task', '{"attrs":{"status":{"type":"string","enum":["todo","in_progress","done","blocked"],"default":"todo"},"assignee":{"type":"string"},"due_date":{"type":"string"},"priority":{"type":"string","enum":["low","medium","high","critical"]}}}'),
    ('milestone', 'Milestone', '🏁', 'task', '{"attrs":{"target_date":{"type":"string"}}}'),
    ('blocker', 'Blocker', '🚧', 'task', '{"attrs":{"blocking":{"type":"array"}}}');

-- Category: Bot/LLM
INSERT OR IGNORE INTO block_types (name, label, icon, category, schema) VALUES
    ('bot_request', 'Bot Request', '🤖', 'bot', '{"attrs":{"bot_id":{"type":"string"},"status":{"type":"string","enum":["pending","processing","completed","failed"],"default":"pending"},"context_blocks":{"type":"array"}}}'),
    ('bot_response', 'Bot Response', '💭', 'bot', '{"attrs":{"model":{"type":"string"},"tokens_used":{"type":"integer"},"duration_ms":{"type":"integer"}}}'),
    ('bot_reasoning', 'Bot Reasoning', '🧠', 'bot', '{"attrs":{"steps":{"type":"array"}}}');

-- Default relation types
INSERT OR IGNORE INTO relation_types (name, label, inverse, symmetric) VALUES
    ('parent_of', 'Parent of', 'child_of', 0),
    ('references', 'References', 'referenced_by', 0),
    ('cites', 'Cites', 'cited_by', 0),
    ('refutes', 'Refutes', 'refuted_by', 0),
    ('extends', 'Extends', 'extended_by', 0),
    ('summarizes', 'Summarizes', 'summarized_by', 0),
    ('depends', 'Depends on', 'dependency_of', 0),
    ('supersedes', 'Supersedes', 'superseded_by', 0),
    ('triggers', 'Triggers', 'triggered_by', 0),
    ('answers', 'Answers', 'answered_by', 0),
    ('blocks', 'Blocks', 'blocked_by', 0),
    ('related_to', 'Related to', 'related_to', 1);
