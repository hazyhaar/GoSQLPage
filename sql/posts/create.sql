-- Create Post Handler
-- Handles POST from new post form

-- Insert the new post
INSERT INTO posts (user_id, title, content)
VALUES (
    COALESCE(NULLIF($user_id, ''), 1),
    $title,
    $content
);

-- @query component=shell title="Post Created"

-- @query component=text
SELECT '<div class="alert alert-success">Post published successfully!</div>' as html;

-- @query component=text
SELECT '<p><a href="/posts" role="button">Back to Posts List</a></p>' as html;
