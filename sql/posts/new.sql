-- New Post Form
-- Creates a new post

-- @query component=shell title="New Post"

-- @query component=text
SELECT '<p><a href="/posts">&larr; Back to posts</a></p>' as html;

-- @query component=form title="Create New Post" action="/posts/create" method="POST"
SELECT 'text' as type, 'title' as name, 'Title' as label, 'Enter post title' as placeholder, 1 as required
UNION ALL
SELECT 'textarea', 'content', 'Content', 'Write your post content here...', 1
UNION ALL
SELECT 'hidden', 'user_id', '', '1', 0
UNION ALL
SELECT 'submit', '', 'Publish Post', '', 0;
