// Package api provides HTTP API handlers for GoSQLPage v2.1.
package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/hazyhaar/gopage/v2/pkg/audit"
	"github.com/hazyhaar/gopage/v2/pkg/blocks"
	"github.com/hazyhaar/gopage/v2/pkg/gc"
	"github.com/hazyhaar/gopage/v2/pkg/merger"
	"github.com/hazyhaar/gopage/v2/pkg/session"
	"github.com/hazyhaar/gopage/v2/pkg/users"
	_ "modernc.org/sqlite"
)

// API provides HTTP handlers for the v2.1 API.
type API struct {
	sessionMgr  *session.Manager
	merger      *merger.Merger
	gc          *gc.GC
	auditLogger *audit.Logger
	contentDB   *sql.DB
	schemaDB    *sql.DB
	usersDB     *sql.DB
	logger      *slog.Logger
}

// Config holds API configuration.
type Config struct {
	SessionManager *session.Manager
	Merger         *merger.Merger
	GC             *gc.GC
	AuditLogger    *audit.Logger
	ContentDBPath  string
	SchemaDBPath   string
	UsersDBPath    string
	Logger         *slog.Logger
}

// New creates a new API handler.
func New(cfg Config) (*API, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	contentDB, err := sql.Open("sqlite", cfg.ContentDBPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open content.db: %w", err)
	}

	schemaDB, err := sql.Open("sqlite", cfg.SchemaDBPath+"?mode=ro")
	if err != nil {
		contentDB.Close()
		return nil, fmt.Errorf("open schema.db: %w", err)
	}

	usersDB, err := sql.Open("sqlite", cfg.UsersDBPath)
	if err != nil {
		contentDB.Close()
		schemaDB.Close()
		return nil, fmt.Errorf("open users.db: %w", err)
	}

	return &API{
		sessionMgr:  cfg.SessionManager,
		merger:      cfg.Merger,
		gc:          cfg.GC,
		auditLogger: cfg.AuditLogger,
		contentDB:   contentDB,
		schemaDB:    schemaDB,
		usersDB:     usersDB,
		logger:      cfg.Logger,
	}, nil
}

// Routes returns the API router.
func (a *API) Routes() chi.Router {
	r := chi.NewRouter()

	// Health endpoints
	r.Get("/health", a.healthHandler)
	r.Get("/health/merger", a.mergerHealthHandler)
	r.Get("/health/gc", a.gcHealthHandler)

	// Session endpoints
	r.Route("/session", func(r chi.Router) {
		r.Post("/", a.createSession)
		r.Get("/", a.getCurrentSession)
		r.Delete("/", a.abandonSession)
		r.Post("/blocks", a.addBlockToSession)
		r.Get("/blocks", a.listSessionBlocks)
		r.Get("/blocks/{id}", a.getSessionBlock)
		r.Put("/blocks/{id}", a.updateSessionBlock)
		r.Delete("/blocks/{id}", a.deleteSessionBlock)
		r.Post("/batch", a.batchMutation)
		r.Get("/diff", a.getSessionDiff)
		r.Post("/submit", a.submitSession)
		r.Get("/conflicts", a.getConflicts)
		r.Post("/resolve", a.resolveConflicts)
	})

	// Block endpoints (read from content.db)
	r.Route("/blocks", func(r chi.Router) {
		r.Get("/", a.listBlocks)
		r.Get("/{id}", a.getBlock)
		r.Get("/{id}/children", a.getBlockChildren)
		r.Get("/{id}/refs", a.getBlockRefs)
		r.Get("/{id}/backlinks", a.getBlockBacklinks)
		r.Get("/{id}/history", a.getBlockHistory)
		r.Get("/{id}/tree", a.getBlockTree)
	})

	// Search endpoint
	r.Get("/search", a.searchBlocks)

	// Admin endpoints
	r.Route("/admin", func(r chi.Router) {
		r.Get("/queue", a.getQueueStatus)
		r.Get("/queue/failed", a.getFailedSessions)
		r.Post("/queue/failed/{id}/retry", a.retryFailedSession)
		r.Delete("/queue/failed/{id}", a.deleteFailedSession)
		r.Get("/audit", a.queryAudit)
		r.Get("/schema", a.getSchema)
		r.Post("/backup", a.triggerBackup)
		r.Get("/users", a.listUsers)
		r.Post("/users", a.createUser)
		r.Put("/users/{id}", a.updateUser)
		r.Get("/permissions", a.listPermissions)
		r.Post("/permissions", a.grantPermission)
		r.Delete("/permissions/{id}", a.revokePermission)
	})

	return r
}

