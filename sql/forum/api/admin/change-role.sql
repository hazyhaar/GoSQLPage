-- Admin Change Role API
-- Handles POST /forum/api/admin/change-role

-- Check admin access and update role
-- @query component=text
SELECT CASE
    WHEN cu.role != 'admin' THEN
        '<div class="alert alert-error">Acces refuse</div>
         <script>setTimeout(() => window.history.back(), 2000);</script>'
    WHEN $user_id = s.user_id THEN
        '<div class="alert alert-error">Vous ne pouvez pas modifier votre propre role</div>
         <script>setTimeout(() => window.history.back(), 2000);</script>'
    WHEN $role NOT IN ('member', 'moderator', 'admin', 'banned') THEN
        '<div class="alert alert-error">Role invalide</div>
         <script>setTimeout(() => window.history.back(), 2000);</script>'
    ELSE
        '<div class="alert alert-success">Role mis a jour</div>
         <script>setTimeout(() => window.location.href = "/forum/admin/user?id=' || $user_id || '", 1500);</script>'
END as html
FROM forum_sessions s
JOIN forum_users cu ON cu.id = s.user_id
WHERE s.id = $session_id AND s.expires_at > datetime('now');

-- Update role
UPDATE forum_users
SET role = $role
WHERE id = $user_id
    AND EXISTS(
        SELECT 1 FROM forum_sessions s
        JOIN forum_users cu ON cu.id = s.user_id
        WHERE s.id = $session_id AND s.expires_at > datetime('now') AND cu.role = 'admin'
    )
    AND $user_id != (SELECT user_id FROM forum_sessions WHERE id = $session_id);

-- Log action
INSERT INTO forum_mod_log (moderator_id, action, target_type, target_id, reason)
SELECT
    s.user_id,
    'change_role_to_' || $role,
    'user',
    $user_id,
    NULLIF($reason, '')
FROM forum_sessions s
JOIN forum_users cu ON cu.id = s.user_id
WHERE s.id = $session_id AND s.expires_at > datetime('now') AND cu.role = 'admin';
