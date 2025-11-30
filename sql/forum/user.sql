-- User Profile (Public)
-- @query component=shell title="Profil"

-- User header
-- @query component=text
SELECT '<div class="profile-header">
    <img src="' || COALESCE(u.avatar_url, '/assets/default-avatar.png') || '" alt="" class="avatar-large">
    <div class="profile-info">
        <h1>' || u.display_name || '</h1>
        <div class="profile-meta">
            <span class="badge badge-' || u.role || '">' || u.role || '</span>
            &bull; Membre depuis ' || strftime('%d/%m/%Y', u.created_at) || '
            &bull; ' || u.post_count || ' messages
            &bull; ' || u.reputation || ' reputation
        </div>
        ' || CASE WHEN u.bio IS NOT NULL THEN '<p class="bio">' || u.bio || '</p>' ELSE '' END || '
        ' || CASE WHEN u.last_seen_at IS NOT NULL THEN
            '<div class="last-seen">Vu ' || time_ago(u.last_seen_at) || '</div>'
        ELSE '' END || '
    </div>
</div>' as html
FROM forum_users u
WHERE u.id = $id;

-- User's recent topics
-- @query component=table title="Derniers sujets"
SELECT
    '<a href="/forum/topic?id=' || t.id || '">' || t.title || '</a>' as "Sujet",
    '<a href="/forum/category?slug=' || c.slug || '">' || c.name || '</a>' as "Categorie",
    t.reply_count as "Reponses",
    time_ago(t.created_at) as "Date"
FROM forum_topics t
JOIN forum_categories c ON c.id = t.category_id
WHERE t.user_id = $id AND t.deleted_at IS NULL
ORDER BY t.created_at DESC
LIMIT 10;

-- User's recent posts
-- @query component=table title="Derniers messages"
SELECT
    '<a href="/forum/topic?id=' || t.id || '#post-' || p.id || '">' || t.title || '</a>' as "Sujet",
    SUBSTR(p.content, 1, 100) || '...' as "Extrait",
    time_ago(p.created_at) as "Date"
FROM forum_posts p
JOIN forum_topics t ON t.id = p.topic_id
WHERE p.user_id = $id AND p.deleted_at IS NULL
ORDER BY p.created_at DESC
LIMIT 10;

-- Actions (if viewing own profile or admin)
-- @query component=text
SELECT CASE
    WHEN s.user_id = $id THEN
        '<div class="profile-actions">
            <a href="/forum/profile" class="btn btn-primary">Modifier mon profil</a>
            <a href="/forum/settings" class="btn">Parametres</a>
        </div>'
    WHEN cu.role = 'admin' THEN
        '<div class="profile-actions admin-actions">
            <a href="/forum/admin/user?id=' || $id || '" class="btn btn-warning">Gerer cet utilisateur</a>
        </div>'
    ELSE ''
END as html
FROM (SELECT 1) dummy
LEFT JOIN forum_sessions s ON s.id = $session_id AND s.expires_at > datetime('now')
LEFT JOIN forum_users cu ON cu.id = s.user_id;
