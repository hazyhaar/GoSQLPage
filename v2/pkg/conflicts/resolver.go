// Package conflicts provides conflict detection and resolution for GoSQLPage v2.1.
package conflicts

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/hazyhaar/gopage/v2/pkg/blocks"
	"github.com/hazyhaar/gopage/v2/pkg/session"
	_ "modernc.org/sqlite"
)

// Resolver handles conflict detection and resolution.
type Resolver struct {
	contentDB *sql.DB
	logger    *slog.Logger
}

// ResolverConfig holds resolver configuration.
type ResolverConfig struct {
	ContentDBPath string
	Logger        *slog.Logger
}

// NewResolver creates a new conflict resolver.
func NewResolver(cfg ResolverConfig) (*Resolver, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	db, err := sql.Open("sqlite", cfg.ContentDBPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open content.db: %w", err)
	}

	return &Resolver{
		contentDB: db,
		logger:    cfg.Logger,
	}, nil
}

// ConflictDetail provides detailed information about a conflict.
type ConflictDetail struct {
	BlockID       string              `json:"block_id"`
	Type          session.ConflictType `json:"type"`
	Message       string              `json:"message"`
	SessionBlock  *blocks.Block       `json:"session_block,omitempty"`
	ContentBlock  *blocks.Block       `json:"content_block,omitempty"`
	AncestorBlock *blocks.Block       `json:"ancestor_block,omitempty"`
	FieldDiffs    []FieldDiff         `json:"field_diffs,omitempty"`
	Suggestions   []ResolutionOption  `json:"suggestions,omitempty"`
}

// FieldDiff represents a difference in a specific field.
type FieldDiff struct {
	Field         string `json:"field"`
	SessionValue  string `json:"session_value"`
	ContentValue  string `json:"content_value"`
	AncestorValue string `json:"ancestor_value,omitempty"`
}

// ResolutionOption represents a possible resolution.
type ResolutionOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Preview     string `json:"preview,omitempty"`
}

