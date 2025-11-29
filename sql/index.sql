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

-- @query component=card title="Application"
SELECT
    'Users' as title,
    'Manage application users' as description,
    '/users' as link,
    'View Users' as action
UNION ALL
SELECT
    'Posts' as title,
    'Browse and create posts' as description,
    '/posts' as link,
    'View Posts' as action;

-- @query component=card title="Component Demos"
SELECT
    'Table Demo' as title,
    'See table component with data' as description,
    '/demo/table' as link,
    'View' as action
UNION ALL
SELECT
    'Form Demo' as title,
    'Try the form component' as description,
    '/demo/form' as link,
    'View' as action
UNION ALL
SELECT
    'List Demo' as title,
    'Check out the list component' as description,
    '/demo/list' as link,
    'View' as action
UNION ALL
SELECT
    'SQL Functions' as title,
    'Custom SQL functions (hash, JSON, HTTP, LLM)' as description,
    '/demo/functions' as link,
    'View' as action;
