-- Register API Handler
-- Handles POST /forum/api/register

-- Step 1: Insert user (only if all validation passes)
-- Uses INSERT OR IGNORE to avoid errors on duplicate
INSERT OR IGNORE INTO forum_users (username, email, password_hash, display_name)
SELECT
    $username,
    $email,
    hash_password($password),
    COALESCE(NULLIF($display_name, ''), $username)
WHERE length($username) >= 3
    AND $password = $password_confirm
    AND length($password) >= 8
    AND NOT EXISTS(SELECT 1 FROM forum_users WHERE username = $username)
    AND NOT EXISTS(SELECT 1 FROM forum_users WHERE email = $email)
    AND $terms = 'on';

-- Step 2: Show result based on actual database state
-- @query component=text
SELECT CASE
    WHEN length($username) < 3 THEN
        '<div class="alert alert-error">Le nom utilisateur doit faire au moins 3 caractères</div>
         <script>setTimeout(() => window.history.back(), 2000);</script>'
    WHEN $password != $password_confirm THEN
        '<div class="alert alert-error">Les mots de passe ne correspondent pas</div>
         <script>setTimeout(() => window.history.back(), 2000);</script>'
    WHEN length($password) < 8 THEN
        '<div class="alert alert-error">Le mot de passe doit faire au moins 8 caractères</div>
         <script>setTimeout(() => window.history.back(), 2000);</script>'
    WHEN $terms != 'on' THEN
        '<div class="alert alert-error">Vous devez accepter les conditions</div>
         <script>setTimeout(() => window.history.back(), 2000);</script>'
    WHEN EXISTS(SELECT 1 FROM forum_users WHERE username = $username AND email = $email) THEN
        '<div class="alert alert-success">Compte créé avec succès ! Redirection...</div>
         <script>setTimeout(() => window.location.href = "/forum/login", 2000);</script>'
    WHEN EXISTS(SELECT 1 FROM forum_users WHERE username = $username) THEN
        '<div class="alert alert-error">Ce nom utilisateur est déjà pris</div>
         <script>setTimeout(() => window.history.back(), 2000);</script>'
    WHEN EXISTS(SELECT 1 FROM forum_users WHERE email = $email) THEN
        '<div class="alert alert-error">Cette adresse email est déjà utilisée</div>
         <script>setTimeout(() => window.history.back(), 2000);</script>'
    ELSE
        '<div class="alert alert-error">Erreur lors de la création du compte</div>
         <script>setTimeout(() => window.history.back(), 2000);</script>'
END as html;
