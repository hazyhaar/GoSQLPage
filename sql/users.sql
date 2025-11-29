-- Users List Page
-- Displays all users with search and pagination

-- @query component=shell title="Users"

-- @query component=text
SELECT '<p>Manage your application users. <a href="/users/new" role="button" class="outline">+ Add New User</a></p>' as html;

-- @query component=search placeholder="Search users by name or email..." action="/users" target="#users-table-container"

-- @query component=table title="All Users" id="users-table"
SELECT
    id,
    name,
    email,
    role,
    datetime(created_at) as created_at
FROM users
WHERE
    ($q IS NULL OR $q = '' OR name LIKE '%' || $q || '%' OR email LIKE '%' || $q || '%')
ORDER BY id DESC
LIMIT COALESCE(NULLIF($limit, ''), 10)
OFFSET (COALESCE(NULLIF($page, ''), 1) - 1) * COALESCE(NULLIF($limit, ''), 10);
