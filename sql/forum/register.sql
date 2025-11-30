-- Registration Page
-- @query component=shell title="Inscription"

-- Check if already logged in
-- @query component=text
SELECT CASE
    WHEN s.user_id IS NOT NULL THEN
        '<script>window.location.href = "/forum";</script>
         <p>Vous etes deja connecte. <a href="/forum">Retour au forum</a></p>'
    ELSE ''
END as html
FROM (SELECT 1) dummy
LEFT JOIN forum_sessions s ON s.id = $session_id AND s.expires_at > datetime('now');

-- Registration form
-- @query component=form action="/forum/api/register" method="POST"
SELECT
    'text' as type,
    'username' as name,
    'Nom utilisateur' as label,
    'Choisissez un nom unique' as placeholder,
    1 as required
UNION ALL SELECT
    'email' as type,
    'email' as name,
    'Adresse email' as label,
    'votre@email.com' as placeholder,
    1 as required
UNION ALL SELECT
    'text' as type,
    'display_name' as name,
    'Nom affiche' as label,
    'Comment voulez-vous etre appele ?' as placeholder,
    0 as required
UNION ALL SELECT
    'password' as type,
    'password' as name,
    'Mot de passe' as label,
    'Minimum 8 caracteres' as placeholder,
    1 as required
UNION ALL SELECT
    'password' as type,
    'password_confirm' as name,
    'Confirmer le mot de passe' as label,
    'Retapez votre mot de passe' as placeholder,
    1 as required
UNION ALL SELECT
    'checkbox' as type,
    'terms' as name,
    'J accepte les conditions d utilisation' as label,
    '' as placeholder,
    1 as required
UNION ALL SELECT
    'submit' as type,
    'submit' as name,
    'Creer mon compte' as label,
    '' as placeholder,
    0 as required;

-- Links
-- @query component=text
SELECT '<div class="form-links">
    <a href="/forum/login">Deja inscrit ? Se connecter</a>
</div>' as html;
