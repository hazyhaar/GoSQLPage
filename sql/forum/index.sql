-- Forum Homepage
-- @query component=shell title="Forum"

-- @query component=text
SELECT '<div class="forum-header">
    <h1>Forum</h1>
    <p class="text-muted">Bienvenue sur notre forum communautaire</p>
</div>' as html;

-- Get current user from session
-- @query component=text
SELECT CASE
    WHEN s.user_id IS NOT NULL THEN
        '<div class="user-bar">
            <span>Connecte en tant que <strong>' || u.display_name || '</strong></span>
            <a href="/forum/profile" class="btn btn-sm">Mon Profil</a>
            <a href="/forum/logout" class="btn btn-sm btn-outline">Deconnexion</a>
        </div>'
    ELSE
        '<div class="user-bar">
            <a href="/forum/login" class="btn btn-primary">Connexion</a>
            <a href="/forum/register" class="btn btn-outline">Inscription</a>
        </div>'
END as html
FROM (SELECT 1) dummy
LEFT JOIN forum_sessions s ON s.id = $session_id AND s.expires_at > datetime('now')
LEFT JOIN forum_users u ON u.id = s.user_id;

-- Categories list
-- @query component=table title="Categories"
SELECT
    '<a href="/forum/category?slug=' || c.slug || '">' || c.name || '</a>' as "Categorie",
    c.description as "Description",
    (SELECT COUNT(*) FROM forum_topics t WHERE t.category_id = c.id AND t.deleted_at IS NULL) as "Sujets",
    (SELECT COUNT(*) FROM forum_posts p
     JOIN forum_topics t ON p.topic_id = t.id
     WHERE t.category_id = c.id AND p.deleted_at IS NULL) as "Messages",
    COALESCE(
        (SELECT '<a href="/forum/topic?id=' || t.id || '">' || t.title || '</a><br><small>' ||
                time_ago(t.last_reply_at) || ' par ' || u.display_name || '</small>'
         FROM forum_topics t
         LEFT JOIN forum_users u ON u.id = t.last_reply_by
         WHERE t.category_id = c.id AND t.deleted_at IS NULL
         ORDER BY COALESCE(t.last_reply_at, t.created_at) DESC
         LIMIT 1),
        '<span class="text-muted">Aucun sujet</span>'
    ) as "Dernier message"
FROM forum_categories c
WHERE c.parent_id IS NULL
ORDER BY c.sort_order;

-- Recent topics
-- @query component=table title="Discussions recentes"
SELECT
    '<a href="/forum/topic?id=' || t.id || '">' || t.title || '</a>' as "Sujet",
    '<a href="/forum/category?slug=' || c.slug || '">' || c.name || '</a>' as "Categorie",
    '<a href="/forum/user?id=' || u.id || '">' || u.display_name || '</a>' as "Auteur",
    t.reply_count as "Reponses",
    t.view_count as "Vues",
    time_ago(COALESCE(t.last_reply_at, t.created_at)) as "Activite"
FROM forum_topics t
JOIN forum_categories c ON c.id = t.category_id
JOIN forum_users u ON u.id = t.user_id
WHERE t.deleted_at IS NULL
ORDER BY COALESCE(t.last_reply_at, t.created_at) DESC
LIMIT 10;

-- Forum stats
-- @query component=card title="Statistiques"
SELECT
    (SELECT COUNT(*) FROM forum_users) as "Membres",
    (SELECT COUNT(*) FROM forum_topics WHERE deleted_at IS NULL) as "Sujets",
    (SELECT COUNT(*) FROM forum_posts WHERE deleted_at IS NULL) as "Messages",
    (SELECT display_name FROM forum_users ORDER BY created_at DESC LIMIT 1) as "Dernier inscrit";
