-- Posts List Page
-- Displays all posts with author info

-- @query component=shell title="Posts"

-- @query component=text
SELECT '<p>Browse all posts. <a href="/posts/new">Create new post</a></p>' as html;

-- @query component=table title="Recent Posts"
SELECT
    p.id,
    p.title,
    u.name as author,
    substr(p.content, 1, 50) || '...' as preview,
    datetime(p.created_at) as created_at
FROM posts p
JOIN users u ON p.user_id = u.id
ORDER BY p.created_at DESC
LIMIT COALESCE(NULLIF($limit, ''), 10)
OFFSET (COALESCE(NULLIF($page, ''), 1) - 1) * COALESCE(NULLIF($limit, ''), 10);
