# GoPage

A SQL-driven web application server written in Go. Build web applications using SQL files with built-in components for tables, forms, lists, and more.

> SQLPage on zombiezen steroids with HTMX/Alpine outfit.

## Features

- **SQL-First Development**: Define your pages using SQL files
- **Built-in Components**: table, form, list, card, text
- **HTMX Integration**: Dynamic UIs without writing JavaScript
- **Alpine.js**: Lightweight client-side interactivity
- **SQLite Backend**: Using zombiezen.com/go/sqlite (pure Go, no CGO)
- **Secure by Design**: Parameter binding prevents SQL injection

## Quick Start

```bash
# Install dependencies
go mod tidy

# Build
go build -o gopage ./cmd/gopage

# Run
./gopage -db myapp.db -sql ./sql -port 8080
```

Then open http://localhost:8080

## Project Structure

```
gopage/
├── cmd/gopage/           # Entry point
├── pkg/
│   ├── db/               # SQLite connection pool (reader/writer pattern)
│   ├── engine/           # SQL parser & executor
│   ├── render/           # HTML rendering & components
│   └── server/           # HTTP server (Chi router)
├── internal/templates/   # Embedded HTML templates
├── sql/                  # Your SQL pages go here
└── assets/               # Static files (CSS/JS)
```

## Writing SQL Pages

Create `.sql` files in the `sql/` directory:

```sql
-- @query component=shell title="My Page"

-- @query component=text
SELECT 'Hello, World!' as content;

-- @query component=table title="Users"
SELECT id, name, email FROM users;
```

### Available Components

| Component | Description |
|-----------|-------------|
| `shell`   | Sets page title and metadata |
| `text`    | Displays simple text or HTML |
| `table`   | Renders data as an HTML table |
| `list`    | Displays items as a list |
| `card`    | Shows data as cards in a grid |
| `form`    | Generates HTML forms |

### Query Annotation Syntax

```sql
-- @query component=table title="My Title" option="value"
SELECT * FROM my_table WHERE id = $id;
```

### Parameters

Use `$param` or `:param` syntax for URL query parameters:

```sql
-- URL: /users?role=admin
SELECT * FROM users WHERE role = $role;
```

## Configuration

| Flag | Default | Description |
|------|---------|-------------|
| `-db` | `gopage.db` | SQLite database path |
| `-sql` | `./sql` | SQL files directory |
| `-port` | `8080` | HTTP port |
| `-debug` | `false` | Enable debug logging |

## Architecture

- **Reader/Writer Pool**: Separate connection pools for reads (concurrent) and writes (serialized)
- **WAL Mode**: SQLite Write-Ahead Logging for better concurrency
- **Embedded Templates**: HTML templates compiled into the binary
- **HTMX-Aware**: Serves fragments for HTMX requests, full pages otherwise

## License

MIT
