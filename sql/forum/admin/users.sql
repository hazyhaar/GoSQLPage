-- Admin Users Management
-- @query component=shell title="Gestion des utilisateurs"

-- Check admin access
-- @query component=text
SELECT CASE
    WHEN u.role NOT IN ('admin', 'moderator') THEN
        '<script>window.location.href = "/forum";</script>'
    ELSE
        '<h1>Gestion des utilisateurs</h1>
         <nav class="admin-nav">
             <a href="/forum/admin">Dashboard</a>
             <a href="/forum/admin/users" class="active">Utilisateurs</a>
             <a href="/forum/admin/categories">Categories</a>
             <a href="/forum/admin/reports">Signalements</a>
             <a href="/forum/admin/logs">Logs</a>
         </nav>'
END as html
FROM forum_sessions s
JOIN forum_users u ON u.id = s.user_id
WHERE s.id = $session_id AND s.expires_at > datetime('now');

-- Search form
-- @query component=search action="/forum/admin/users" placeholder="Rechercher un utilisateur..."

-- Users list
-- @query component=table title="Utilisateurs"
SELECT
    u.id as "ID",
    '<a href="/forum/user?id=' || u.id || '">' || u.display_name || '</a>' as "Nom",
    u.username as "Username",
    u.email as "Email",
    '<span class="badge badge-' || u.role || '">' || u.role || '</span>' as "Role",
    u.post_count as "Messages",
    COALESCE(time_ago(u.last_seen_at), 'Jamais') as "Vu",
    time_ago(u.created_at) as "Inscrit",
    '<a href="/forum/admin/user?id=' || u.id || '" class="btn btn-sm">Gerer</a>' as ""
FROM forum_users u
WHERE ($q IS NULL OR $q = '' OR u.username LIKE '%' || $q || '%' OR u.display_name LIKE '%' || $q || '%' OR u.email LIKE '%' || $q || '%')
ORDER BY
    CASE $sort
        WHEN 'name' THEN u.display_name
        WHEN 'posts' THEN CAST(u.post_count AS TEXT)
        WHEN 'recent' THEN u.last_seen_at
        ELSE u.created_at
    END DESC
LIMIT 50 OFFSET COALESCE($page, 0) * 50;

-- Pagination
-- @query component=text
SELECT '<div class="pagination">
    ' || CASE WHEN COALESCE($page, 0) > 0 THEN
        '<a href="/forum/admin/users?page=' || (COALESCE($page, 0) - 1) || COALESCE('&q=' || $q, '') || '" class="btn">&laquo; Precedent</a>'
    ELSE '' END || '
    <span>Page ' || (COALESCE($page, 0) + 1) || '</span>
    ' || CASE WHEN (SELECT COUNT(*) FROM forum_users) > (COALESCE($page, 0) + 1) * 50 THEN
        '<a href="/forum/admin/users?page=' || (COALESCE($page, 0) + 1) || COALESCE('&q=' || $q, '') || '" class="btn">Suivant &raquo;</a>'
    ELSE '' END || '
</div>' as html;
