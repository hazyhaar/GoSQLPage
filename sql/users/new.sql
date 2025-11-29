-- New User Form
-- Creates a new user

-- @query component=shell title="New User"

-- @query component=text
SELECT '<p><a href="/users">&larr; Back to users</a></p>' as html;

-- @query component=form title="Create New User" action="/users/create" method="POST"
SELECT 'text' as type, 'name' as name, 'Full Name' as label, 'Enter full name' as placeholder, 1 as required
UNION ALL
SELECT 'email', 'email', 'Email Address', 'user@example.com', 1
UNION ALL
SELECT 'select', 'role', 'Role', '', 0
UNION ALL
SELECT 'submit', '', 'Create User', '', 0;
