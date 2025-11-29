-- Demo: Format Functions
-- Showcases formatting utilities available in GoPage

-- @query component=shell title="Format Functions Demo"

-- @query component=text
SELECT '<h2>Number Formatting</h2>' as html;

-- @query component=table title="Number Formats"
SELECT
    1234567 as "Raw Number",
    format_number(1234567) as "Formatted",
    format_number_decimals(1234567.891, 2) as "With Decimals",
    format_currency(1234.50, '$') as "Currency USD",
    format_currency(1234.50, '€') as "Currency EUR",
    format_percent(0.8534) as "Percent"
UNION ALL
SELECT
    9876543210,
    format_number(9876543210),
    format_number_decimals(9876543210.123, 2),
    format_currency(9876543.21, '$'),
    format_currency(9876543.21, '€'),
    format_percent(0.1234);

-- @query component=text
SELECT '<h2>Byte Size Formatting</h2>' as html;

-- @query component=table title="File Sizes"
SELECT
    512 as "Bytes",
    format_bytes(512) as "Human Readable"
UNION ALL SELECT 1024, format_bytes(1024)
UNION ALL SELECT 1048576, format_bytes(1048576)
UNION ALL SELECT 1073741824, format_bytes(1073741824)
UNION ALL SELECT 5368709120, format_bytes(5368709120);

-- @query component=text
SELECT '<h2>Time Formatting</h2>' as html;

-- @query component=table title="Relative Time"
SELECT
    datetime('now', '-30 seconds') as "Timestamp",
    time_ago(strftime('%s', 'now', '-30 seconds')) as "Relative"
UNION ALL
SELECT datetime('now', '-5 minutes'), time_ago(strftime('%s', 'now', '-5 minutes'))
UNION ALL
SELECT datetime('now', '-2 hours'), time_ago(strftime('%s', 'now', '-2 hours'))
UNION ALL
SELECT datetime('now', '-1 day'), time_ago(strftime('%s', 'now', '-1 day'))
UNION ALL
SELECT datetime('now', '-7 days'), time_ago(strftime('%s', 'now', '-7 days'))
UNION ALL
SELECT datetime('now', '-30 days'), time_ago(strftime('%s', 'now', '-30 days'));

-- @query component=text
SELECT '<h2>Duration Formatting</h2>' as html;

-- @query component=table title="Durations"
SELECT
    45 as "Seconds",
    format_duration(45) as "Formatted"
UNION ALL SELECT 125, format_duration(125)
UNION ALL SELECT 3661, format_duration(3661)
UNION ALL SELECT 90061, format_duration(90061);

-- @query component=text
SELECT '<h2>Text Utilities</h2>' as html;

-- @query component=table title="Pluralization & Ordinals"
SELECT
    1 as "Count",
    pluralize(1, 'item', 'items') as "Pluralized",
    ordinal(1) as "Ordinal"
UNION ALL SELECT 2, pluralize(2, 'item', 'items'), ordinal(2)
UNION ALL SELECT 3, pluralize(3, 'item', 'items'), ordinal(3)
UNION ALL SELECT 11, pluralize(11, 'item', 'items'), ordinal(11)
UNION ALL SELECT 21, pluralize(21, 'item', 'items'), ordinal(21)
UNION ALL SELECT 100, pluralize(100, 'item', 'items'), ordinal(100);

-- @query component=text
SELECT '<p><a href="/demo/functions">← Back to Functions Demo</a></p>' as html;
