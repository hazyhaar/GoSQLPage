-- New Topic API Handler
-- Handles POST /forum/api/new-topic

-- Check auth and create topic
-- @query component=text
SELECT CASE
    WHEN s.user_id IS NULL THEN
        '<div class="alert alert-error">Vous devez etre connecte</div>
         <script>setTimeout(() => window.location.href = "/forum/login", 2000);</script>'
    WHEN length(TRIM($title)) < 5 THEN
        '<div class="alert alert-error">Le titre doit faire au moins 5 caracteres</div>
         <script>setTimeout(() => window.history.back(), 2000);</script>'
    WHEN length(TRIM($content)) < 20 THEN
        '<div class="alert alert-error">Le contenu doit faire au moins 20 caracteres</div>
         <script>setTimeout(() => window.history.back(), 2000);</script>'
    WHEN NOT EXISTS(SELECT 1 FROM forum_categories WHERE id = $category_id AND is_locked = 0) THEN
        '<div class="alert alert-error">Categorie invalide ou verrouillee</div>
         <script>setTimeout(() => window.history.back(), 2000);</script>'
    ELSE
        '<div class="alert alert-success">Sujet cree ! Redirection...</div>
         <script>setTimeout(() => window.location.href = "/forum/topic?id=' ||
            (SELECT id FROM forum_topics WHERE user_id = s.user_id ORDER BY created_at DESC LIMIT 1) ||
         '", 1500);</script>'
END as html
FROM (SELECT 1) dummy
LEFT JOIN forum_sessions s ON s.id = $session_id AND s.expires_at > datetime('now');

-- Insert topic
INSERT INTO forum_topics (category_id, user_id, title, slug, content)
SELECT
    $category_id,
    s.user_id,
    TRIM($title),
    lower(replace(replace(replace(TRIM($title), ' ', '-'), '''', ''), '"', '')),
    TRIM($content)
FROM forum_sessions s
WHERE s.id = $session_id
    AND s.expires_at > datetime('now')
    AND length(TRIM($title)) >= 5
    AND length(TRIM($content)) >= 20
    AND EXISTS(SELECT 1 FROM forum_categories WHERE id = $category_id AND is_locked = 0);

-- Update user post count
UPDATE forum_users
SET post_count = post_count + 1
WHERE id = (SELECT user_id FROM forum_sessions WHERE id = $session_id);
