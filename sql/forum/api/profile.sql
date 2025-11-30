-- Profile Update API Handler
-- Handles POST /forum/api/profile

-- Update profile
-- @query component=text
SELECT CASE
    WHEN s.user_id IS NULL THEN
        '<div class="alert alert-error">Session expiree</div>
         <script>setTimeout(() => window.location.href = "/forum/login", 2000);</script>'
    WHEN length(TRIM($display_name)) < 2 THEN
        '<div class="alert alert-error">Le nom doit faire au moins 2 caracteres</div>
         <script>setTimeout(() => window.history.back(), 2000);</script>'
    WHEN $email NOT LIKE '%@%.%' THEN
        '<div class="alert alert-error">Email invalide</div>
         <script>setTimeout(() => window.history.back(), 2000);</script>'
    WHEN EXISTS(SELECT 1 FROM forum_users WHERE email = $email AND id != s.user_id) THEN
        '<div class="alert alert-error">Cet email est deja utilise</div>
         <script>setTimeout(() => window.history.back(), 2000);</script>'
    ELSE
        '<div class="alert alert-success">Profil mis a jour !</div>
         <script>setTimeout(() => window.location.href = "/forum/profile", 1500);</script>'
END as html
FROM (SELECT 1) dummy
LEFT JOIN forum_sessions s ON s.id = $session_id AND s.expires_at > datetime('now');

-- Update user
UPDATE forum_users
SET
    display_name = TRIM($display_name),
    email = $email,
    avatar_url = NULLIF(TRIM($avatar_url), ''),
    bio = NULLIF(TRIM($bio), '')
WHERE id = (SELECT user_id FROM forum_sessions WHERE id = $session_id AND expires_at > datetime('now'))
    AND length(TRIM($display_name)) >= 2
    AND $email LIKE '%@%.%';
