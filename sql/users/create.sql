-- Create User Handler
-- Handles POST from new user form

-- Insert the new user
INSERT INTO users (name, email, role)
VALUES ($name, $email, COALESCE(NULLIF($role, ''), 'user'));

-- Redirect back to users list after successful creation
-- @query component=redirect target="/users"
SELECT '/users' as target;
