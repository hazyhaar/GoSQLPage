-- Demo: Table Component
-- Shows how to display data in a table

-- @query component=shell title="Table Demo"

-- @query component=text
SELECT 'This page demonstrates the table component.' as content;

-- @query component=table title="Sample Users"
SELECT
    1 as id,
    'Alice' as name,
    'alice@example.com' as email,
    'Admin' as role
UNION ALL
SELECT 2, 'Bob', 'bob@example.com', 'User'
UNION ALL
SELECT 3, 'Charlie', 'charlie@example.com', 'User'
UNION ALL
SELECT 4, 'Diana', 'diana@example.com', 'Moderator'
UNION ALL
SELECT 5, 'Eve', 'eve@example.com', 'User';

-- @query component=text
SELECT '<p><a href="/">Back to Home</a></p>' as html;
