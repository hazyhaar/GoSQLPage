-- Edit User Page
-- Shows form to edit an existing user

-- @query component=shell title="Edit User"

-- @query component=text
SELECT '<p><a href="/users">&larr; Back to users</a></p>' as html;

-- Get current user data
-- @query component=form title="Edit User" action="/users/update" method="POST"
SELECT 'hidden' as type, 'id' as name, '' as label, '' as placeholder, id as value
FROM users WHERE id = $id
UNION ALL
SELECT 'text', 'name', 'Full Name', 'Enter full name', name
FROM users WHERE id = $id
UNION ALL
SELECT 'email', 'email', 'Email Address', 'user@example.com', email
FROM users WHERE id = $id
UNION ALL
SELECT 'text', 'role', 'Role', 'admin, user, moderator', role
FROM users WHERE id = $id
UNION ALL
SELECT 'submit', '', 'Update User', '', '';

-- Delete button
-- @query component=text
SELECT '<form method="POST" action="/users/delete" style="margin-top: 2rem;"
    onsubmit="return confirm(''Are you sure you want to delete this user?'')">
    <input type="hidden" name="id" value="' || $id || '">
    <button type="submit" class="secondary outline">Delete User</button>
</form>' as html;
