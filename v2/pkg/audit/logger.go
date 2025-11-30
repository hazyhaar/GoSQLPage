// Package audit provides audit logging for GoSQLPage v2.1.
package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/hazyhaar/gopage/v2/pkg/blocks"
	_ "modernc.org/sqlite"
)

// Logger provides audit logging functionality.
type Logger struct {
	db     *sql.DB
	cfg    Config
	logger *slog.Logger
}

// LoggerConfig holds logger configuration.
type LoggerConfig struct {
	DBPath string
	Config Config
	Logger *slog.Logger
}

// NewLogger creates a new audit logger.
func NewLogger(cfg LoggerConfig) (*Logger, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	db, err := sql.Open("sqlite", cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open audit db: %w", err)
	}

	// Enable WAL mode
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable WAL: %w", err)
	}

	return &Logger{
		db:     db,
		cfg:    cfg.Config,
		logger: cfg.Logger,
	}, nil
}

// LogBlockChange logs a block modification.
func (l *Logger) LogBlockChange(ctx context.Context, entry *Entry) error {
	metadataJSON := ""
	if entry.Metadata != "" {
		metadataJSON = entry.Metadata
	}

	// Check if we should store content
	storeContent := l.cfg.StoreContent || l.shouldStoreContentForType(entry.BlockType)

	beforeContent := ""
	afterContent := ""
	if storeContent {
		beforeContent = entry.BeforeContent
		afterContent = entry.AfterContent
	}

	_, err := l.db.ExecContext(ctx, `
		INSERT INTO audit_log
		(user_id, user_type, session_id, operation, block_id, block_type,
		 before_hash, after_hash, before_content, after_content, diff, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.UserID, entry.UserType, entry.SessionID, entry.Operation,
		entry.BlockID, entry.BlockType, entry.BeforeHash, entry.AfterHash,
		beforeContent, afterContent, entry.Diff, metadataJSON)

	if err != nil {
		l.logger.Error("log block change failed", "error", err)
	}

	return err
}

// LogInsert logs a block insertion.
func (l *Logger) LogInsert(ctx context.Context, userID, userType, sessionID string, block *blocks.Block, meta *Metadata) error {
	afterJSON, _ := json.Marshal(block)

	return l.LogBlockChange(ctx, &Entry{
		UserID:       userID,
		UserType:     userType,
		SessionID:    sessionID,
		Operation:    OpInsert,
		BlockID:      block.ID,
		BlockType:    block.Type,
		AfterHash:    block.Hash,
		AfterContent: block.Content,
		Diff:         string(afterJSON),
		Metadata:     metadataToJSON(meta),
	})
}

// LogUpdate logs a block update.
func (l *Logger) LogUpdate(ctx context.Context, userID, userType, sessionID string, before, after *blocks.Block, meta *Metadata) error {
	diffJSON := computeDiff(before, after)

	return l.LogBlockChange(ctx, &Entry{
		UserID:        userID,
		UserType:      userType,
		SessionID:     sessionID,
		Operation:     OpUpdate,
		BlockID:       after.ID,
		BlockType:     after.Type,
		BeforeHash:    before.Hash,
		AfterHash:     after.Hash,
		BeforeContent: before.Content,
		AfterContent:  after.Content,
		Diff:          diffJSON,
		Metadata:      metadataToJSON(meta),
	})
}

// LogDelete logs a block deletion.
func (l *Logger) LogDelete(ctx context.Context, userID, userType, sessionID string, block *blocks.Block, meta *Metadata) error {
	beforeJSON, _ := json.Marshal(block)

	return l.LogBlockChange(ctx, &Entry{
		UserID:        userID,
		UserType:      userType,
		SessionID:     sessionID,
		Operation:     OpDelete,
		BlockID:       block.ID,
		BlockType:     block.Type,
		BeforeHash:    block.Hash,
		BeforeContent: block.Content,
		Diff:          string(beforeJSON),
		Metadata:      metadataToJSON(meta),
	})
}

// LogMerge logs a merge operation.
func (l *Logger) LogMerge(ctx context.Context, log *MergeLog) error {
	metadataJSON := ""
	if log.Metadata != "" {
		metadataJSON = log.Metadata
	}

	_, err := l.db.ExecContext(ctx, `
		INSERT INTO merge_log
		(session_id, user_id, status, blocks_inserted, blocks_updated, blocks_deleted, duration_ms, error_message, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.SessionID, log.UserID, log.Status, log.BlocksInserted,
		log.BlocksUpdated, log.BlocksDeleted, log.DurationMS, log.ErrorMessage, metadataJSON)

	return err
}

// Query retrieves audit log entries matching the query.
func (l *Logger) Query(ctx context.Context, q *Query) (*QueryResult, error) {
	// Build query
	query := "SELECT id, timestamp, user_id, user_type, session_id, operation, block_id, block_type, before_hash, after_hash, before_content, after_content, diff, metadata FROM audit_log WHERE 1=1"
	args := []interface{}{}

	if q.BlockID != "" {
		query += " AND block_id = ?"
		args = append(args, q.BlockID)
	}
	if q.UserID != "" {
		query += " AND user_id = ?"
		args = append(args, q.UserID)
	}
	if q.Operation != nil {
		query += " AND operation = ?"
		args = append(args, *q.Operation)
	}
	if q.From != nil {
		query += " AND timestamp >= ?"
		args = append(args, q.From.Format(time.RFC3339))
	}
	if q.To != nil {
		query += " AND timestamp <= ?"
		args = append(args, q.To.Format(time.RFC3339))
	}

	query += " ORDER BY timestamp DESC"

	// Get total count
	countQuery := "SELECT COUNT(*) FROM (" + query + ")"
	var totalCount int
	l.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)

	// Apply pagination
	if q.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", q.Limit)
	}
	if q.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", q.Offset)
	}

	rows, err := l.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*Entry
	for rows.Next() {
		var entry Entry
		var timestamp string
		var sessionID, beforeHash, afterHash, beforeContent, afterContent, diff, metadata sql.NullString

		err := rows.Scan(&entry.ID, &timestamp, &entry.UserID, &entry.UserType,
			&sessionID, &entry.Operation, &entry.BlockID, &entry.BlockType,
			&beforeHash, &afterHash, &beforeContent, &afterContent, &diff, &metadata)
		if err != nil {
			continue
		}

		entry.Timestamp, _ = time.Parse(time.RFC3339Nano, timestamp)
		if sessionID.Valid {
			entry.SessionID = sessionID.String
		}
		if beforeHash.Valid {
			entry.BeforeHash = beforeHash.String
		}
		if afterHash.Valid {
			entry.AfterHash = afterHash.String
		}
		if beforeContent.Valid {
			entry.BeforeContent = beforeContent.String
		}
		if afterContent.Valid {
			entry.AfterContent = afterContent.String
		}
		if diff.Valid {
			entry.Diff = diff.String
		}
		if metadata.Valid {
			entry.Metadata = metadata.String
		}

		entries = append(entries, &entry)
	}

	hasMore := false
	if q.Limit > 0 {
		hasMore = q.Offset+len(entries) < totalCount
	}

	return &QueryResult{
		Entries:    entries,
		TotalCount: totalCount,
		HasMore:    hasMore,
	}, nil
}

