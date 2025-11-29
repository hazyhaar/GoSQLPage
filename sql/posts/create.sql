-- Create Post Handler
-- Handles POST from new post form

-- Insert the new post
INSERT INTO posts (user_id, title, content)
VALUES (
    COALESCE(NULLIF($user_id, ''), 1),
    $title,
    $content
);

-- Redirect back to posts list after successful creation
-- @query component=redirect target="/posts"
SELECT '/posts' as target;
