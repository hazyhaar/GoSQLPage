-- Demo: List Component
-- Shows how to display items in a list

-- @query component=shell title="List Demo"

-- @query component=text
SELECT 'This page demonstrates the list component.' as content;

-- @query component=list title="Navigation Links"
SELECT
    'Home' as title,
    '/' as link,
    'Return to the homepage' as description
UNION ALL
SELECT 'Table Demo', '/demo/table', 'See the table component'
UNION ALL
SELECT 'Form Demo', '/demo/form', 'Try the form component'
UNION ALL
SELECT 'GitHub', 'https://github.com/hazyhaar/gopage', 'View source code';

-- @query component=list title="Features"
SELECT 'SQL-First Development' as title, 'Build applications using SQL queries' as description
UNION ALL
SELECT 'HTMX Integration', 'Dynamic UIs without writing JavaScript'
UNION ALL
SELECT 'Alpine.js', 'Lightweight client-side interactivity'
UNION ALL
SELECT 'Component System', 'Reusable UI components';

-- @query component=text
SELECT '<p><a href="/">Back to Home</a></p>' as html;
