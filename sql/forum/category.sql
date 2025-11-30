-- Category View
-- @query component=shell title="Categorie"

-- Category header
-- @query component=text
SELECT '<div class="category-header">
    <nav class="breadcrumb">
        <a href="/forum">Forum</a> &raquo; ' || c.name || '
    </nav>
    <h1>' || c.name || '</h1>
    <p class="text-muted">' || COALESCE(c.description, '') || '</p>
</div>' as html
FROM forum_categories c
WHERE c.slug = $slug;

-- New topic button (if logged in)
-- @query component=text
SELECT CASE
    WHEN s.user_id IS NOT NULL AND cat.is_locked = 0 THEN
        '<div class="actions">
            <a href="/forum/new-topic?category=' || $slug || '" class="btn btn-primary">Nouveau sujet</a>
        </div>'
    WHEN cat.is_locked = 1 THEN
        '<div class="alert alert-info">Cette categorie est verrouillee</div>'
    ELSE
        '<div class="alert alert-info">
            <a href="/forum/login">Connectez-vous</a> pour creer un sujet
        </div>'
END as html
FROM forum_categories cat
LEFT JOIN forum_sessions s ON s.id = $session_id AND s.expires_at > datetime('now')
WHERE cat.slug = $slug;

-- Pinned topics
-- @query component=table title="Sujets epingles"
SELECT
    '<span class="badge badge-pinned">Epingle</span> <a href="/forum/topic?id=' || t.id || '">' || t.title || '</a>' as "Sujet",
    '<a href="/forum/user?id=' || u.id || '">' || u.display_name || '</a>' as "Auteur",
    t.reply_count as "Reponses",
    t.view_count as "Vues",
    time_ago(COALESCE(t.last_reply_at, t.created_at)) as "Dernier msg"
FROM forum_topics t
JOIN forum_categories c ON c.id = t.category_id
JOIN forum_users u ON u.id = t.user_id
WHERE c.slug = $slug AND t.is_pinned = 1 AND t.deleted_at IS NULL
ORDER BY t.created_at DESC;

-- Regular topics with pagination
-- @query component=table title="Sujets"
SELECT
    CASE WHEN t.is_locked THEN '<span class="badge badge-locked">Verrouille</span> ' ELSE '' END ||
    CASE WHEN t.is_solved THEN '<span class="badge badge-solved">Resolu</span> ' ELSE '' END ||
    '<a href="/forum/topic?id=' || t.id || '">' || t.title || '</a>' as "Sujet",
    '<a href="/forum/user?id=' || u.id || '">' || u.display_name || '</a>' as "Auteur",
    t.reply_count as "Reponses",
    t.view_count as "Vues",
    time_ago(COALESCE(t.last_reply_at, t.created_at)) as "Dernier msg"
FROM forum_topics t
JOIN forum_categories c ON c.id = t.category_id
JOIN forum_users u ON u.id = t.user_id
WHERE c.slug = $slug AND t.is_pinned = 0 AND t.deleted_at IS NULL
ORDER BY COALESCE(t.last_reply_at, t.created_at) DESC
LIMIT 20 OFFSET COALESCE($page, 0) * 20;

-- Pagination
-- @query component=text
SELECT '<div class="pagination">
    ' || CASE WHEN COALESCE($page, 0) > 0 THEN
        '<a href="/forum/category?slug=' || $slug || '&page=' || (COALESCE($page, 0) - 1) || '" class="btn">&laquo; Precedent</a>'
    ELSE '' END || '
    <span>Page ' || (COALESCE($page, 0) + 1) || '</span>
    ' || CASE WHEN (SELECT COUNT(*) FROM forum_topics t JOIN forum_categories c ON c.id = t.category_id WHERE c.slug = $slug AND t.deleted_at IS NULL) > (COALESCE($page, 0) + 1) * 20 THEN
        '<a href="/forum/category?slug=' || $slug || '&page=' || (COALESCE($page, 0) + 1) || '" class="btn">Suivant &raquo;</a>'
    ELSE '' END || '
</div>' as html;
