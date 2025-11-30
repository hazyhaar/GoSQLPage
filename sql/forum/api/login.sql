-- Login API Handler
-- Handles POST /forum/api/login

-- Validate credentials and create session
-- @query component=text
SELECT CASE
    WHEN u.id IS NULL THEN
        '<div class="alert alert-error">Identifiants incorrects</div>
         <script>setTimeout(() => window.location.href = "/forum/login", 2000);</script>'
    WHEN u.role = 'banned' THEN
        '<div class="alert alert-error">Ce compte a ete suspendu</div>
         <script>setTimeout(() => window.location.href = "/forum/login", 2000);</script>'
    ELSE
        '<div class="alert alert-success">Connexion reussie ! Redirection...</div>
         <script>
            document.cookie = "session_id=" || new_session.id || "; path=/; max-age=" ||
                CASE WHEN $remember = 'on' THEN '2592000' ELSE '86400' END;
            window.location.href = "/forum";
         </script>'
END as html
FROM (SELECT 1) dummy
LEFT JOIN forum_users u ON (u.username = $username OR u.email = $username)
    AND u.password_hash = hash_password($password)
LEFT JOIN (
    INSERT INTO forum_sessions (id, user_id, expires_at, ip_address)
    SELECT
        hex(randomblob(16)),
        u.id,
        datetime('now', CASE WHEN $remember = 'on' THEN '+30 days' ELSE '+1 day' END),
        $remote_addr
    FROM forum_users u
    WHERE (u.username = $username OR u.email = $username)
        AND u.password_hash = hash_password($password)
        AND u.role != 'banned'
    RETURNING id
) new_session ON 1=1;

-- Update last seen
UPDATE forum_users
SET last_seen_at = datetime('now')
WHERE (username = $username OR email = $username)
    AND password_hash = hash_password($password);
