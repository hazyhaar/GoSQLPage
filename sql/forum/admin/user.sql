-- Admin User Management
-- @query component=shell title="Gérer l'utilisateur"

-- Check admin access
-- @query component=text
SELECT CASE
    WHEN cu.role != 'admin' THEN
        '<script>window.location.href = "/forum";</script>'
    ELSE
        '<nav class="breadcrumb">
            <a href="/forum/admin">Admin</a> &raquo;
            <a href="/forum/admin/users">Utilisateurs</a> &raquo;
            ' || escape_html(u.display_name) || '
        </nav>
        <h1>Gérer : ' || escape_html(u.display_name) || '</h1>'
END as html
FROM forum_users u
JOIN forum_sessions s ON s.id = $session_id AND s.expires_at > datetime('now')
JOIN forum_users cu ON cu.id = s.user_id
WHERE u.id = $id;

-- User info card (only show to admins)
-- @query component=card title="Informations"
SELECT
    u.id as "ID",
    u.username as "Username",
    escape_html(u.display_name) as "Nom affiché",
    u.email as "Email",
    u.role as "Rôle actuel",
    u.post_count as "Messages",
    u.reputation as "Réputation",
    u.created_at as "Inscrit le",
    COALESCE(u.last_seen_at, 'Jamais') as "Dernière visite"
FROM forum_users u
JOIN forum_sessions s ON s.id = $session_id AND s.expires_at > datetime('now')
JOIN forum_users cu ON cu.id = s.user_id
WHERE u.id = $id AND cu.role = 'admin';

-- Role change form
-- @query component=form action="/forum/api/admin/change-role" method="POST" title="Changer le rôle"
SELECT
    'hidden' as type,
    'user_id' as name,
    '' as label,
    $id as value,
    0 as required
UNION ALL SELECT
    'select' as type,
    'role' as name,
    'Nouveau rôle' as label,
    'member:Membre|moderator:Modérateur|admin:Administrateur|banned:Banni' as options,
    1 as required
UNION ALL SELECT
    'textarea' as type,
    'reason' as name,
    'Raison du changement' as label,
    'Optionnel' as placeholder,
    0 as required
UNION ALL SELECT
    'submit' as type,
    'submit' as name,
    'Changer le rôle' as label,
    '' as placeholder,
    0 as required;

-- Quick actions
-- @query component=text
SELECT '<div class="admin-actions">
    <h3>Actions rapides</h3>
    <form action="/forum/api/admin/reset-password" method="POST" style="display:inline">
        <input type="hidden" name="user_id" value="' || $id || '">
        <button type="submit" class="btn btn-warning">Réinitialiser mot de passe</button>
    </form>
    <form action="/forum/api/admin/delete-user" method="POST" style="display:inline" onsubmit="return confirm(''Supprimer définitivement cet utilisateur ?'')">
        <input type="hidden" name="user_id" value="' || $id || '">
        <button type="submit" class="btn btn-danger">Supprimer le compte</button>
    </form>
</div>' as html;

-- Recent activity (only show to admins)
-- @query component=table title="Activité récente"
SELECT
    'Sujet' as "Type",
    '<a href="/forum/topic?id=' || t.id || '">' || escape_html(t.title) || '</a>' as "Contenu",
    time_ago(t.created_at) as "Date"
FROM forum_topics t
JOIN forum_sessions s ON s.id = $session_id AND s.expires_at > datetime('now')
JOIN forum_users cu ON cu.id = s.user_id
WHERE t.user_id = $id AND cu.role = 'admin'
UNION ALL
SELECT
    'Message' as "Type",
    '<a href="/forum/topic?id=' || p.topic_id || '#post-' || p.id || '">' || escape_html(SUBSTR(p.content, 1, 50)) || '...</a>' as "Contenu",
    time_ago(p.created_at) as "Date"
FROM forum_posts p
JOIN forum_sessions s ON s.id = $session_id AND s.expires_at > datetime('now')
JOIN forum_users cu ON cu.id = s.user_id
WHERE p.user_id = $id AND cu.role = 'admin'
ORDER BY 3 DESC
LIMIT 20;
