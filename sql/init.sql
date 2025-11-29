-- GoPage Database Initialization
-- Run this to set up example tables

-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    email TEXT UNIQUE NOT NULL,
    role TEXT DEFAULT 'user',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Create posts table
CREATE TABLE IF NOT EXISTS posts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    content TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Insert sample data
INSERT OR IGNORE INTO users (id, name, email, role) VALUES
    (1, 'Alice', 'alice@example.com', 'admin'),
    (2, 'Bob', 'bob@example.com', 'user'),
    (3, 'Charlie', 'charlie@example.com', 'user');

INSERT OR IGNORE INTO posts (user_id, title, content) VALUES
    (1, 'Welcome to GoPage', 'This is the first post on our new platform!'),
    (2, 'Getting Started', 'Here is how to build your first SQL page...'),
    (1, 'Advanced Features', 'Let me show you some cool tricks.');