// Response helpers

func (a *API) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (a *API) error(w http.ResponseWriter, status int, message string) {
	a.json(w, status, map[string]string{"error": message})
}

// getUserFromRequest extracts user info from the request (placeholder for auth).
func (a *API) getUserFromRequest(r *http.Request) (userID, userType string) {
	// TODO: Implement proper authentication
	// For now, use a default user
	return "admin", "human"
}

// Health handlers

func (a *API) healthHandler(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status": "ok",
		"components": map[string]string{
			"content_db": "ok",
			"schema_db":  "ok",
			"users_db":   "ok",
		},
	}

	// Check database connections
	if err := a.contentDB.Ping(); err != nil {
		health["components"].(map[string]string)["content_db"] = "error"
		health["status"] = "degraded"
	}
	if err := a.schemaDB.Ping(); err != nil {
		health["components"].(map[string]string)["schema_db"] = "error"
		health["status"] = "degraded"
	}
	if err := a.usersDB.Ping(); err != nil {
		health["components"].(map[string]string)["users_db"] = "error"
		health["status"] = "degraded"
	}

	a.json(w, http.StatusOK, health)
}

func (a *API) mergerHealthHandler(w http.ResponseWriter, r *http.Request) {
	if a.merger == nil {
		a.error(w, http.StatusServiceUnavailable, "merger not available")
		return
	}
	a.json(w, http.StatusOK, a.merger.GetHealth())
}

func (a *API) gcHealthHandler(w http.ResponseWriter, r *http.Request) {
	if a.gc == nil {
		a.error(w, http.StatusServiceUnavailable, "gc not available")
		return
	}
	a.json(w, http.StatusOK, a.gc.GetHealth())
}

// Session handlers

func (a *API) createSession(w http.ResponseWriter, r *http.Request) {
	userID, userType := a.getUserFromRequest(r)

	sess, err := a.sessionMgr.Create(r.Context(), userID, userType)
	if err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.json(w, http.StatusCreated, sess)
}

func (a *API) getCurrentSession(w http.ResponseWriter, r *http.Request) {
	userID, _ := a.getUserFromRequest(r)

	sessions := a.sessionMgr.ListSessionsByUser(userID)
	for _, s := range sessions {
		if s.Status == session.StatusActive {
			a.json(w, http.StatusOK, s)
			return
		}
	}

	a.error(w, http.StatusNotFound, "no active session")
}

func (a *API) abandonSession(w http.ResponseWriter, r *http.Request) {
	userID, _ := a.getUserFromRequest(r)

	sessions := a.sessionMgr.ListSessionsByUser(userID)
	for _, s := range sessions {
		if s.Status == session.StatusActive {
			if err := a.sessionMgr.Abandon(r.Context(), s.ID); err != nil {
				a.error(w, http.StatusInternalServerError, err.Error())
				return
			}
			a.json(w, http.StatusOK, map[string]string{"status": "abandoned"})
			return
		}
	}

	a.error(w, http.StatusNotFound, "no active session")
}

