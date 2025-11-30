-- Notifications Page
-- @query component=shell title="Notifications"

-- Check if logged in
-- @query component=text
SELECT CASE
    WHEN s.user_id IS NULL THEN
        '<script>window.location.href = "/forum/login";</script>'
    ELSE
        '<h1>Notifications</h1>
         <div class="notification-actions">
             <form action="/forum/api/mark-all-read" method="POST" style="display:inline">
                 <button type="submit" class="btn btn-sm">Tout marquer comme lu</button>
             </form>
         </div>'
END as html
FROM (SELECT 1) dummy
LEFT JOIN forum_sessions s ON s.id = $session_id AND s.expires_at > datetime('now');

-- Unread notifications
-- @query component=table title="Non lues"
SELECT
    '<span class="notification-icon icon-' || n.type || '"></span>' as "",
    '<strong>' || n.title || '</strong><br><small>' || COALESCE(n.message, '') || '</small>' as "Notification",
    time_ago(n.created_at) as "Date",
    '<a href="' || n.link || '" class="btn btn-sm" hx-post="/forum/api/mark-read?id=' || n.id || '">Voir</a>' as ""
FROM forum_notifications n
JOIN forum_sessions s ON s.user_id = n.user_id
WHERE s.id = $session_id AND s.expires_at > datetime('now') AND n.is_read = 0
ORDER BY n.created_at DESC
LIMIT 20;

-- Read notifications
-- @query component=table title="Historique"
SELECT
    n.title as "Notification",
    COALESCE(n.message, '') as "Details",
    time_ago(n.created_at) as "Date"
FROM forum_notifications n
JOIN forum_sessions s ON s.user_id = n.user_id
WHERE s.id = $session_id AND s.expires_at > datetime('now') AND n.is_read = 1
ORDER BY n.created_at DESC
LIMIT 50;
