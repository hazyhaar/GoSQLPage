-- Forum Schema for GoSQLPage
-- Run this once to initialize the forum database

-- Users table
CREATE TABLE IF NOT EXISTS forum_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    display_name TEXT,
    avatar_url TEXT,
    bio TEXT,
    role TEXT DEFAULT 'member' CHECK(role IN ('admin', 'moderator', 'member', 'banned')),
    post_count INTEGER DEFAULT 0,
    reputation INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    last_seen_at TEXT,
    email_verified INTEGER DEFAULT 0,
    settings_json TEXT DEFAULT '{}'
);

-- Sessions table
CREATE TABLE IF NOT EXISTS forum_sessions (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES forum_users(id) ON DELETE CASCADE,
    created_at TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    expires_at TEXT NOT NULL,
    ip_address TEXT,
    user_agent TEXT
);

-- Categories table
CREATE TABLE IF NOT EXISTS forum_categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    description TEXT,
    icon TEXT DEFAULT 'folder',
    color TEXT DEFAULT '#6366f1',
    sort_order INTEGER DEFAULT 0,
    parent_id INTEGER REFERENCES forum_categories(id),
    is_locked INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Topics table
CREATE TABLE IF NOT EXISTS forum_topics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    category_id INTEGER NOT NULL REFERENCES forum_categories(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES forum_users(id),
    title TEXT NOT NULL,
    slug TEXT NOT NULL,
    content TEXT NOT NULL,
    content_html TEXT,
    view_count INTEGER DEFAULT 0,
    reply_count INTEGER DEFAULT 0,
    last_reply_at TEXT,
    last_reply_by INTEGER REFERENCES forum_users(id),
    is_pinned INTEGER DEFAULT 0,
    is_locked INTEGER DEFAULT 0,
    is_solved INTEGER DEFAULT 0,
    solved_post_id INTEGER REFERENCES forum_posts(id),
    created_at TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    deleted_at TEXT
);

-- Posts (replies) table
CREATE TABLE IF NOT EXISTS forum_posts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    topic_id INTEGER NOT NULL REFERENCES forum_topics(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES forum_users(id),
    parent_id INTEGER REFERENCES forum_posts(id),
    content TEXT NOT NULL,
    content_html TEXT,
    is_solution INTEGER DEFAULT 0,
    edit_count INTEGER DEFAULT 0,
    edited_at TEXT,
    edited_by INTEGER REFERENCES forum_users(id),
    created_at TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    deleted_at TEXT
);

-- Likes/reactions table
CREATE TABLE IF NOT EXISTS forum_reactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES forum_users(id) ON DELETE CASCADE,
    post_id INTEGER REFERENCES forum_posts(id) ON DELETE CASCADE,
    topic_id INTEGER REFERENCES forum_topics(id) ON DELETE CASCADE,
    reaction_type TEXT DEFAULT 'like' CHECK(reaction_type IN ('like', 'helpful', 'insightful', 'funny')),
    created_at TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE(user_id, post_id, reaction_type),
    UNIQUE(user_id, topic_id, reaction_type)
);

-- Notifications table
CREATE TABLE IF NOT EXISTS forum_notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES forum_users(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    message TEXT,
    link TEXT,
    is_read INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Private messages table
CREATE TABLE IF NOT EXISTS forum_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    from_user_id INTEGER NOT NULL REFERENCES forum_users(id),
    to_user_id INTEGER NOT NULL REFERENCES forum_users(id),
    subject TEXT NOT NULL,
    content TEXT NOT NULL,
    is_read INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    deleted_by_sender INTEGER DEFAULT 0,
    deleted_by_recipient INTEGER DEFAULT 0
);

-- Bookmarks table
CREATE TABLE IF NOT EXISTS forum_bookmarks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES forum_users(id) ON DELETE CASCADE,
    topic_id INTEGER REFERENCES forum_topics(id) ON DELETE CASCADE,
    post_id INTEGER REFERENCES forum_posts(id) ON DELETE CASCADE,
    created_at TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE(user_id, topic_id),
    UNIQUE(user_id, post_id)
);

-- Tags table
CREATE TABLE IF NOT EXISTS forum_tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    color TEXT DEFAULT '#gray'
);

-- Topic tags junction
CREATE TABLE IF NOT EXISTS forum_topic_tags (
    topic_id INTEGER NOT NULL REFERENCES forum_topics(id) ON DELETE CASCADE,
    tag_id INTEGER NOT NULL REFERENCES forum_tags(id) ON DELETE CASCADE,
    PRIMARY KEY (topic_id, tag_id)
);

-- Moderation log
CREATE TABLE IF NOT EXISTS forum_mod_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    moderator_id INTEGER NOT NULL REFERENCES forum_users(id),
    action TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id INTEGER NOT NULL,
    reason TEXT,
    details_json TEXT,
    created_at TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_topics_category ON forum_topics(category_id);
CREATE INDEX IF NOT EXISTS idx_topics_user ON forum_topics(user_id);
CREATE INDEX IF NOT EXISTS idx_topics_created ON forum_topics(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_posts_topic ON forum_posts(topic_id);
CREATE INDEX IF NOT EXISTS idx_posts_user ON forum_posts(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON forum_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON forum_sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_notifications_user ON forum_notifications(user_id, is_read);

-- Full-text search
CREATE VIRTUAL TABLE IF NOT EXISTS forum_search USING fts5(
    title,
    content,
    content='',
    tokenize='unicode61'
);

-- Insert default categories
INSERT OR IGNORE INTO forum_categories (id, name, slug, description, icon, sort_order) VALUES
    (1, 'Annonces', 'annonces', 'Annonces officielles et nouvelles importantes', 'megaphone', 1),
    (2, 'Discussions', 'discussions', 'Discussions generales', 'chat-bubble-left-right', 2),
    (3, 'Questions', 'questions', 'Posez vos questions ici', 'question-mark-circle', 3),
    (4, 'Tutoriels', 'tutoriels', 'Guides et tutoriels de la communaute', 'academic-cap', 4),
    (5, 'Projets', 'projets', 'Partagez vos projets', 'rocket-launch', 5);

-- Insert admin user (password: admin123 - change in production!)
INSERT OR IGNORE INTO forum_users (id, username, email, password_hash, display_name, role) VALUES
    (1, 'admin', 'admin@example.com', '$2a$10$rQEY0tHxLdGYFtAH4hJxheYs0XlXqR3SXplVh8gQJzKv8V8rlKYPi', 'Administrateur', 'admin');
