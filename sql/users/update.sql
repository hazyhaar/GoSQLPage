-- Update User Handler
-- Handles POST to update a user

-- Update the user
UPDATE users
SET name = $name, email = $email, role = $role
WHERE id = $id;

-- Redirect back to users list
-- @query component=redirect target="/users"
SELECT '/users' as target;
