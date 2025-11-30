-- Login Page
-- @query component=shell title="Connexion"

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

-- Login form
-- @query component=form action="/forum/api/login" method="POST"
SELECT
    'text' as type,
    'username' as name,
    'Nom utilisateur ou email' as label,
    'Entrez votre identifiant' as placeholder,
    1 as required
UNION ALL SELECT
    'password' as type,
    'password' as name,
    'Mot de passe' as label,
    'Entrez votre mot de passe' as placeholder,
    1 as required
UNION ALL SELECT
    'checkbox' as type,
    'remember' as name,
    'Se souvenir de moi' as label,
    '' as placeholder,
    0 as required
UNION ALL SELECT
    'submit' as type,
    'submit' as name,
    'Se connecter' as label,
    '' as placeholder,
    0 as required;

-- Links
-- @query component=text
SELECT '<div class="form-links">
    <a href="/forum/register">Pas encore inscrit ? Creer un compte</a><br>
    <a href="/forum/forgot-password">Mot de passe oublie ?</a>
</div>' as html;
