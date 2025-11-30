-- Reaction API Handler
-- Handles POST /forum/api/react

-- Toggle reaction (like/unlike)
-- @query component=text
SELECT CASE
    WHEN s.user_id IS NULL THEN
        '<button class="btn btn-sm disabled">Connectez-vous</button>'
    ELSE
        '<button class="btn btn-sm' ||
        CASE WHEN EXISTS(
            SELECT 1 FROM forum_reactions r
            WHERE r.user_id = s.user_id
                AND (r.post_id = $post_id OR r.topic_id = $topic_id)
                AND r.reaction_type = COALESCE($type, 'like')
        ) THEN ' active' ELSE '' END ||
        '" hx-post="/forum/api/react?post_id=' || COALESCE($post_id, '') || '&topic_id=' || COALESCE($topic_id, '') || '&type=' || COALESCE($type, 'like') || '" hx-swap="outerHTML">
            <span class="icon">+</span> ' ||
            (SELECT COUNT(*) FROM forum_reactions r
             WHERE (r.post_id = $post_id OR r.topic_id = $topic_id)
               AND r.reaction_type = COALESCE($type, 'like')) ||
        '</button>'
END as html
FROM (SELECT 1) dummy
LEFT JOIN forum_sessions s ON s.id = $session_id AND s.expires_at > datetime('now');

-- Toggle reaction in database
INSERT INTO forum_reactions (user_id, post_id, topic_id, reaction_type)
SELECT
    s.user_id,
    NULLIF($post_id, ''),
    NULLIF($topic_id, ''),
    COALESCE($type, 'like')
FROM forum_sessions s
WHERE s.id = $session_id AND s.expires_at > datetime('now')
    AND NOT EXISTS(
        SELECT 1 FROM forum_reactions r
        WHERE r.user_id = s.user_id
            AND (($post_id IS NOT NULL AND r.post_id = $post_id) OR ($topic_id IS NOT NULL AND r.topic_id = $topic_id))
            AND r.reaction_type = COALESCE($type, 'like')
    )
ON CONFLICT DO NOTHING;

-- Remove if already exists (toggle behavior)
DELETE FROM forum_reactions
WHERE user_id = (SELECT user_id FROM forum_sessions WHERE id = $session_id AND expires_at > datetime('now'))
    AND (($post_id IS NOT NULL AND post_id = $post_id) OR ($topic_id IS NOT NULL AND topic_id = $topic_id))
    AND reaction_type = COALESCE($type, 'like')
    AND id NOT IN (
        SELECT MAX(id) FROM forum_reactions
        WHERE user_id = (SELECT user_id FROM forum_sessions WHERE id = $session_id)
            AND (($post_id IS NOT NULL AND post_id = $post_id) OR ($topic_id IS NOT NULL AND topic_id = $topic_id))
            AND reaction_type = COALESCE($type, 'like')
    );
