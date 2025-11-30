-- Logout Handler
-- @query component=shell title="Deconnexion"

-- Delete session only if it exists and belongs to a user (security: verify session validity)
DELETE FROM forum_sessions
WHERE id = $session_id
  AND expires_at > datetime('now');

-- Redirect
-- @query component=text
SELECT '<div class="alert alert-success">Vous avez ete deconnecte</div>
<script>
    document.cookie = "session_id=; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT";
    setTimeout(() => window.location.href = "/forum", 1500);
</script>' as html;
