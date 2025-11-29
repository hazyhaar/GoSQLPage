-- Demo: Form Component
-- Shows how to create forms

-- @query component=shell title="Form Demo"

-- @query component=text
SELECT 'This page demonstrates the form component.' as content;

-- @query component=form title="Contact Form" action="/demo/form" method="POST"
SELECT 'text' as type, 'name' as name, 'Your Name' as label, 'Enter your name' as placeholder, 1 as required
UNION ALL
SELECT 'email', 'email', 'Email Address', 'you@example.com', 1
UNION ALL
SELECT 'textarea', 'message', 'Message', 'Type your message here...', 1
UNION ALL
SELECT 'checkbox', 'subscribe', 'Subscribe to newsletter', '', 0
UNION ALL
SELECT 'submit', 'submit', 'Send Message', '', 0;

-- @query component=text
SELECT '<p><a href="/">Back to Home</a></p>' as html;
