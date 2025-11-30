-- Reply API Handler
-- Handles POST /forum/api/reply

-- Check auth and create reply
-- @query component=text
SELECT CASE
    WHEN s.user_id IS NULL THEN
        '<div class="alert alert-error">Vous devez etre connecte</div>
         <script>setTimeout(() => window.location.href = "/forum/login", 2000);</script>'
    WHEN length(TRIM($content)) < 5 THEN
        '<div class="alert alert-error">La reponse doit faire au moins 5 caracteres</div>
         <script>setTimeout(() => window.history.back(), 2000);</script>'
    WHEN NOT EXISTS(SELECT 1 FROM forum_topics WHERE id = $topic_id AND is_locked = 0 AND deleted_at IS NULL) THEN
        '<div class="alert alert-error">Sujet invalide ou verrouille</div>
         <script>setTimeout(() => window.history.back(), 2000);</script>'
    ELSE
        '<div class="alert alert-success">Reponse publiee !</div>
         <script>setTimeout(() => window.location.href = "/forum/topic?id=' || $topic_id || '#post-' ||
            (SELECT id FROM forum_posts WHERE topic_id = $topic_id ORDER BY created_at DESC LIMIT 1) ||
         '", 1000);</script>'
END as html
FROM (SELECT 1) dummy
LEFT JOIN forum_sessions s ON s.id = $session_id AND s.expires_at > datetime('now');

-- Insert reply
INSERT INTO forum_posts (topic_id, user_id, parent_id, content)
SELECT
    $topic_id,
    s.user_id,
    NULLIF($quote, ''),
    TRIM($content)
FROM forum_sessions s
WHERE s.id = $session_id
    AND s.expires_at > datetime('now')
    AND length(TRIM($content)) >= 5
    AND EXISTS(SELECT 1 FROM forum_topics WHERE id = $topic_id AND is_locked = 0 AND deleted_at IS NULL);

-- Update topic stats
UPDATE forum_topics
SET
    reply_count = reply_count + 1,
    last_reply_at = datetime('now'),
    last_reply_by = (SELECT user_id FROM forum_sessions WHERE id = $session_id)
WHERE id = $topic_id
    AND EXISTS(SELECT 1 FROM forum_sessions WHERE id = $session_id AND expires_at > datetime('now'));

-- Update user post count
UPDATE forum_users
SET post_count = post_count + 1
WHERE id = (SELECT user_id FROM forum_sessions WHERE id = $session_id);

-- Create notification for topic author
INSERT INTO forum_notifications (user_id, type, title, message, link)
SELECT
    t.user_id,
    'reply',
    'Nouvelle reponse a votre sujet',
    u.display_name || ' a repondu a "' || t.title || '"',
    '/forum/topic?id=' || t.id
FROM forum_topics t
JOIN forum_sessions s ON s.id = $session_id
JOIN forum_users u ON u.id = s.user_id
WHERE t.id = $topic_id
    AND t.user_id != s.user_id;
