-- My Profile (Edit)
-- @query component=shell title="Mon profil"

-- Check if logged in
-- @query component=text
SELECT CASE
    WHEN s.user_id IS NULL THEN
        '<script>window.location.href = "/forum/login";</script>'
    ELSE
        '<h1>Mon profil</h1>'
END as html
FROM (SELECT 1) dummy
LEFT JOIN forum_sessions s ON s.id = $session_id AND s.expires_at > datetime('now');

-- Profile form
-- @query component=form action="/forum/api/profile" method="POST"
SELECT
    'text' as type,
    'display_name' as name,
    'Nom affiche' as label,
    u.display_name as value,
    1 as required
FROM forum_users u
JOIN forum_sessions s ON s.user_id = u.id
WHERE s.id = $session_id
UNION ALL SELECT
    'email' as type,
    'email' as name,
    'Email' as label,
    u.email as value,
    1 as required
FROM forum_users u
JOIN forum_sessions s ON s.user_id = u.id
WHERE s.id = $session_id
UNION ALL SELECT
    'url' as type,
    'avatar_url' as name,
    'URL de l avatar' as label,
    COALESCE(u.avatar_url, '') as value,
    0 as required
FROM forum_users u
JOIN forum_sessions s ON s.user_id = u.id
WHERE s.id = $session_id
UNION ALL SELECT
    'textarea' as type,
    'bio' as name,
    'Biographie' as label,
    COALESCE(u.bio, '') as value,
    0 as required
FROM forum_users u
JOIN forum_sessions s ON s.user_id = u.id
WHERE s.id = $session_id
UNION ALL SELECT
    'submit' as type,
    'submit' as name,
    'Enregistrer' as label,
    '' as value,
    0 as required;

-- Change password section
-- @query component=text
SELECT '<hr><h2>Changer le mot de passe</h2>' as html;

-- @query component=form action="/forum/api/change-password" method="POST"
SELECT
    'password' as type,
    'current_password' as name,
    'Mot de passe actuel' as label,
    '' as placeholder,
    1 as required
UNION ALL SELECT
    'password' as type,
    'new_password' as name,
    'Nouveau mot de passe' as label,
    'Minimum 8 caracteres' as placeholder,
    1 as required
UNION ALL SELECT
    'password' as type,
    'confirm_password' as name,
    'Confirmer le nouveau mot de passe' as label,
    '' as placeholder,
    1 as required
UNION ALL SELECT
    'submit' as type,
    'submit' as name,
    'Changer le mot de passe' as label,
    '' as placeholder,
    0 as required;
