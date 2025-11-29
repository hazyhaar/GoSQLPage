-- Delete User Handler
-- Handles POST to delete a user

-- Delete the user
DELETE FROM users WHERE id = $id;

-- Also delete their posts
DELETE FROM posts WHERE user_id = $id;

-- Redirect back to users list
-- @query component=redirect target="/users"
SELECT '/users' as target;
