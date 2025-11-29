-- Handler: Send SSE Message
-- Broadcasts a message to all clients on the "demo" channel

-- @query component=shell title="Send Message"

-- Get the message parameter and broadcast it
-- @query component=text
SELECT sse_notify('demo', '<div class="fade-in"><strong>' || datetime('now') || ':</strong> ' || $message || '</div>') as result;

-- Redirect back to the realtime demo page
-- @query component=redirect target="/demo/realtime"
SELECT '/demo/realtime' as target;
