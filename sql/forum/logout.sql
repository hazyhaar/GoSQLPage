-- Logout Handler
-- @query component=shell title="Déconnexion"

-- Delete session only if it exists and is not expired
DELETE FROM forum_sessions
WHERE id = $session_id
  AND expires_at > datetime('now')
  AND user_id IS NOT NULL;

-- Redirect
-- @query component=text
SELECT '<div class="alert alert-success">Vous avez été déconnecté</div>
<script>
    document.cookie = "session_id=; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT";
    setTimeout(() => window.location.href = "/forum", 1500);
</script>' as html;
