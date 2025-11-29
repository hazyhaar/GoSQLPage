-- Demo: Custom SQL Functions
-- Showcases all built-in custom functions

-- @query component=shell title="Custom SQL Functions Demo"

-- @query component=text
SELECT '<p>GoPage extends SQLite with powerful custom functions for strings, hashing, JSON, HTTP, and more.</p>' as html;

-- String Functions
-- @query component=table title="String Functions"
SELECT 'str_reverse' as function, 'str_reverse(''hello'')' as example, str_reverse('hello') as result
UNION ALL
SELECT 'str_repeat', 'str_repeat(''ab'', 3)', str_repeat('ab', 3)
UNION ALL
SELECT 'str_slug', 'str_slug(''Hello World!'')' , str_slug('Hello World!')
UNION ALL
SELECT 'str_truncate', 'str_truncate(''Long text here'', 10)', str_truncate('Long text here', 10)
UNION ALL
SELECT 'str_contains', 'str_contains(''hello'', ''ell'')', str_contains('hello', 'ell')
UNION ALL
SELECT 'str_split', 'str_split(''a,b,c'', '','', 1)', str_split('a,b,c', ',', 1);

-- Hash Functions
-- @query component=table title="Hash & Encoding Functions"
SELECT 'hash_md5' as function, 'hash_md5(''password'')' as example, hash_md5('password') as result
UNION ALL
SELECT 'hash_sha256', 'hash_sha256(''data'')', substr(hash_sha256('data'), 1, 32) || '...'
UNION ALL
SELECT 'base64_encode', 'base64_encode(''hello'')', base64_encode('hello')
UNION ALL
SELECT 'base64_decode', 'base64_decode(''aGVsbG8='')', base64_decode('aGVsbG8=')
UNION ALL
SELECT 'hex_encode', 'hex_encode(''ABC'')', hex_encode('ABC');

-- JSON Functions
-- @query component=table title="JSON Functions"
SELECT 'json_get' as function, 'json_get(''{"name":"John"}'', ''name'')' as example, json_get('{"name":"John"}', 'name') as result
UNION ALL
SELECT 'json_set', 'json_set(''{"a":1}'', ''b'', ''2'')', json_set('{"a":1}', 'b', '2')
UNION ALL
SELECT 'json_keys', 'json_keys(''{"a":1,"b":2}'')', json_keys('{"a":1,"b":2}')
UNION ALL
SELECT 'json_array_length', 'json_array_length(''[1,2,3]'')', json_array_length('[1,2,3]');

-- Utility Functions
-- @query component=table title="Utility Functions"
SELECT 'uuid' as function, 'uuid()' as example, uuid() as result
UNION ALL
SELECT 'uuid_short', 'uuid_short()', uuid_short()
UNION ALL
SELECT 'random_int', 'random_int(1, 100)', random_int(1, 100)
UNION ALL
SELECT 'random_string', 'random_string(8)', random_string(8)
UNION ALL
SELECT 'now_unix', 'now_unix()', now_unix()
UNION ALL
SELECT 'now_iso', 'now_iso()', now_iso();

-- HTTP Functions (examples - may not work without network)
-- @query component=text
SELECT '<h3>HTTP Functions</h3>
<p>Available HTTP functions (require network access):</p>
<ul>
<li><code>http_get(url)</code> - GET request, returns body</li>
<li><code>http_get_json(url, path)</code> - GET JSON and extract path</li>
<li><code>http_post(url, content_type, body)</code> - POST request</li>
<li><code>http_post_json(url, json_body)</code> - POST JSON</li>
<li><code>url_encode(string)</code> - URL encode</li>
<li><code>url_decode(string)</code> - URL decode</li>
</ul>' as html;

-- LLM Functions (examples - require API key)
-- @query component=text
SELECT '<h3>LLM Functions</h3>
<p>AI-powered functions (require LLM_API_KEY environment variable):</p>
<ul>
<li><code>llm_complete(prompt)</code> - Complete a prompt</li>
<li><code>llm_complete_with_model(prompt, model)</code> - Use specific model</li>
<li><code>llm_summarize(text)</code> - Summarize text</li>
<li><code>llm_translate(text, language)</code> - Translate text</li>
<li><code>llm_extract(text, what)</code> - Extract information</li>
<li><code>llm_classify(text, categories)</code> - Classify into categories</li>
</ul>
<p>Set <code>LLM_API_KEY</code>, <code>LLM_API_URL</code>, and <code>LLM_MODEL</code> environment variables to configure.</p>' as html;

-- @query component=text
SELECT '<p><a href="/">&larr; Back to Home</a></p>' as html;