// DetectConflicts detects conflicts for a session.
func (r *Resolver) DetectConflicts(ctx context.Context, sessionDBPath string) ([]*ConflictDetail, error) {
	sessionDB, err := sql.Open("sqlite", sessionDBPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open session db: %w", err)
	}
	defer sessionDB.Close()

	var conflicts []*ConflictDetail

	// Get structural dependencies and check for hash mismatches
	rows, err := sessionDB.QueryContext(ctx, `
		SELECT block_id, depends_on, snapshot_hashes FROM _structural_deps`)
	if err != nil {
		return nil, fmt.Errorf("query deps: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var blockID, depsJSON, hashesJSON string
		if err := rows.Scan(&blockID, &depsJSON, &hashesJSON); err != nil {
			continue
		}

		var hashes map[string]string
		if err := json.Unmarshal([]byte(hashesJSON), &hashes); err != nil {
			continue
		}

		for depID, expectedHash := range hashes {
			conflict := r.checkBlockConflict(ctx, sessionDB, depID, expectedHash)
			if conflict != nil {
				conflicts = append(conflicts, conflict)
			}
		}
	}

	// Check dirty blocks for structural conflicts
	dirtyRows, err := sessionDB.QueryContext(ctx, `
		SELECT id, parent_id, type, content, position, hash FROM blocks WHERE _dirty = 1`)
	if err != nil {
		return nil, fmt.Errorf("query dirty blocks: %w", err)
	}
	defer dirtyRows.Close()

	for dirtyRows.Next() {
		var sessionBlock blocks.Block
		var parentID sql.NullString

		if err := dirtyRows.Scan(&sessionBlock.ID, &parentID, &sessionBlock.Type,
			&sessionBlock.Content, &sessionBlock.Position, &sessionBlock.Hash); err != nil {
			continue
		}

		if parentID.Valid {
			sessionBlock.ParentID = &parentID.String
			// Check if parent exists
			conflict := r.checkParentConflict(ctx, sessionDB, &sessionBlock)
			if conflict != nil {
				conflicts = append(conflicts, conflict)
			}
		}
	}

	return conflicts, nil
}

// checkBlockConflict checks if a block has conflicts.
func (r *Resolver) checkBlockConflict(ctx context.Context, sessionDB *sql.DB, blockID, expectedHash string) *ConflictDetail {
	// Get current state from content.db
	var contentBlock blocks.Block
	var parentID, contentHTML sql.NullString

	row := r.contentDB.QueryRowContext(ctx, `
		SELECT id, parent_id, type, content, content_html, position, hash,
		       created_at, updated_at, created_by, published, deleted_at
		FROM blocks WHERE id = ?`, blockID)

	var deletedAt sql.NullString
	err := row.Scan(&contentBlock.ID, &parentID, &contentBlock.Type, &contentBlock.Content,
		&contentHTML, &contentBlock.Position, &contentBlock.Hash,
		&contentBlock.CreatedAt, &contentBlock.UpdatedAt, &contentBlock.CreatedBy,
		&contentBlock.Published, &deletedAt)

	if err == sql.ErrNoRows {
		// Block was deleted
		return &ConflictDetail{
			BlockID: blockID,
			Type:    session.ConflictDeleted,
			Message: "Le bloc a été supprimé dans content.db",
			Suggestions: []ResolutionOption{
				{ID: "recreate", Label: "Recréer le bloc", Description: "Créer le bloc à nouveau"},
				{ID: "discard", Label: "Abandonner", Description: "Abandonner les modifications"},
			},
		}
	}

	if err != nil {
		return nil
	}

	if parentID.Valid {
		contentBlock.ParentID = &parentID.String
	}
	if contentHTML.Valid {
		contentBlock.ContentHTML = contentHTML.String
	}

	// Check hash mismatch
	if contentBlock.Hash != expectedHash {
		// Get session version
		sessionBlock := r.getSessionBlock(ctx, sessionDB, blockID)

		conflict := &ConflictDetail{
			BlockID:      blockID,
			Type:         session.ConflictContent,
			Message:      "Le contenu a été modifié par un autre utilisateur",
			ContentBlock: &contentBlock,
			SessionBlock: sessionBlock,
			FieldDiffs:   r.computeFieldDiffs(sessionBlock, &contentBlock),
			Suggestions: []ResolutionOption{
				{ID: "keep_session", Label: "Garder ma version", Description: "Utiliser votre version du contenu"},
				{ID: "keep_content", Label: "Garder la version actuelle", Description: "Utiliser la version de content.db"},
				{ID: "merge", Label: "Fusionner", Description: "Combiner les deux versions manuellement"},
			},
		}

		// Add preview for each option
		if sessionBlock != nil {
			conflict.Suggestions[0].Preview = sessionBlock.Content
		}
		conflict.Suggestions[1].Preview = contentBlock.Content

		return conflict
	}

	return nil
}

// checkParentConflict checks if a block's parent still exists.
func (r *Resolver) checkParentConflict(ctx context.Context, sessionDB *sql.DB, block *blocks.Block) *ConflictDetail {
	if block.ParentID == nil {
		return nil
	}

	// Check if parent exists in content.db
	var exists int
	row := r.contentDB.QueryRowContext(ctx, `
		SELECT 1 FROM blocks WHERE id = ? AND deleted_at IS NULL`, *block.ParentID)
	if err := row.Scan(&exists); err == nil {
		return nil // Parent exists
	}

	// Check if parent is being created in session
	row = sessionDB.QueryRowContext(ctx, `
		SELECT 1 FROM blocks WHERE id = ? AND _source = 'new'`, *block.ParentID)
	if err := row.Scan(&exists); err == nil {
		return nil // Parent is being created
	}

	return &ConflictDetail{
		BlockID: block.ID,
		Type:    session.ConflictStructure,
		Message: fmt.Sprintf("Le bloc parent '%s' n'existe plus", *block.ParentID),
		Suggestions: []ResolutionOption{
			{ID: "new_parent", Label: "Choisir un nouveau parent", Description: "Sélectionner un autre bloc parent"},
			{ID: "make_root", Label: "Rendre racine", Description: "Convertir en bloc racine (sans parent)"},
			{ID: "discard", Label: "Abandonner", Description: "Abandonner les modifications"},
		},
	}
}

// getSessionBlock retrieves a block from the session database.
func (r *Resolver) getSessionBlock(ctx context.Context, sessionDB *sql.DB, blockID string) *blocks.Block {
	var block blocks.Block
	var parentID, contentHTML sql.NullString

	row := sessionDB.QueryRowContext(ctx, `
		SELECT id, parent_id, type, content, content_html, position, hash,
		       created_at, updated_at, created_by, published
		FROM blocks WHERE id = ?`, blockID)

	err := row.Scan(&block.ID, &parentID, &block.Type, &block.Content, &contentHTML,
		&block.Position, &block.Hash, &block.CreatedAt, &block.UpdatedAt,
		&block.CreatedBy, &block.Published)
	if err != nil {
		return nil
	}

	if parentID.Valid {
		block.ParentID = &parentID.String
	}
	if contentHTML.Valid {
		block.ContentHTML = contentHTML.String
	}

	return &block
}

// computeFieldDiffs computes field-level differences between two blocks.
func (r *Resolver) computeFieldDiffs(session, content *blocks.Block) []FieldDiff {
	if session == nil || content == nil {
		return nil
	}

	var diffs []FieldDiff

	if session.Content != content.Content {
		diffs = append(diffs, FieldDiff{
			Field:        "content",
			SessionValue: session.Content,
			ContentValue: content.Content,
		})
	}

	if session.Position != content.Position {
		diffs = append(diffs, FieldDiff{
			Field:        "position",
			SessionValue: session.Position,
			ContentValue: content.Position,
		})
	}

	if session.Type != content.Type {
		diffs = append(diffs, FieldDiff{
			Field:        "type",
			SessionValue: session.Type,
			ContentValue: content.Type,
		})
	}

	if session.Published != content.Published {
		diffs = append(diffs, FieldDiff{
			Field:        "published",
			SessionValue: fmt.Sprintf("%v", session.Published),
			ContentValue: fmt.Sprintf("%v", content.Published),
		})
	}

	return diffs
}

// Resolution represents a user's resolution choice.
type Resolution struct {
	BlockID      string        `json:"block_id"`
	Choice       string        `json:"choice"` // keep_session, keep_content, merge, recreate, new_parent, make_root, discard
	MergedBlock  *blocks.Block `json:"merged_block,omitempty"`
	NewParentID  *string       `json:"new_parent_id,omitempty"`
}

// ApplyResolution applies a resolution to a session.
func (r *Resolver) ApplyResolution(ctx context.Context, sessionDBPath string, resolution *Resolution) error {
	sessionDB, err := sql.Open("sqlite", sessionDBPath)
	if err != nil {
		return fmt.Errorf("open session db: %w", err)
	}
	defer sessionDB.Close()

	switch resolution.Choice {
	case "keep_session":
		// Keep session version - update snapshot hash to current content.db hash
		return r.updateSnapshotHash(ctx, sessionDB, resolution.BlockID)

	case "keep_content":
		// Replace session block with content.db version
		return r.replaceWithContentVersion(ctx, sessionDB, resolution.BlockID)

	case "merge":
		// Apply merged block
		if resolution.MergedBlock == nil {
			return fmt.Errorf("merged_block required for merge resolution")
		}
		return r.applyMergedBlock(ctx, sessionDB, resolution.MergedBlock)

	case "recreate":
		// Mark block as new (will be re-inserted)
		return r.markAsNew(ctx, sessionDB, resolution.BlockID)

	case "new_parent":
		// Update parent ID
		if resolution.NewParentID == nil {
			return fmt.Errorf("new_parent_id required")
		}
		return r.updateParent(ctx, sessionDB, resolution.BlockID, *resolution.NewParentID)

	case "make_root":
		// Remove parent
		return r.makeRoot(ctx, sessionDB, resolution.BlockID)

	case "discard":
		// Remove block from session
		return r.discardBlock(ctx, sessionDB, resolution.BlockID)

	default:
		return fmt.Errorf("unknown resolution choice: %s", resolution.Choice)
	}
}

func (r *Resolver) updateSnapshotHash(ctx context.Context, sessionDB *sql.DB, blockID string) error {
	// Get current hash from content.db
	var currentHash string
	row := r.contentDB.QueryRowContext(ctx, "SELECT hash FROM blocks WHERE id = ?", blockID)
	if err := row.Scan(&currentHash); err != nil {
		return err
	}

	// Update snapshot hash in session
	_, err := sessionDB.ExecContext(ctx, `
		UPDATE _structural_deps
		SET snapshot_hashes = json_set(snapshot_hashes, '$.' || ?, ?)
		WHERE block_id = ?`, blockID, currentHash, blockID)
	return err
}

func (r *Resolver) replaceWithContentVersion(ctx context.Context, sessionDB *sql.DB, blockID string) error {
	// Get block from content.db
	var parentID, contentHTML sql.NullString
	var content, blockType, position, hash, createdBy string
	var createdAt, updatedAt time.Time
	var published bool

	row := r.contentDB.QueryRowContext(ctx, `
		SELECT parent_id, type, content, content_html, position, hash,
		       created_at, updated_at, created_by, published
		FROM blocks WHERE id = ?`, blockID)

	err := row.Scan(&parentID, &blockType, &content, &contentHTML, &position, &hash,
		&createdAt, &updatedAt, &createdBy, &published)
	if err != nil {
		return err
	}

	// Update session block
	_, err = sessionDB.ExecContext(ctx, `
		UPDATE blocks SET
			parent_id = ?, type = ?, content = ?, content_html = ?,
			position = ?, hash = ?, updated_at = ?, _dirty = 0
		WHERE id = ?`,
		parentID, blockType, content, contentHTML, position, hash,
		time.Now().UTC(), blockID)
	return err
}

func (r *Resolver) applyMergedBlock(ctx context.Context, sessionDB *sql.DB, block *blocks.Block) error {
	block.UpdateHash()
	block.UpdatedAt = time.Now().UTC()

	_, err := sessionDB.ExecContext(ctx, `
		UPDATE blocks SET
			content = ?, content_html = ?, position = ?, hash = ?,
			updated_at = ?, _dirty = 1
		WHERE id = ?`,
		block.Content, block.ContentHTML, block.Position, block.Hash,
		block.UpdatedAt, block.ID)
	return err
}

func (r *Resolver) markAsNew(ctx context.Context, sessionDB *sql.DB, blockID string) error {
	_, err := sessionDB.ExecContext(ctx, `
		UPDATE blocks SET _source = 'new', _dirty = 1 WHERE id = ?`, blockID)
	return err
}

func (r *Resolver) updateParent(ctx context.Context, sessionDB *sql.DB, blockID, newParentID string) error {
	_, err := sessionDB.ExecContext(ctx, `
		UPDATE blocks SET parent_id = ?, _dirty = 1, updated_at = ?
		WHERE id = ?`, newParentID, time.Now().UTC(), blockID)
	return err
}

func (r *Resolver) makeRoot(ctx context.Context, sessionDB *sql.DB, blockID string) error {
	_, err := sessionDB.ExecContext(ctx, `
		UPDATE blocks SET parent_id = NULL, _dirty = 1, updated_at = ?
		WHERE id = ?`, time.Now().UTC(), blockID)
	return err
}

func (r *Resolver) discardBlock(ctx context.Context, sessionDB *sql.DB, blockID string) error {
	// Remove from blocks table
	_, err := sessionDB.ExecContext(ctx, `DELETE FROM blocks WHERE id = ?`, blockID)
	if err != nil {
		return err
	}

	// Remove from changes
	_, err = sessionDB.ExecContext(ctx, `DELETE FROM _changes WHERE block_id = ?`, blockID)
	if err != nil {
		return err
	}

	// Remove from structural deps
	_, err = sessionDB.ExecContext(ctx, `DELETE FROM _structural_deps WHERE block_id = ?`, blockID)
	return err
}

// Close closes the resolver.
func (r *Resolver) Close() error {
	return r.contentDB.Close()
}
