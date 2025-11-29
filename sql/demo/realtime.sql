-- Demo: Real-time Updates with SSE
-- Demonstrates Server-Sent Events for live updates

-- @query component=shell title="Real-time Demo"

-- @query component=text
SELECT '<p>This page demonstrates real-time updates using Server-Sent Events (SSE).</p>
<p>Open this page in multiple browser tabs to see updates appear in real-time!</p>' as html;

-- Live message display
-- @query component=sse channel="demo" target="#live-messages" event="message" title="Live Messages"
SELECT '<em>Waiting for messages...</em>' as html;

-- Form to send messages
-- @query component=text
SELECT '<hr><h3>Send a Message</h3>' as html;

-- @query component=form title="Broadcast Message" action="/demo/realtime/send" method="POST"
SELECT 'text' as type, 'message' as name, 'Your Message' as label, 'Type something...' as placeholder, 1 as required
UNION ALL
SELECT 'submit', '', 'Send to All', '', 0;

-- Stats
-- @query component=text
SELECT '<hr><h3>Connection Stats</h3>
<p>Connected clients: <strong>' || sse_client_count() || '</strong></p>
<p>Clients on "demo" channel: <strong>' || sse_channel_count('demo') || '</strong></p>' as html;

-- Info about SSE functions
-- @query component=text
SELECT '<hr><h3>SSE SQL Functions</h3>
<ul>
<li><code>sse_notify(channel, data)</code> - Send message to channel</li>
<li><code>sse_notify_event(channel, event_type, data)</code> - Send typed event</li>
<li><code>sse_broadcast(data)</code> - Send to all clients</li>
<li><code>sse_client_count()</code> - Get total connected clients</li>
<li><code>sse_channel_count(channel)</code> - Get clients in channel</li>
</ul>' as html;

-- @query component=text
SELECT '<p><a href="/">&larr; Back to Home</a></p>' as html;
