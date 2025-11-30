-- Search Page
-- @query component=shell title="Recherche"

-- Search form
-- @query component=search action="/forum/search" placeholder="Rechercher dans le forum..."

-- Show query if provided
-- @query component=text
SELECT CASE
    WHEN $q IS NOT NULL AND $q != '' THEN
        '<h2>Resultats pour "' || $q || '"</h2>'
    ELSE
        '<h2>Rechercher</h2>
         <p>Entrez un terme de recherche pour trouver des sujets et messages.</p>'
END as html;

-- Search in topics
-- @query component=table title="Sujets"
SELECT
    '<a href="/forum/topic?id=' || t.id || '">' || t.title || '</a>' as "Sujet",
    '<a href="/forum/category?slug=' || c.slug || '">' || c.name || '</a>' as "Categorie",
    '<a href="/forum/user?id=' || u.id || '">' || u.display_name || '</a>' as "Auteur",
    t.reply_count as "Reponses",
    time_ago(t.created_at) as "Date"
FROM forum_topics t
JOIN forum_categories c ON c.id = t.category_id
JOIN forum_users u ON u.id = t.user_id
WHERE t.deleted_at IS NULL
    AND $q IS NOT NULL AND $q != ''
    AND (t.title LIKE '%' || $q || '%' OR t.content LIKE '%' || $q || '%')
ORDER BY
    CASE WHEN t.title LIKE '%' || $q || '%' THEN 0 ELSE 1 END,
    t.created_at DESC
LIMIT 20;

-- Search in posts
-- @query component=table title="Messages"
SELECT
    '<a href="/forum/topic?id=' || t.id || '#post-' || p.id || '">' || t.title || '</a>' as "Sujet",
    SUBSTR(p.content, 1, 150) || '...' as "Extrait",
    '<a href="/forum/user?id=' || u.id || '">' || u.display_name || '</a>' as "Auteur",
    time_ago(p.created_at) as "Date"
FROM forum_posts p
JOIN forum_topics t ON t.id = p.topic_id
JOIN forum_users u ON u.id = p.user_id
WHERE p.deleted_at IS NULL AND t.deleted_at IS NULL
    AND $q IS NOT NULL AND $q != ''
    AND p.content LIKE '%' || $q || '%'
ORDER BY p.created_at DESC
LIMIT 20;

-- Search in users
-- @query component=table title="Membres"
SELECT
    '<a href="/forum/user?id=' || u.id || '">' || u.display_name || '</a>' as "Membre",
    u.role as "Role",
    u.post_count || ' messages' as "Activite",
    'Inscrit ' || time_ago(u.created_at) as "Inscription"
FROM forum_users u
WHERE $q IS NOT NULL AND $q != ''
    AND (u.username LIKE '%' || $q || '%' OR u.display_name LIKE '%' || $q || '%')
ORDER BY u.post_count DESC
LIMIT 10;
