// Package engine provides SQL parsing and execution for GoPage.
package engine

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Query represents a parsed SQL query with its metadata.
type Query struct {
	// Component is the UI component to render (table, form, text, etc.)
	Component string

	// SQL is the actual SQL statement to execute
	SQL string

	// Options are key-value pairs from the query annotation
	Options map[string]string
}

// File represents a parsed SQL file containing multiple queries.
type File struct {
	Path    string
	Queries []Query
}

// Parser parses SQL files with GoPage conventions.
type Parser struct{}

// NewParser creates a new SQL parser.
func NewParser() *Parser {
	return &Parser{}
}

// queryAnnotationRegex matches: -- @query component=table title="My Title" ...
var queryAnnotationRegex = regexp.MustCompile(`^--\s*@query\s+(.*)$`)

// optionRegex matches: key=value or key="value with spaces"
var optionRegex = regexp.MustCompile(`(\w+)=(?:"([^"]+)"|(\S+))`)

// ParseFile reads and parses a SQL file.
func (p *Parser) ParseFile(path string) (*File, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return p.Parse(path, string(content))
}

// Parse parses SQL content with GoPage conventions.
//
// Convention:
//
//	-- @query component=table title="Users"
//	SELECT id, name FROM users;
//
//	-- @query component=text
//	SELECT 'Hello World' as content;
func (p *Parser) Parse(path, content string) (*File, error) {
	file := &File{
		Path:    path,
		Queries: []Query{},
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	var currentQuery *Query
	var sqlBuilder strings.Builder

	flushQuery := func() {
		if currentQuery != nil {
			sql := strings.TrimSpace(sqlBuilder.String())
			if sql != "" {
				currentQuery.SQL = sql
				file.Queries = append(file.Queries, *currentQuery)
			}
		}
		currentQuery = nil
		sqlBuilder.Reset()
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Check for query annotation
		if matches := queryAnnotationRegex.FindStringSubmatch(trimmed); matches != nil {
			// Flush previous query
			flushQuery()

			// Parse new query
			currentQuery = &Query{
				Component: "text", // default component
				Options:   make(map[string]string),
			}

			// Parse options
			optMatches := optionRegex.FindAllStringSubmatch(matches[1], -1)
			for _, m := range optMatches {
				key := m[1]
				value := m[2]
				if value == "" {
					value = m[3]
				}
				if key == "component" {
					currentQuery.Component = value
				} else {
					currentQuery.Options[key] = value
				}
			}
			continue
		}

		// Skip empty lines between queries (but not within)
		if currentQuery == nil && trimmed == "" {
			continue
		}

		// Skip regular comments (not annotations)
		if strings.HasPrefix(trimmed, "--") && !strings.HasPrefix(trimmed, "-- @") {
			continue
		}

		// If we have SQL but no annotation, create implicit query
		if currentQuery == nil && trimmed != "" {
			currentQuery = &Query{
				Component: "text",
				Options:   make(map[string]string),
			}
		}

		// Accumulate SQL
		if currentQuery != nil {
			if sqlBuilder.Len() > 0 {
				sqlBuilder.WriteString("\n")
			}
			sqlBuilder.WriteString(line)
		}
	}

	// Flush last query
	flushQuery()

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan error: %w", err)
	}

	return file, nil
}

// ExtractParams finds all parameter placeholders in a query.
// Supports $param and :param syntax.
func ExtractParams(sql string) []string {
	re := regexp.MustCompile(`[$:](\w+)`)
	matches := re.FindAllStringSubmatch(sql, -1)
	seen := make(map[string]bool)
	var params []string
	for _, m := range matches {
		if !seen[m[1]] {
			seen[m[1]] = true
			params = append(params, m[1])
		}
	}
	return params
}
