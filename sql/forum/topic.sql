-- Topic View
-- @query component=shell title="Sujet"

-- Increment view count
UPDATE forum_topics SET view_count = view_count + 1 WHERE id = $id;

-- Topic header with breadcrumb
-- @query component=text
SELECT '<div class="topic-header">
    <nav class="breadcrumb">
        <a href="/forum">Forum</a> &raquo;
        <a href="/forum/category?slug=' || c.slug || '">' || escape_html(c.name) || '</a> &raquo;
        ' || escape_html(t.title) || '
    </nav>
    <h1>' || escape_html(t.title) || '</h1>
    <div class="topic-meta">
        Par <a href="/forum/user?id=' || u.id || '">' || escape_html(u.display_name) || '</a>
        &bull; ' || time_ago(t.created_at) || '
        &bull; ' || t.view_count || ' vues
        &bull; ' || t.reply_count || ' reponses
        ' || CASE WHEN t.is_pinned THEN '&bull; <span class="badge badge-pinned">Epingle</span>' ELSE '' END || '
        ' || CASE WHEN t.is_locked THEN '&bull; <span class="badge badge-locked">Verrouille</span>' ELSE '' END || '
        ' || CASE WHEN t.is_solved THEN '&bull; <span class="badge badge-solved">Resolu</span>' ELSE '' END || '
    </div>
</div>' as html
FROM forum_topics t
JOIN forum_categories c ON c.id = t.category_id
JOIN forum_users u ON u.id = t.user_id
WHERE t.id = $id;

-- Original post
-- @query component=text
SELECT '<article class="post post-original" id="post-0">
    <aside class="post-author">
        <img src="' || COALESCE(u.avatar_url, '/assets/default-avatar.png') || '" alt="" class="avatar">
        <div class="author-name"><a href="/forum/user?id=' || u.id || '">' || escape_html(u.display_name) || '</a></div>
        <div class="author-role badge-' || u.role || '">' || u.role || '</div>
        <div class="author-stats">' || u.post_count || ' messages</div>
    </aside>
    <div class="post-content">
        <div class="post-body">' || COALESCE(t.content_html, escape_html(t.content)) || '</div>
        <footer class="post-footer">
            <span class="post-date">' || time_ago(t.created_at) || '</span>
            <div class="post-actions">
                ' || CASE WHEN s.user_id IS NOT NULL THEN
                    '<button class="btn btn-sm" hx-post="/forum/api/react?topic_id=' || t.id || '&type=like" hx-swap="outerHTML">
                        <span class="icon">+</span> ' ||
                        (SELECT COUNT(*) FROM forum_reactions r WHERE r.topic_id = t.id AND r.reaction_type = ''like'') ||
                    '</button>
                    <a href="/forum/reply?topic=' || t.id || '" class="btn btn-sm">Repondre</a>'
                ELSE '' END || '
                ' || CASE WHEN s.user_id = t.user_id OR cu.role IN (''admin'', ''moderator'') THEN
                    '<a href="/forum/edit-topic?id=' || t.id || '" class="btn btn-sm">Modifier</a>'
                ELSE '' END || '
            </div>
        </footer>
    </div>
</article>' as html
FROM forum_topics t
JOIN forum_users u ON u.id = t.user_id
LEFT JOIN forum_sessions s ON s.id = $session_id AND s.expires_at > datetime('now')
LEFT JOIN forum_users cu ON cu.id = s.user_id
WHERE t.id = $id;

-- Replies
-- @query component=text
SELECT '<article class="post' || CASE WHEN p.is_solution THEN ' post-solution' ELSE '' END || '" id="post-' || p.id || '">
    <aside class="post-author">
        <img src="' || COALESCE(u.avatar_url, '/assets/default-avatar.png') || '" alt="" class="avatar">
        <div class="author-name"><a href="/forum/user?id=' || u.id || '">' || escape_html(u.display_name) || '</a></div>
        <div class="author-role badge-' || u.role || '">' || u.role || '</div>
        <div class="author-stats">' || u.post_count || ' messages</div>
    </aside>
    <div class="post-content">
        ' || CASE WHEN p.is_solution THEN '<div class="solution-badge">Solution acceptee</div>' ELSE '' END || '
        ' || CASE WHEN p.parent_id IS NOT NULL THEN
            '<blockquote class="quote">En reponse a ' ||
            escape_html((SELECT display_name FROM forum_users WHERE id = (SELECT user_id FROM forum_posts WHERE id = p.parent_id))) ||
            '</blockquote>'
        ELSE '' END || '
        <div class="post-body">' || COALESCE(p.content_html, escape_html(p.content)) || '</div>
        <footer class="post-footer">
            <span class="post-date">' || time_ago(p.created_at) ||
            CASE WHEN p.edit_count > 0 THEN ' (modifie ' || p.edit_count || ' fois)' ELSE '' END || '</span>
            <div class="post-actions">
                ' || CASE WHEN s.user_id IS NOT NULL THEN
                    '<button class="btn btn-sm" hx-post="/forum/api/react?post_id=' || p.id || '&type=like" hx-swap="outerHTML">
                        <span class="icon">+</span> ' ||
                        (SELECT COUNT(*) FROM forum_reactions r WHERE r.post_id = p.id AND r.reaction_type = ''like'') ||
                    '</button>
                    <a href="/forum/reply?topic=' || p.topic_id || '&quote=' || p.id || '" class="btn btn-sm">Citer</a>'
                ELSE '' END || '
                ' || CASE WHEN s.user_id = p.user_id OR cu.role IN (''admin'', ''moderator'') THEN
                    '<a href="/forum/edit-post?id=' || p.id || '" class="btn btn-sm">Modifier</a>'
                ELSE '' END || '
                ' || CASE WHEN (cu.role IN (''admin'', ''moderator'') OR s.user_id = t.user_id) AND t.is_solved = 0 THEN
                    '<button class="btn btn-sm btn-success" hx-post="/forum/api/mark-solution?post_id=' || p.id || '">Marquer comme solution</button>'
                ELSE '' END || '
            </div>
        </footer>
    </div>
</article>' as html
FROM forum_posts p
JOIN forum_users u ON u.id = p.user_id
JOIN forum_topics t ON t.id = p.topic_id
LEFT JOIN forum_sessions s ON s.id = $session_id AND s.expires_at > datetime('now')
LEFT JOIN forum_users cu ON cu.id = s.user_id
WHERE p.topic_id = $id AND p.deleted_at IS NULL
ORDER BY p.created_at;

-- Reply form (if not locked and logged in)
-- @query component=text
SELECT CASE
    WHEN t.is_locked = 1 THEN
        '<div class="alert alert-info">Ce sujet est verrouille, vous ne pouvez pas repondre.</div>'
    WHEN s.user_id IS NULL THEN
        '<div class="alert alert-info">
            <a href="/forum/login">Connectez-vous</a> pour repondre a ce sujet.
        </div>'
    ELSE
        '<div class="reply-form">
            <h3>Repondre</h3>
            <form action="/forum/api/reply" method="POST">
                <input type="hidden" name="topic_id" value="' || t.id || '">
                <textarea name="content" rows="6" placeholder="Votre reponse..." required></textarea>
                <button type="submit" class="btn btn-primary">Publier la reponse</button>
            </form>
        </div>'
END as html
FROM forum_topics t
LEFT JOIN forum_sessions s ON s.id = $session_id AND s.expires_at > datetime('now')
WHERE t.id = $id;