func (a *API) addBlockToSession(w http.ResponseWriter, r *http.Request) {
	userID, userType := a.getUserFromRequest(r)

	var req struct {
		BlockID string        `json:"block_id,omitempty"`
		Block   *blocks.Block `json:"block,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	sess, err := a.sessionMgr.GetOrCreate(r.Context(), userID, userType)
	if err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}

	if req.BlockID != "" {
		// Copy existing block from content.db
		block, err := a.sessionMgr.CopyBlock(r.Context(), sess.ID, req.BlockID)
		if err != nil {
			a.error(w, http.StatusInternalServerError, err.Error())
			return
		}
		a.json(w, http.StatusOK, block)
	} else if req.Block != nil {
		// Insert new block
		req.Block.CreatedBy = userID
		if err := a.sessionMgr.InsertBlock(r.Context(), sess.ID, req.Block); err != nil {
			a.error(w, http.StatusInternalServerError, err.Error())
			return
		}
		a.json(w, http.StatusCreated, req.Block)
	} else {
		a.error(w, http.StatusBadRequest, "block_id or block required")
	}
}

func (a *API) listSessionBlocks(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement listing blocks in current session
	a.json(w, http.StatusOK, []interface{}{})
}

func (a *API) getSessionBlock(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement getting a specific block from session
	blockID := chi.URLParam(r, "id")
	a.json(w, http.StatusOK, map[string]string{"id": blockID})
}

func (a *API) updateSessionBlock(w http.ResponseWriter, r *http.Request) {
	userID, userType := a.getUserFromRequest(r)
	blockID := chi.URLParam(r, "id")

	var block blocks.Block
	if err := json.NewDecoder(r.Body).Decode(&block); err != nil {
		a.error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	block.ID = blockID

	sess, err := a.sessionMgr.GetOrCreate(r.Context(), userID, userType)
	if err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := a.sessionMgr.UpdateBlock(r.Context(), sess.ID, &block); err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.json(w, http.StatusOK, &block)
}

func (a *API) deleteSessionBlock(w http.ResponseWriter, r *http.Request) {
	userID, userType := a.getUserFromRequest(r)
	blockID := chi.URLParam(r, "id")

	sess, err := a.sessionMgr.GetOrCreate(r.Context(), userID, userType)
	if err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := a.sessionMgr.DeleteBlock(r.Context(), sess.ID, blockID); err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.json(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) batchMutation(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement batch mutations
	a.json(w, http.StatusOK, map[string]interface{}{"success": true, "changes_count": 0})
}

func (a *API) getSessionDiff(w http.ResponseWriter, r *http.Request) {
	userID, _ := a.getUserFromRequest(r)

	sessions := a.sessionMgr.ListSessionsByUser(userID)
	for _, s := range sessions {
		if s.Status == session.StatusActive {
			diff, err := a.sessionMgr.GetDiff(r.Context(), s.ID)
			if err != nil {
				a.error(w, http.StatusInternalServerError, err.Error())
				return
			}
			a.json(w, http.StatusOK, diff)
			return
		}
	}

	a.error(w, http.StatusNotFound, "no active session")
}

func (a *API) submitSession(w http.ResponseWriter, r *http.Request) {
	userID, _ := a.getUserFromRequest(r)

	sessions := a.sessionMgr.ListSessionsByUser(userID)
	for _, s := range sessions {
		if s.Status == session.StatusActive {
			if err := a.sessionMgr.Submit(r.Context(), s.ID); err != nil {
				a.error(w, http.StatusInternalServerError, err.Error())
				return
			}
			a.json(w, http.StatusOK, map[string]string{"status": "submitted", "session_id": s.ID})
			return
		}
	}

	a.error(w, http.StatusNotFound, "no active session")
}

func (a *API) getConflicts(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement conflict retrieval
	a.json(w, http.StatusOK, []interface{}{})
}

func (a *API) resolveConflicts(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement conflict resolution
	a.json(w, http.StatusOK, map[string]string{"status": "resolved"})
}

// Block handlers (read from content.db)

func (a *API) listBlocks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	blockType := r.URL.Query().Get("type")
	parentID := r.URL.Query().Get("parent_id")

	query := `SELECT id, parent_id, type, content, position, hash, created_at, updated_at, created_by, published
		FROM blocks WHERE deleted_at IS NULL`
	args := []interface{}{}

	if blockType != "" {
		query += " AND type = ?"
		args = append(args, blockType)
	}
	if parentID != "" {
		query += " AND parent_id = ?"
		args = append(args, parentID)
	}

	query += " ORDER BY position LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := a.contentDB.QueryContext(ctx, query, args...)
	if err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var blockList []*blocks.Block
	for rows.Next() {
		var b blocks.Block
		var parentID sql.NullString
		err := rows.Scan(&b.ID, &parentID, &b.Type, &b.Content, &b.Position, &b.Hash,
			&b.CreatedAt, &b.UpdatedAt, &b.CreatedBy, &b.Published)
		if err != nil {
			continue
		}
		if parentID.Valid {
			b.ParentID = &parentID.String
		}
		blockList = append(blockList, &b)
	}

	a.json(w, http.StatusOK, blockList)
}

func (a *API) getBlock(w http.ResponseWriter, r *http.Request) {
	blockID := chi.URLParam(r, "id")

	var b blocks.Block
	var parentID, contentHTML sql.NullString
	row := a.contentDB.QueryRowContext(r.Context(), `
		SELECT id, parent_id, type, content, content_html, position, hash, created_at, updated_at, created_by, published
		FROM blocks WHERE id = ? AND deleted_at IS NULL`, blockID)

	err := row.Scan(&b.ID, &parentID, &b.Type, &b.Content, &contentHTML, &b.Position, &b.Hash,
		&b.CreatedAt, &b.UpdatedAt, &b.CreatedBy, &b.Published)
	if err == sql.ErrNoRows {
		a.error(w, http.StatusNotFound, "block not found")
		return
	}
	if err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}

	if parentID.Valid {
		b.ParentID = &parentID.String
	}
	if contentHTML.Valid {
		b.ContentHTML = contentHTML.String
	}

	a.json(w, http.StatusOK, &b)
}

func (a *API) getBlockChildren(w http.ResponseWriter, r *http.Request) {
	blockID := chi.URLParam(r, "id")

	rows, err := a.contentDB.QueryContext(r.Context(), `
		SELECT id, parent_id, type, content, position, hash, created_at, updated_at, created_by, published
		FROM blocks WHERE parent_id = ? AND deleted_at IS NULL
		ORDER BY position`, blockID)
	if err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var children []*blocks.Block
	for rows.Next() {
		var b blocks.Block
		var parentID sql.NullString
		err := rows.Scan(&b.ID, &parentID, &b.Type, &b.Content, &b.Position, &b.Hash,
			&b.CreatedAt, &b.UpdatedAt, &b.CreatedBy, &b.Published)
		if err != nil {
			continue
		}
		if parentID.Valid {
			b.ParentID = &parentID.String
		}
		children = append(children, &b)
	}

	a.json(w, http.StatusOK, children)
}

func (a *API) getBlockRefs(w http.ResponseWriter, r *http.Request) {
	blockID := chi.URLParam(r, "id")

	rows, err := a.contentDB.QueryContext(r.Context(), `
		SELECT from_id, to_id, type, anchor, created_at, created_by
		FROM refs WHERE from_id = ?`, blockID)
	if err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var refs []*blocks.Ref
	for rows.Next() {
		var ref blocks.Ref
		var anchor sql.NullString
		err := rows.Scan(&ref.FromID, &ref.ToID, &ref.Type, &anchor, &ref.CreatedAt, &ref.CreatedBy)
		if err != nil {
			continue
		}
		if anchor.Valid {
			ref.Anchor = &anchor.String
		}
		refs = append(refs, &ref)
	}

	a.json(w, http.StatusOK, refs)
}

func (a *API) getBlockBacklinks(w http.ResponseWriter, r *http.Request) {
	blockID := chi.URLParam(r, "id")

	rows, err := a.contentDB.QueryContext(r.Context(), `
		SELECT from_id, to_id, type, anchor, created_at, created_by
		FROM refs WHERE to_id = ?`, blockID)
	if err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var refs []*blocks.Ref
	for rows.Next() {
		var ref blocks.Ref
		var anchor sql.NullString
		err := rows.Scan(&ref.FromID, &ref.ToID, &ref.Type, &anchor, &ref.CreatedAt, &ref.CreatedBy)
		if err != nil {
			continue
		}
		if anchor.Valid {
			ref.Anchor = &anchor.String
		}
		refs = append(refs, &ref)
	}

	a.json(w, http.StatusOK, refs)
}

func (a *API) getBlockHistory(w http.ResponseWriter, r *http.Request) {
	blockID := chi.URLParam(r, "id")

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	history, err := a.auditLogger.GetBlockHistory(r.Context(), blockID, limit)
	if err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.json(w, http.StatusOK, history)
}

func (a *API) getBlockTree(w http.ResponseWriter, r *http.Request) {
	blockID := chi.URLParam(r, "id")
	ctx := r.Context()

	maxDepth := 10
	if d := r.URL.Query().Get("max_depth"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
			maxDepth = parsed
		}
	}

	tree, err := a.buildBlockTree(ctx, blockID, maxDepth, 0)
	if err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.json(w, http.StatusOK, tree)
}

func (a *API) buildBlockTree(ctx context.Context, blockID string, maxDepth, currentDepth int) (*blocks.BlockTree, error) {
	if currentDepth >= maxDepth {
		return nil, nil
	}

	var b blocks.Block
	var parentID sql.NullString
	row := a.contentDB.QueryRowContext(ctx, `
		SELECT id, parent_id, type, content, position, hash, created_at, updated_at, created_by, published
		FROM blocks WHERE id = ? AND deleted_at IS NULL`, blockID)

	err := row.Scan(&b.ID, &parentID, &b.Type, &b.Content, &b.Position, &b.Hash,
		&b.CreatedAt, &b.UpdatedAt, &b.CreatedBy, &b.Published)
	if err != nil {
		return nil, err
	}

	if parentID.Valid {
		b.ParentID = &parentID.String
	}

	tree := &blocks.BlockTree{Block: &b}

	// Get children
	rows, err := a.contentDB.QueryContext(ctx, `
		SELECT id FROM blocks WHERE parent_id = ? AND deleted_at IS NULL ORDER BY position`, blockID)
	if err != nil {
		return tree, nil
	}
	defer rows.Close()

	for rows.Next() {
		var childID string
		if err := rows.Scan(&childID); err != nil {
			continue
		}
		childTree, err := a.buildBlockTree(ctx, childID, maxDepth, currentDepth+1)
		if err != nil {
			continue
		}
		if childTree != nil {
			tree.Children = append(tree.Children, childTree)
		}
	}

	return tree, nil
}

// Search handler

func (a *API) searchBlocks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		a.error(w, http.StatusBadRequest, "query parameter 'q' required")
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	rows, err := a.contentDB.QueryContext(r.Context(), `
		SELECT b.id, b.parent_id, b.type, b.content, b.position, b.hash,
		       b.created_at, b.updated_at, b.created_by, b.published
		FROM blocks_fts f
		JOIN blocks b ON f.id = b.id
		WHERE blocks_fts MATCH ? AND b.deleted_at IS NULL
		ORDER BY rank LIMIT ?`, q, limit)
	if err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var results []*blocks.Block
	for rows.Next() {
		var b blocks.Block
		var parentID sql.NullString
		err := rows.Scan(&b.ID, &parentID, &b.Type, &b.Content, &b.Position, &b.Hash,
			&b.CreatedAt, &b.UpdatedAt, &b.CreatedBy, &b.Published)
		if err != nil {
			continue
		}
		if parentID.Valid {
			b.ParentID = &parentID.String
		}
		results = append(results, &b)
	}

	a.json(w, http.StatusOK, results)
}

// Admin handlers

func (a *API) getQueueStatus(w http.ResponseWriter, r *http.Request) {
	if a.merger == nil {
		a.error(w, http.StatusServiceUnavailable, "merger not available")
		return
	}
	a.json(w, http.StatusOK, a.merger.Stats())
}

func (a *API) getFailedSessions(w http.ResponseWriter, r *http.Request) {
	// TODO: List failed sessions
	a.json(w, http.StatusOK, []interface{}{})
}

func (a *API) retryFailedSession(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement retry logic
	a.json(w, http.StatusOK, map[string]string{"status": "retried"})
}

func (a *API) deleteFailedSession(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement delete logic
	a.json(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) queryAudit(w http.ResponseWriter, r *http.Request) {
	query := &audit.Query{}

	if blockID := r.URL.Query().Get("block_id"); blockID != "" {
		query.BlockID = blockID
	}
	if userID := r.URL.Query().Get("user_id"); userID != "" {
		query.UserID = userID
	}
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if parsed, err := strconv.Atoi(limit); err == nil {
			query.Limit = parsed
		}
	}
	if offset := r.URL.Query().Get("offset"); offset != "" {
		if parsed, err := strconv.Atoi(offset); err == nil {
			query.Offset = parsed
		}
	}

	result, err := a.auditLogger.Query(r.Context(), query)
	if err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.json(w, http.StatusOK, result)
}

func (a *API) getSchema(w http.ResponseWriter, r *http.Request) {
	rows, err := a.schemaDB.QueryContext(r.Context(), `
		SELECT name, label, icon, schema, render_template, allowed_children, allowed_parents, category, version
		FROM block_types`)
	if err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var types []*blocks.BlockType
	for rows.Next() {
		var bt blocks.BlockType
		var icon, schema, renderTemplate, allowedChildren, allowedParents sql.NullString
		err := rows.Scan(&bt.Name, &bt.Label, &icon, &schema, &renderTemplate,
			&allowedChildren, &allowedParents, &bt.Category, &bt.Version)
		if err != nil {
			continue
		}
		if icon.Valid {
			bt.Icon = icon.String
		}
		if schema.Valid {
			bt.Schema = schema.String
		}
		if renderTemplate.Valid {
			bt.RenderTemplate = renderTemplate.String
		}
		if allowedChildren.Valid {
			json.Unmarshal([]byte(allowedChildren.String), &bt.AllowedChildren)
		}
		if allowedParents.Valid {
			json.Unmarshal([]byte(allowedParents.String), &bt.AllowedParents)
		}
		types = append(types, &bt)
	}

	a.json(w, http.StatusOK, types)
}

func (a *API) triggerBackup(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement backup trigger
	a.json(w, http.StatusOK, map[string]string{"status": "backup_started"})
}

func (a *API) listUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := a.usersDB.QueryContext(r.Context(), `
		SELECT id, type, username, email, created_at, last_login, status
		FROM users WHERE status != 'deleted'`)
	if err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var userList []*users.User
	for rows.Next() {
		var u users.User
		var email, lastLogin sql.NullString
		err := rows.Scan(&u.ID, &u.Type, &u.Username, &email, &u.CreatedAt, &lastLogin, &u.Status)
		if err != nil {
			continue
		}
		if email.Valid {
			u.Email = email.String
		}
		userList = append(userList, &u)
	}

	a.json(w, http.StatusOK, userList)
}

func (a *API) createUser(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement user creation
	a.json(w, http.StatusCreated, map[string]string{"status": "created"})
}

func (a *API) updateUser(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement user update
	a.json(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (a *API) listPermissions(w http.ResponseWriter, r *http.Request) {
	rows, err := a.usersDB.QueryContext(r.Context(), `
		SELECT id, user_id, scope, scope_id, action, granted, granted_by, granted_at, expires_at
		FROM permissions`)
	if err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var perms []*users.Permission
	for rows.Next() {
		var p users.Permission
		var scopeID, grantedBy, expiresAt sql.NullString
		err := rows.Scan(&p.ID, &p.UserID, &p.Scope, &scopeID, &p.Action, &p.Granted,
			&grantedBy, &p.GrantedAt, &expiresAt)
		if err != nil {
			continue
		}
		if scopeID.Valid {
			p.ScopeID = &scopeID.String
		}
		if grantedBy.Valid {
			p.GrantedBy = grantedBy.String
		}
		perms = append(perms, &p)
	}

	a.json(w, http.StatusOK, perms)
}

func (a *API) grantPermission(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement permission grant
	a.json(w, http.StatusCreated, map[string]string{"status": "granted"})
}

func (a *API) revokePermission(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement permission revoke
	a.json(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// Close closes the API handler.
func (a *API) Close() error {
	var errs []error
	if err := a.contentDB.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := a.schemaDB.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := a.usersDB.Close(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}
