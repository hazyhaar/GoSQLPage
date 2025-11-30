-- New Topic Page
-- @query component=shell title="Nouveau sujet"

-- Check if logged in
-- @query component=text
SELECT CASE
    WHEN s.user_id IS NULL THEN
        '<div class="alert alert-error">Vous devez etre connecte pour creer un sujet.</div>
         <script>setTimeout(() => window.location.href = "/forum/login", 2000);</script>'
    ELSE ''
END as html
FROM (SELECT 1) dummy
LEFT JOIN forum_sessions s ON s.id = $session_id AND s.expires_at > datetime('now');

-- Breadcrumb
-- @query component=text
SELECT '<nav class="breadcrumb">
    <a href="/forum">Forum</a> &raquo;
    ' || CASE WHEN $category IS NOT NULL THEN
        '<a href="/forum/category?slug=' || $category || '">' ||
        (SELECT name FROM forum_categories WHERE slug = $category) || '</a> &raquo;'
    ELSE '' END || '
    Nouveau sujet
</nav>
<h1>Creer un nouveau sujet</h1>' as html;

-- New topic form
-- @query component=form action="/forum/api/new-topic" method="POST"
SELECT
    'hidden' as type,
    'category_slug' as name,
    '' as label,
    $category as value,
    0 as required
UNION ALL SELECT
    'select' as type,
    'category_id' as name,
    'Categorie' as label,
    (SELECT GROUP_CONCAT(id || ':' || name, '|') FROM forum_categories WHERE is_locked = 0 ORDER BY sort_order) as options,
    1 as required
UNION ALL SELECT
    'text' as type,
    'title' as name,
    'Titre du sujet' as label,
    'Entrez un titre descriptif' as placeholder,
    1 as required
UNION ALL SELECT
    'textarea' as type,
    'content' as name,
    'Contenu' as label,
    'Decrivez votre sujet en detail...' as placeholder,
    1 as required
UNION ALL SELECT
    'text' as type,
    'tags' as name,
    'Tags (separes par des virgules)' as label,
    'ex: question, aide, tutoriel' as placeholder,
    0 as required
UNION ALL SELECT
    'submit' as type,
    'submit' as name,
    'Publier le sujet' as label,
    '' as placeholder,
    0 as required;
