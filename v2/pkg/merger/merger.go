// Package merger provides the singleton merger daemon for GoSQLPage v2.1.
// The merger is the only component that writes to content.db, ensuring
// atomicity and conflict detection.
package merger

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hazyhaar/gopage/v2/pkg/blocks"
	"github.com/hazyhaar/gopage/v2/pkg/session"
	_ "modernc.org/sqlite"
)

// Config holds merger configuration.
type Config struct {
	ContentDBPath    string
	SchemaDBPath     string
	AuditDBPath      string
	PendingDir       string
	ProcessingDir    string
	DoneDir          string
	FailedDir        string
	PollIntervalMS   int
	MaxRetries       int
	LockTimeoutMS    int
	RecoverOnStartup bool
	Logger           *slog.Logger
}

// Merger is the singleton daemon that merges sessions into content.db.
type Merger struct {
	cfg       Config
	contentDB *sql.DB
	schemaDB  *sql.DB
	auditDB   *sql.DB
	running   bool
	mu        sync.Mutex
	stopCh    chan struct{}
	logger    *slog.Logger

	// Metrics
	mergesTotal    int64
	mergesSuccess  int64
	mergesFailed   int64
	mergesConflict int64
}

// New creates a new merger daemon.
func New(cfg Config) (*Merger, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.PollIntervalMS <= 0 {
		cfg.PollIntervalMS = 500
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.LockTimeoutMS <= 0 {
		cfg.LockTimeoutMS = 30000
	}

	// Ensure directories exist
	for _, dir := range []string{cfg.PendingDir, cfg.ProcessingDir, cfg.DoneDir, cfg.FailedDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create dir %s: %w", dir, err)
		}
	}

	// Open content.db for writing
	contentDB, err := sql.Open("sqlite", cfg.ContentDBPath)
	if err != nil {
		return nil, fmt.Errorf("open content.db: %w", err)
	}

	// Enable WAL mode
	if _, err := contentDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		contentDB.Close()
		return nil, fmt.Errorf("enable WAL: %w", err)
	}

	// Open schema.db for validation
	schemaDB, err := sql.Open("sqlite", cfg.SchemaDBPath+"?mode=ro")
	if err != nil {
		contentDB.Close()
		return nil, fmt.Errorf("open schema.db: %w", err)
	}

	// Open audit.db for logging
	auditDB, err := sql.Open("sqlite", cfg.AuditDBPath)
	if err != nil {
		contentDB.Close()
		schemaDB.Close()
		return nil, fmt.Errorf("open audit.db: %w", err)
	}

	m := &Merger{
		cfg:       cfg,
		contentDB: contentDB,
		schemaDB:  schemaDB,
		auditDB:   auditDB,
		stopCh:    make(chan struct{}),
		logger:    cfg.Logger,
	}

	// Recover stuck sessions on startup
	if cfg.RecoverOnStartup {
		if err := m.recoverProcessing(); err != nil {
			cfg.Logger.Warn("recovery failed", "error", err)
		}
	}

	return m, nil
}

// recoverProcessing handles sessions stuck in the processing directory.
func (m *Merger) recoverProcessing() error {
	entries, err := os.ReadDir(m.cfg.ProcessingDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".db" {
			continue
		}

		src := filepath.Join(m.cfg.ProcessingDir, entry.Name())
		dst := filepath.Join(m.cfg.FailedDir, entry.Name())

		m.logger.Warn("recovering stuck session", "file", entry.Name())
		if err := os.Rename(src, dst); err != nil {
			m.logger.Error("failed to recover session", "file", entry.Name(), "error", err)
		}
	}

	return nil
}

// Start starts the merger daemon.
func (m *Merger) Start(ctx context.Context) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	m.logger.Info("merger started")

	ticker := time.NewTicker(time.Duration(m.cfg.PollIntervalMS) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.Stop()
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.processPending(ctx)
		}
	}
}

// Stop stops the merger daemon.
func (m *Merger) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	close(m.stopCh)
	m.running = false
	m.logger.Info("merger stopped")
}

