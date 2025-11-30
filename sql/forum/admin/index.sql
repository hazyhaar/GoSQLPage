-- Admin Dashboard
-- @query component=shell title="Administration"

-- Check admin access
-- @query component=text
SELECT CASE
    WHEN s.user_id IS NULL THEN
        '<script>window.location.href = "/forum/login";</script>'
    WHEN u.role NOT IN ('admin', 'moderator') THEN
        '<div class="alert alert-error">Acces refuse</div>
         <script>setTimeout(() => window.location.href = "/forum", 2000);</script>'
    ELSE
        '<h1>Administration du forum</h1>
         <nav class="admin-nav">
             <a href="/forum/admin" class="active">Dashboard</a>
             <a href="/forum/admin/users">Utilisateurs</a>
             <a href="/forum/admin/categories">Categories</a>
             <a href="/forum/admin/reports">Signalements</a>
             <a href="/forum/admin/logs">Logs</a>
         </nav>'
END as html
FROM (SELECT 1) dummy
LEFT JOIN forum_sessions s ON s.id = $session_id AND s.expires_at > datetime('now')
LEFT JOIN forum_users u ON u.id = s.user_id;

-- Stats cards
-- @query component=card title="Statistiques"
SELECT
    (SELECT COUNT(*) FROM forum_users) as "Membres total",
    (SELECT COUNT(*) FROM forum_users WHERE created_at > datetime('now', '-7 days')) as "Nouveaux (7j)",
    (SELECT COUNT(*) FROM forum_topics WHERE deleted_at IS NULL) as "Sujets",
    (SELECT COUNT(*) FROM forum_posts WHERE deleted_at IS NULL) as "Messages",
    (SELECT COUNT(*) FROM forum_users WHERE last_seen_at > datetime('now', '-1 hour')) as "En ligne";

-- Recent registrations
-- @query component=table title="Derniers inscrits"
SELECT
    '<a href="/forum/user?id=' || u.id || '">' || u.display_name || '</a>' as "Membre",
    u.username as "Username",
    u.email as "Email",
    u.role as "Role",
    time_ago(u.created_at) as "Inscription"
FROM forum_users u
ORDER BY u.created_at DESC
LIMIT 10;

-- Recent activity
-- @query component=table title="Activite recente"
SELECT
    m.action as "Action",
    '<a href="/forum/user?id=' || mod.id || '">' || mod.display_name || '</a>' as "Moderateur",
    m.target_type || ' #' || m.target_id as "Cible",
    m.reason as "Raison",
    time_ago(m.created_at) as "Date"
FROM forum_mod_log m
JOIN forum_users mod ON mod.id = m.moderator_id
ORDER BY m.created_at DESC
LIMIT 20;