// GetBlockHistory returns the history for a specific block.
func (l *Logger) GetBlockHistory(ctx context.Context, blockID string, limit int) ([]*Entry, error) {
	if limit <= 0 {
		limit = 50
	}

	result, err := l.Query(ctx, &Query{
		BlockID: blockID,
		Limit:   limit,
	})
	if err != nil {
		return nil, err
	}

	return result.Entries, nil
}

// shouldStoreContentForType checks if content should be stored for a block type.
func (l *Logger) shouldStoreContentForType(blockType string) bool {
	for _, t := range l.cfg.StoreContentTypes {
		if t == blockType {
			return true
		}
	}
	return false
}

// Close closes the audit logger.
func (l *Logger) Close() error {
	return l.db.Close()
}

// computeDiff computes a JSON diff between two blocks.
func computeDiff(before, after *blocks.Block) string {
	diff := map[string]interface{}{}

	if before.Content != after.Content {
		diff["content"] = map[string]string{
			"before": before.Content,
			"after":  after.Content,
		}
	}
	if before.Position != after.Position {
		diff["position"] = map[string]string{
			"before": before.Position,
			"after":  after.Position,
		}
	}
	if before.Published != after.Published {
		diff["published"] = map[string]bool{
			"before": before.Published,
			"after":  after.Published,
		}
	}

	diffJSON, _ := json.Marshal(diff)
	return string(diffJSON)
}

// metadataToJSON converts metadata to JSON string.
func metadataToJSON(meta *Metadata) string {
	if meta == nil {
		return ""
	}
	j, _ := json.Marshal(meta)
	return string(j)
}
