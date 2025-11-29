-- GoPage Index Page
-- This is the homepage of your GoPage application

-- @query component=shell title="Welcome to GoPage"

-- @query component=text
SELECT 'Welcome to GoPage!' as content;

-- @query component=text
SELECT '<p>GoPage is a SQL-driven web application server written in Go.</p>
<p>Create <code>.sql</code> files in the <code>sql/</code> directory to build your application.</p>
<h3>Features</h3>
<ul>
<li>SQL-first development</li>
<li>HTMX integration for dynamic UIs</li>
<li>Alpine.js for client-side interactivity</li>
<li>Multiple built-in components</li>
</ul>' as html;

-- @query component=card title="Quick Links"
SELECT
    'Demo Table' as title,
    'See a table component in action' as description,
    '/demo/table' as link,
    'View Demo' as action
UNION ALL
SELECT
    'Demo Form' as title,
    'Try the form component' as description,
    '/demo/form' as link,
    'View Demo' as action
UNION ALL
SELECT
    'Demo List' as title,
    'Check out the list component' as description,
    '/demo/list' as link,
    'View Demo' as action;
