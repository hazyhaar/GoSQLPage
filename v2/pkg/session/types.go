// Package session provides session management for isolated editing.
package session

import (
	"time"

	"github.com/hazyhaar/gopage/v2/pkg/blocks"
)

// Status represents the session status.
type Status string

const (
	StatusActive    Status = "active"
	StatusSubmitted Status = "submitted"
	StatusMerged    Status = "merged"
	StatusAbandoned Status = "abandoned"
	StatusConflict  Status = "conflict"
)

// Session represents an editing session.
type Session struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	UserType      string    `json:"user_type"` // human, bot, system
	CreatedAt     time.Time `json:"created_at"`
	LastActivity  time.Time `json:"last_activity"`
	BaseSnapshot  string    `json:"base_snapshot"`  // hash of content.db at creation
	SchemaVersion int       `json:"schema_version"` // version of schema.db
	SchemaHash    string    `json:"schema_hash"`
	Status        Status    `json:"status"`

	// Runtime fields (not persisted)
	DBPath string `json:"-"` // path to session.db file
}

// SessionMeta is the metadata stored in _session_meta table.
type SessionMeta struct {
	SessionID     string `json:"session_id"`
	UserID        string `json:"user_id"`
	UserType      string `json:"user_type"`
	CreatedAt     string `json:"created_at"`
	LastActivity  string `json:"last_activity"`
	BaseSnapshot  string `json:"base_snapshot"`
	SchemaVersion int    `json:"schema_version"`
	SchemaHash    string `json:"schema_hash"`
	Status        string `json:"status"`
}

// Diff represents the differences between session and content.db.
type Diff struct {
	SessionID  string        `json:"session_id"`
	Inserts    []*BlockDiff  `json:"inserts,omitempty"`
	Updates    []*BlockDiff  `json:"updates,omitempty"`
	Deletes    []*BlockDiff  `json:"deletes,omitempty"`
	RefChanges []*RefDiff    `json:"ref_changes,omitempty"`
	AttrChanges []*AttrDiff  `json:"attr_changes,omitempty"`
}

// BlockDiff represents a block change.
type BlockDiff struct {
	BlockID string        `json:"block_id"`
	Type    string        `json:"type"`
	Before  *blocks.Block `json:"before,omitempty"`
	After   *blocks.Block `json:"after,omitempty"`
	Fields  []string      `json:"changed_fields,omitempty"`
}

// RefDiff represents a reference change.
type RefDiff struct {
	Operation string      `json:"operation"` // link, unlink
	Ref       *blocks.Ref `json:"ref"`
}

// AttrDiff represents an attribute change.
type AttrDiff struct {
	Operation string       `json:"operation"` // set, delete
	Attr      *blocks.Attr `json:"attr"`
	Before    *string      `json:"before,omitempty"`
}

// Conflict represents a merge conflict.
type Conflict struct {
	BlockID       string        `json:"block_id"`
	Type          ConflictType  `json:"type"`
	SessionValue  *blocks.Block `json:"session_value,omitempty"`
	ContentValue  *blocks.Block `json:"content_value,omitempty"`
	AncestorValue *blocks.Block `json:"ancestor_value,omitempty"`
	Message       string        `json:"message"`
}

// ConflictType represents the type of conflict.
type ConflictType string

const (
	ConflictContent    ConflictType = "content"    // content hash mismatch
	ConflictStructure  ConflictType = "structure"  // parent/ref doesn't exist
	ConflictDeleted    ConflictType = "deleted"    // block was deleted in content.db
	ConflictPermission ConflictType = "permission" // user lacks permission
)

// Resolution represents a conflict resolution choice.
type Resolution struct {
	BlockID string         `json:"block_id"`
	Choice  ResolutionType `json:"choice"`
	Merged  *blocks.Block  `json:"merged,omitempty"` // for manual merge
}

// ResolutionType represents how to resolve a conflict.
type ResolutionType string

const (
	ResolutionKeepSession ResolutionType = "keep_session"
	ResolutionKeepContent ResolutionType = "keep_content"
	ResolutionManual      ResolutionType = "manual"
)

// BatchOp represents a batch operation.
type BatchOp struct {
	Operation string                 `json:"op"`       // insert, update, delete, link, unlink
	Block     *blocks.Block          `json:"block,omitempty"`
	BlockID   string                 `json:"block_id,omitempty"`
	Changes   map[string]interface{} `json:"changes,omitempty"`
	Ref       *blocks.Ref            `json:"ref,omitempty"`
}

// BatchResult represents the result of a batch operation.
type BatchResult struct {
	Success bool     `json:"success"`
	Errors  []string `json:"errors,omitempty"`
	Changes int      `json:"changes_count"`
}

// QueueItem represents a session in the merge queue.
type QueueItem struct {
	SessionID  string    `json:"session_id"`
	UserID     string    `json:"user_id"`
	SubmittedAt time.Time `json:"submitted_at"`
	FilePath   string    `json:"file_path"`
	Status     string    `json:"status"` // pending, processing, done, failed
	Error      string    `json:"error,omitempty"`
}