// processPending processes all pending sessions in FIFO order.
func (m *Merger) processPending(ctx context.Context) {
	entries, err := os.ReadDir(m.cfg.PendingDir)
	if err != nil {
		m.logger.Error("read pending dir", "error", err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".db" {
			continue
		}

		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := m.processSession(ctx, entry.Name()); err != nil {
			m.logger.Error("process session failed", "file", entry.Name(), "error", err)
		}
	}
}

// processSession processes a single session.
func (m *Merger) processSession(ctx context.Context, filename string) error {
	m.mergesTotal++
	startTime := time.Now()

	pendingPath := filepath.Join(m.cfg.PendingDir, filename)
	processingPath := filepath.Join(m.cfg.ProcessingDir, filename)

	// Move to processing
	if err := os.Rename(pendingPath, processingPath); err != nil {
		return fmt.Errorf("move to processing: %w", err)
	}

	// Open session database
	sessionDB, err := sql.Open("sqlite", processingPath)
	if err != nil {
		m.moveToFailed(processingPath, "failed to open session db")
		return fmt.Errorf("open session db: %w", err)
	}
	defer sessionDB.Close()

	// Load session metadata
	meta, err := m.loadSessionMeta(sessionDB)
	if err != nil {
		m.moveToFailed(processingPath, "failed to load session metadata")
		return fmt.Errorf("load session meta: %w", err)
	}

	// Validate session
	conflicts, err := m.validateSession(ctx, sessionDB, meta)
	if err != nil {
		m.moveToFailed(processingPath, err.Error())
		m.mergesFailed++
		return fmt.Errorf("validate session: %w", err)
	}

	if len(conflicts) > 0 {
		// Mark as conflict and move to failed
		m.markConflict(sessionDB, meta.SessionID, conflicts)
		m.moveToFailed(processingPath, "conflicts detected")
		m.mergesConflict++
		return fmt.Errorf("conflicts detected: %d", len(conflicts))
	}

	// Apply changes
	result, err := m.applyChanges(ctx, sessionDB, meta)
	if err != nil {
		m.moveToFailed(processingPath, err.Error())
		m.mergesFailed++
		return fmt.Errorf("apply changes: %w", err)
	}

	// Log to audit
	duration := time.Since(startTime)
	m.logMerge(meta, result, duration)

	// Move to done
	donePath := filepath.Join(m.cfg.DoneDir, filename)
	if err := os.Rename(processingPath, donePath); err != nil {
		m.logger.Error("move to done failed", "error", err)
	}

	m.mergesSuccess++
	m.logger.Info("merge completed",
		"session_id", meta.SessionID,
		"inserted", result.Inserted,
		"updated", result.Updated,
		"deleted", result.Deleted,
		"duration_ms", duration.Milliseconds())

	return nil
}

// loadSessionMeta loads session metadata from the session database.
func (m *Merger) loadSessionMeta(db *sql.DB) (*session.SessionMeta, error) {
	var meta session.SessionMeta
	row := db.QueryRow(`SELECT session_id, user_id, user_type, created_at, last_activity,
		base_snapshot, schema_version, schema_hash, status FROM _session_meta LIMIT 1`)
	err := row.Scan(&meta.SessionID, &meta.UserID, &meta.UserType, &meta.CreatedAt,
		&meta.LastActivity, &meta.BaseSnapshot, &meta.SchemaVersion, &meta.SchemaHash, &meta.Status)
	return &meta, err
}

// validateSession validates a session before merging.
func (m *Merger) validateSession(ctx context.Context, sessionDB *sql.DB, meta *session.SessionMeta) ([]*session.Conflict, error) {
	var conflicts []*session.Conflict

	// 1. Validate schema version
	var currentVersion int
	row := m.schemaDB.QueryRow("SELECT version FROM schema_version WHERE id = 1")
	if err := row.Scan(&currentVersion); err != nil {
		return nil, fmt.Errorf("get schema version: %w", err)
	}

	if meta.SchemaVersion > currentVersion {
		return nil, fmt.Errorf("session schema version %d is newer than current %d",
			meta.SchemaVersion, currentVersion)
	}

	// 2. Check for content conflicts (hash mismatches)
	rows, err := sessionDB.QueryContext(ctx, `
		SELECT block_id, snapshot_hashes FROM _structural_deps`)
	if err != nil {
		return nil, fmt.Errorf("query structural deps: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var blockID, hashesJSON string
		if err := rows.Scan(&blockID, &hashesJSON); err != nil {
			continue
		}

		var hashes map[string]string
		if err := json.Unmarshal([]byte(hashesJSON), &hashes); err != nil {
			continue
		}

		// Check if content.db hash matches snapshot
		for depID, expectedHash := range hashes {
			var currentHash string
			row := m.contentDB.QueryRow("SELECT hash FROM blocks WHERE id = ?", depID)
			if err := row.Scan(&currentHash); err != nil {
				// Block was deleted in content.db
				conflicts = append(conflicts, &session.Conflict{
					BlockID: depID,
					Type:    session.ConflictDeleted,
					Message: "block was deleted in content.db",
				})
				continue
			}

			if currentHash != expectedHash {
				conflicts = append(conflicts, &session.Conflict{
					BlockID: depID,
					Type:    session.ConflictContent,
					Message: fmt.Sprintf("hash mismatch: expected %s, got %s", expectedHash, currentHash),
				})
			}
		}
	}

	// 3. Check structural integrity (parent exists, refs exist)
	dirtyBlocks, err := sessionDB.QueryContext(ctx, `
		SELECT id, parent_id FROM blocks WHERE _dirty = 1`)
	if err != nil {
		return nil, fmt.Errorf("query dirty blocks: %w", err)
	}
	defer dirtyBlocks.Close()

	for dirtyBlocks.Next() {
		var blockID string
		var parentID sql.NullString
		if err := dirtyBlocks.Scan(&blockID, &parentID); err != nil {
			continue
		}

		if parentID.Valid {
			// Check parent exists in content.db or session
			var exists int
			row := m.contentDB.QueryRow("SELECT 1 FROM blocks WHERE id = ?", parentID.String)
			if err := row.Scan(&exists); err != nil {
				// Check if parent is being created in this session
				row = sessionDB.QueryRow("SELECT 1 FROM blocks WHERE id = ? AND _source = 'new'", parentID.String)
				if err := row.Scan(&exists); err != nil {
					conflicts = append(conflicts, &session.Conflict{
						BlockID: blockID,
						Type:    session.ConflictStructure,
						Message: fmt.Sprintf("parent %s does not exist", parentID.String),
					})
				}
			}
		}
	}

	return conflicts, nil
}

// MergeResult contains the result of a merge operation.
type MergeResult struct {
	Inserted int
	Updated  int
	Deleted  int
}

// applyChanges applies session changes to content.db.
func (m *Merger) applyChanges(ctx context.Context, sessionDB *sql.DB, meta *session.SessionMeta) (*MergeResult, error) {
	result := &MergeResult{}

	tx, err := m.contentDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Process changes in order
	rows, err := sessionDB.QueryContext(ctx, `
		SELECT operation, block_id, before, after FROM _changes WHERE merged = 0 ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query changes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var op, blockID string
		var before, after sql.NullString

		if err := rows.Scan(&op, &blockID, &before, &after); err != nil {
			continue
		}

		switch op {
		case blocks.OpInsert:
			if err := m.applyInsert(ctx, tx, sessionDB, blockID); err != nil {
				return nil, fmt.Errorf("apply insert %s: %w", blockID, err)
			}
			result.Inserted++

		case blocks.OpUpdate:
			if err := m.applyUpdate(ctx, tx, sessionDB, blockID); err != nil {
				return nil, fmt.Errorf("apply update %s: %w", blockID, err)
			}
			result.Updated++

		case blocks.OpDelete:
			if err := m.applyDelete(ctx, tx, blockID); err != nil {
				return nil, fmt.Errorf("apply delete %s: %w", blockID, err)
			}
			result.Deleted++

		case blocks.OpLink:
			if after.Valid {
				if err := m.applyLink(ctx, tx, after.String); err != nil {
					return nil, fmt.Errorf("apply link: %w", err)
				}
			}

		case blocks.OpUnlink:
			if before.Valid {
				if err := m.applyUnlink(ctx, tx, before.String); err != nil {
					return nil, fmt.Errorf("apply unlink: %w", err)
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	// Mark changes as merged
	sessionDB.ExecContext(ctx, "UPDATE _changes SET merged = 1")
	sessionDB.ExecContext(ctx, "UPDATE _session_meta SET status = 'merged'")

	return result, nil
}

// applyInsert inserts a new block from session to content.db.
func (m *Merger) applyInsert(ctx context.Context, tx *sql.Tx, sessionDB *sql.DB, blockID string) error {
	row := sessionDB.QueryRowContext(ctx, `
		SELECT id, parent_id, type, content, content_html, position, hash,
		       created_at, updated_at, created_by, published
		FROM blocks WHERE id = ?`, blockID)

	var id, blockType, content, position, hash, createdBy string
	var parentID, contentHTML sql.NullString
	var createdAt, updatedAt string
	var published int

	err := row.Scan(&id, &parentID, &blockType, &content, &contentHTML, &position, &hash,
		&createdAt, &updatedAt, &createdBy, &published)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO blocks (id, parent_id, type, content, content_html, position, hash,
		                   created_at, updated_at, created_by, published)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, parentID, blockType, content, contentHTML, position, hash,
		createdAt, updatedAt, createdBy, published)

	return err
}

// applyUpdate updates an existing block in content.db.
func (m *Merger) applyUpdate(ctx context.Context, tx *sql.Tx, sessionDB *sql.DB, blockID string) error {
	row := sessionDB.QueryRowContext(ctx, `
		SELECT content, content_html, position, hash, updated_at, published
		FROM blocks WHERE id = ?`, blockID)

	var content, position, hash, updatedAt string
	var contentHTML sql.NullString
	var published int

	err := row.Scan(&content, &contentHTML, &position, &hash, &updatedAt, &published)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE blocks SET content = ?, content_html = ?, position = ?, hash = ?,
		                  updated_at = ?, published = ?
		WHERE id = ?`,
		content, contentHTML, position, hash, updatedAt, published, blockID)

	return err
}

// applyDelete soft-deletes a block in content.db.
func (m *Merger) applyDelete(ctx context.Context, tx *sql.Tx, blockID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := tx.ExecContext(ctx, `
		UPDATE blocks SET deleted_at = ?, updated_at = ? WHERE id = ?`,
		now, now, blockID)
	return err
}

// applyLink creates a reference in content.db.
func (m *Merger) applyLink(ctx context.Context, tx *sql.Tx, refJSON string) error {
	var ref blocks.Ref
	if err := json.Unmarshal([]byte(refJSON), &ref); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO refs (from_id, to_id, type, anchor, created_at, created_by)
		VALUES (?, ?, ?, ?, ?, ?)`,
		ref.FromID, ref.ToID, ref.Type, ref.Anchor, ref.CreatedAt, ref.CreatedBy)

	return err
}

// applyUnlink removes a reference from content.db.
func (m *Merger) applyUnlink(ctx context.Context, tx *sql.Tx, refJSON string) error {
	var ref blocks.Ref
	if err := json.Unmarshal([]byte(refJSON), &ref); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `
		DELETE FROM refs WHERE from_id = ? AND to_id = ? AND type = ?`,
		ref.FromID, ref.ToID, ref.Type)

	return err
}

// markConflict updates the session status to conflict.
func (m *Merger) markConflict(db *sql.DB, sessionID string, conflicts []*session.Conflict) {
	conflictsJSON, _ := json.Marshal(conflicts)
	db.Exec(`UPDATE _session_meta SET status = 'conflict'`)
	m.logger.Warn("session has conflicts", "session_id", sessionID, "conflicts", string(conflictsJSON))
}

// moveToFailed moves a session to the failed directory.
func (m *Merger) moveToFailed(path, reason string) {
	filename := filepath.Base(path)
	dst := filepath.Join(m.cfg.FailedDir, filename)
	if err := os.Rename(path, dst); err != nil {
		m.logger.Error("move to failed", "path", path, "error", err)
	}
	m.logger.Warn("session moved to failed", "file", filename, "reason", reason)
}

// logMerge logs a merge operation to audit.db.
func (m *Merger) logMerge(meta *session.SessionMeta, result *MergeResult, duration time.Duration) {
	_, err := m.auditDB.Exec(`
		INSERT INTO merge_log (session_id, user_id, status, blocks_inserted, blocks_updated, blocks_deleted, duration_ms)
		VALUES (?, ?, 'success', ?, ?, ?, ?)`,
		meta.SessionID, meta.UserID, result.Inserted, result.Updated, result.Deleted, duration.Milliseconds())
	if err != nil {
		m.logger.Error("log merge failed", "error", err)
	}
}

// Close closes the merger.
func (m *Merger) Close() error {
	m.Stop()
	var errs []error
	if err := m.contentDB.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := m.schemaDB.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := m.auditDB.Close(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}

// Stats returns merger statistics.
func (m *Merger) Stats() map[string]interface{} {
	return map[string]interface{}{
		"merges_total":    m.mergesTotal,
		"merges_success":  m.mergesSuccess,
		"merges_failed":   m.mergesFailed,
		"merges_conflict": m.mergesConflict,
		"running":         m.running,
	}
}

// Health returns the health status of the merger.
type Health struct {
	Status          string `json:"status"`
	QueuePending    int    `json:"queue_pending"`
	QueueProcessing int    `json:"queue_processing"`
	QueueFailed     int    `json:"queue_failed"`
	LastMergeAt     string `json:"last_merge_at,omitempty"`
	MergesLastHour  int64  `json:"merges_last_hour"`
}

// GetHealth returns the current health status.
func (m *Merger) GetHealth() *Health {
	h := &Health{
		Status: "ok",
	}

	if !m.running {
		h.Status = "stopped"
	}

	// Count queue items
	if entries, err := os.ReadDir(m.cfg.PendingDir); err == nil {
		h.QueuePending = len(entries)
	}
	if entries, err := os.ReadDir(m.cfg.ProcessingDir); err == nil {
		h.QueueProcessing = len(entries)
	}
	if entries, err := os.ReadDir(m.cfg.FailedDir); err == nil {
		h.QueueFailed = len(entries)
	}

	// Get merges in last hour
	row := m.auditDB.QueryRow(`
		SELECT COUNT(*) FROM merge_log
		WHERE timestamp > datetime('now', '-1 hour')`)
	row.Scan(&h.MergesLastHour)

	return h
}
