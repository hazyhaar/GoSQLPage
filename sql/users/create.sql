-- Create User Handler
-- Handles POST from new user form

-- Insert the new user
INSERT INTO users (name, email, role)
VALUES ($name, $email, COALESCE(NULLIF($role, ''), 'user'));

-- @query component=shell title="User Created"

-- @query component=text
SELECT '<div class="alert alert-success">User created successfully!</div>' as html;

-- @query component=text
SELECT '<p><a href="/users" role="button">Back to Users List</a></p>' as html;
