package engine

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"zombiezen.com/go/sqlite"
)

// Result represents the result of a query execution.
type Result struct {
	// Query is the original query that was executed
	Query Query

	// Rows contains the result rows as maps
	Rows []map[string]interface{}

	// Columns contains column names in order
	Columns []string

	// RowsAffected is set for INSERT/UPDATE/DELETE
	RowsAffected int64
}

// Executor executes SQL queries with parameter binding.
type Executor struct{}

// NewExecutor creates a new query executor.
func NewExecutor() *Executor {
	return &Executor{}
}

// Params represents query parameters.
type Params map[string]string

// Execute runs a query and returns results.
func (e *Executor) Execute(ctx context.Context, conn *sqlite.Conn, query Query, params Params) (*Result, error) {
	result := &Result{
		Query:   query,
		Rows:    []map[string]interface{}{},
		Columns: []string{},
	}

	// Normalize parameter syntax ($param -> :param for binding)
	sql := normalizeParams(query.SQL)

	// Prepare statement
	stmt, _, err := conn.PrepareTransient(sql)
	if err != nil {
		return nil, fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Finalize()

	// Bind parameters
	if err := bindParams(stmt, params); err != nil {
		return nil, fmt.Errorf("bind: %w", err)
	}

	// Check if this is a SELECT query
	if isSelectQuery(query.SQL) {
		// Get column names
		colCount := stmt.ColumnCount()
		for i := 0; i < colCount; i++ {
			result.Columns = append(result.Columns, stmt.ColumnName(i))
		}

		// Read rows
		for {
			hasRow, err := stmt.Step()
			if err != nil {
				return nil, fmt.Errorf("step: %w", err)
			}
			if !hasRow {
				break
			}

			row := make(map[string]interface{})
			for i := 0; i < colCount; i++ {
				colName := result.Columns[i]
				row[colName] = getColumnValue(stmt, i)
			}
			result.Rows = append(result.Rows, row)
		}
	} else {
		// Execute non-SELECT query
		_, err := stmt.Step()
		if err != nil {
			return nil, fmt.Errorf("exec: %w", err)
		}
		result.RowsAffected = int64(conn.Changes())
	}

	return result, nil
}

// ExecuteFile executes all queries in a file and returns results.
func (e *Executor) ExecuteFile(ctx context.Context, conn *sqlite.Conn, file *File, params Params) ([]*Result, error) {
	var results []*Result
	for _, query := range file.Queries {
		result, err := e.Execute(ctx, conn, query, params)
		if err != nil {
			return results, fmt.Errorf("query %q: %w", query.Component, err)
		}
		results = append(results, result)
	}
	return results, nil
}

// normalizeParams converts $param to :param syntax.
func normalizeParams(sql string) string {
	re := regexp.MustCompile(`\$(\w+)`)
	return re.ReplaceAllString(sql, ":$1")
}

// bindParams binds parameters to a prepared statement.
func bindParams(stmt *sqlite.Stmt, params Params) error {
	// Build a map of parameter names to indices
	paramIndices := make(map[string]int)
	for i := 1; i <= stmt.BindParamCount(); i++ {
		name := stmt.BindParamName(i)
		if name != "" {
			paramIndices[name] = i
		}
	}

	// Bind each provided parameter
	for name, value := range params {
		// Try :name format
		if idx, ok := paramIndices[":"+name]; ok {
			stmt.BindText(idx, value)
			continue
		}
		// Try $name format
		if idx, ok := paramIndices["$"+name]; ok {
			stmt.BindText(idx, value)
		}
	}
	return nil
}

// isSelectQuery checks if a query is a SELECT statement.
func isSelectQuery(sql string) bool {
	trimmed := strings.TrimSpace(strings.ToUpper(sql))
	return strings.HasPrefix(trimmed, "SELECT") ||
		strings.HasPrefix(trimmed, "WITH") ||
		strings.HasPrefix(trimmed, "PRAGMA")
}

// getColumnValue extracts a value from a result column.
func getColumnValue(stmt *sqlite.Stmt, idx int) interface{} {
	switch stmt.ColumnType(idx) {
	case sqlite.TypeNull:
		return nil
	case sqlite.TypeInteger:
		return stmt.ColumnInt64(idx)
	case sqlite.TypeFloat:
		return stmt.ColumnFloat(idx)
	case sqlite.TypeText:
		return stmt.ColumnText(idx)
	case sqlite.TypeBlob:
		return stmt.ColumnBytes(idx, nil)
	default:
		return stmt.ColumnText(idx)
	}
}
