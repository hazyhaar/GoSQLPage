// Package audit provides audit logging types for GoSQLPage v2.1.
package audit

import (
	"time"
)

// Operation represents an audit operation type.
type Operation string

const (
	OpInsert  Operation = "insert"
	OpUpdate  Operation = "update"
	OpDelete  Operation = "delete"
	OpMerge   Operation = "merge"
	OpPublish Operation = "publish"
)

// Entry represents an audit log entry.
type Entry struct {
	ID            int64     `json:"id"`
	Timestamp     time.Time `json:"timestamp"`
	UserID        string    `json:"user_id"`
	UserType      string    `json:"user_type"`
	SessionID     string    `json:"session_id,omitempty"`
	Operation     Operation `json:"operation"`
	BlockID       string    `json:"block_id"`
	BlockType     string    `json:"block_type"`
	BeforeHash    string    `json:"before_hash,omitempty"`
	AfterHash     string    `json:"after_hash,omitempty"`
	BeforeContent string    `json:"before_content,omitempty"`
	AfterContent  string    `json:"after_content,omitempty"`
	Diff          string    `json:"diff,omitempty"` // JSON diff
	Metadata      string    `json:"metadata,omitempty"` // JSON metadata
}

// MergeLog represents a merge operation log entry.
type MergeLog struct {
	ID             int64     `json:"id"`
	Timestamp      time.Time `json:"timestamp"`
	SessionID      string    `json:"session_id"`
	UserID         string    `json:"user_id"`
	Status         string    `json:"status"` // success, conflict, failed
	BlocksInserted int       `json:"blocks_inserted"`
	BlocksUpdated  int       `json:"blocks_updated"`
	BlocksDeleted  int       `json:"blocks_deleted"`
	DurationMS     int64     `json:"duration_ms"`
	ErrorMessage   string    `json:"error_message,omitempty"`
	Metadata       string    `json:"metadata,omitempty"`
}

// Config represents audit configuration.
type Config struct {
	StoreContent      bool     `json:"store_content"`
	StoreContentTypes []string `json:"store_content_types"`
	RetentionDays     int      `json:"retention_days"`
	ArchiveAfterDays  int      `json:"archive_after_days"`
}

// Metadata represents additional audit metadata.
type Metadata struct {
	IPAddress  string            `json:"ip_address,omitempty"`
	UserAgent  string            `json:"user_agent,omitempty"`
	RequestID  string            `json:"request_id,omitempty"`
	Extra      map[string]string `json:"extra,omitempty"`
}

// Query represents an audit query.
type Query struct {
	BlockID   string     `json:"block_id,omitempty"`
	UserID    string     `json:"user_id,omitempty"`
	Operation *Operation `json:"operation,omitempty"`
	From      *time.Time `json:"from,omitempty"`
	To        *time.Time `json:"to,omitempty"`
	Limit     int        `json:"limit,omitempty"`
	Offset    int        `json:"offset,omitempty"`
}

// QueryResult represents audit query results.
type QueryResult struct {
	Entries    []*Entry `json:"entries"`
	TotalCount int      `json:"total_count"`
	HasMore    bool     `json:"has_more"`
}
