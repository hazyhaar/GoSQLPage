// Package bot provides bot/LLM worker integration for GoSQLPage v2.1.
package bot

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/hazyhaar/gopage/v2/pkg/blocks"
	"github.com/hazyhaar/gopage/v2/pkg/session"
	_ "modernc.org/sqlite"
)

// LLMProvider interface for LLM integrations.
type LLMProvider interface {
	Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
	Name() string
}

// GenerateRequest represents a request to the LLM.
type GenerateRequest struct {
	Prompt       string            `json:"prompt"`
	Context      []*blocks.Block   `json:"context,omitempty"`
	SystemPrompt string            `json:"system_prompt,omitempty"`
	MaxTokens    int               `json:"max_tokens,omitempty"`
	Temperature  float64           `json:"temperature,omitempty"`
	Model        string            `json:"model,omitempty"`
}

// GenerateResponse represents the LLM response.
type GenerateResponse struct {
	Content     string   `json:"content"`
	Model       string   `json:"model"`
	TokensUsed  int      `json:"tokens_used"`
	DurationMS  int64    `json:"duration_ms"`
	Reasoning   []string `json:"reasoning,omitempty"`
	FinishReason string  `json:"finish_reason"`
}

// Worker processes bot requests from the database.
type Worker struct {
	cfg          WorkerConfig
	contentDB    *sql.DB
	sessionMgr   *session.Manager
	provider     LLMProvider
	running      bool
	mu           sync.Mutex
	stopCh       chan struct{}
	logger       *slog.Logger

	// Stats
	requestsProcessed int64
	requestsFailed    int64
}

// WorkerConfig holds worker configuration.
type WorkerConfig struct {
	ContentDBPath    string
	SessionManager   *session.Manager
	Provider         LLMProvider
	BotUserID        string
	PollIntervalMS   int
	MaxConcurrent    int
	Logger           *slog.Logger
}

// NewWorker creates a new bot worker.
func NewWorker(cfg WorkerConfig) (*Worker, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.PollIntervalMS <= 0 {
		cfg.PollIntervalMS = 1000
	}
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 1
	}
	if cfg.BotUserID == "" {
		cfg.BotUserID = "bot_system"
	}

	db, err := sql.Open("sqlite", cfg.ContentDBPath)
	if err != nil {
		return nil, fmt.Errorf("open content.db: %w", err)
	}

	return &Worker{
		cfg:        cfg,
		contentDB:  db,
		sessionMgr: cfg.SessionManager,
		provider:   cfg.Provider,
		stopCh:     make(chan struct{}),
		logger:     cfg.Logger,
	}, nil
}

// Start starts the bot worker.
func (w *Worker) Start(ctx context.Context) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	w.logger.Info("bot worker started", "provider", w.provider.Name())

	ticker := time.NewTicker(time.Duration(w.cfg.PollIntervalMS) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.Stop()
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.processPendingRequests(ctx)
		}
	}
}

// Stop stops the bot worker.
func (w *Worker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	close(w.stopCh)
	w.running = false
	w.logger.Info("bot worker stopped")
}

// processPendingRequests processes pending bot requests.
func (w *Worker) processPendingRequests(ctx context.Context) {
	// Find pending bot_request blocks
	rows, err := w.contentDB.QueryContext(ctx, `
		SELECT b.id, b.content, a.value as config
		FROM blocks b
		LEFT JOIN attrs a ON a.block_id = b.id AND a.name = 'bot_config'
		WHERE b.type = 'bot_request'
		  AND b.deleted_at IS NULL
		  AND EXISTS (
			SELECT 1 FROM attrs WHERE block_id = b.id AND name = 'status' AND value = 'pending'
		  )
		ORDER BY b.created_at
		LIMIT ?`, w.cfg.MaxConcurrent)
	if err != nil {
		w.logger.Error("query pending requests", "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var blockID, content string
		var configJSON sql.NullString

		if err := rows.Scan(&blockID, &content, &configJSON); err != nil {
			continue
		}

		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := w.processRequest(ctx, blockID, content, configJSON.String); err != nil {
			w.logger.Error("process request failed", "block_id", blockID, "error", err)
			w.markFailed(ctx, blockID, err.Error())
			w.requestsFailed++
		} else {
			w.requestsProcessed++
		}
	}
}

// processRequest processes a single bot request.
func (w *Worker) processRequest(ctx context.Context, blockID, content, configJSON string) error {
	w.logger.Info("processing bot request", "block_id", blockID)

	// Parse config
	var config BotRequestConfig
	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
			return fmt.Errorf("parse config: %w", err)
		}
	}

	// Mark as processing
	if err := w.updateStatus(ctx, blockID, "processing"); err != nil {
		return fmt.Errorf("mark processing: %w", err)
	}

	// Get context blocks if specified
	var contextBlocks []*blocks.Block
	if len(config.ContextBlocks) > 0 {
		for _, contextID := range config.ContextBlocks {
			block, err := w.getBlock(ctx, contextID)
			if err != nil {
				w.logger.Warn("failed to get context block", "block_id", contextID, "error", err)
				continue
			}
			contextBlocks = append(contextBlocks, block)
		}
	}

	// Build request
	req := &GenerateRequest{
		Prompt:       content,
		Context:      contextBlocks,
		SystemPrompt: config.SystemPrompt,
		MaxTokens:    config.MaxTokens,
		Temperature:  config.Temperature,
		Model:        config.Model,
	}

	// Call LLM
	startTime := time.Now()
	resp, err := w.provider.Generate(ctx, req)
	if err != nil {
		return fmt.Errorf("llm generate: %w", err)
	}
	resp.DurationMS = time.Since(startTime).Milliseconds()

	// Create response block via session
	if err := w.createResponse(ctx, blockID, resp); err != nil {
		return fmt.Errorf("create response: %w", err)
	}

	// Mark request as completed
	if err := w.updateStatus(ctx, blockID, "completed"); err != nil {
		return fmt.Errorf("mark completed: %w", err)
	}

	w.logger.Info("bot request completed", "block_id", blockID, "tokens", resp.TokensUsed, "duration_ms", resp.DurationMS)
	return nil
}

// BotRequestConfig holds bot request configuration.
type BotRequestConfig struct {
	BotID         string   `json:"bot_id,omitempty"`
	Model         string   `json:"model,omitempty"`
	MaxTokens     int      `json:"max_tokens,omitempty"`
	Temperature   float64  `json:"temperature,omitempty"`
	SystemPrompt  string   `json:"system_prompt,omitempty"`
	ContextBlocks []string `json:"context_blocks,omitempty"`
}

// getBlock retrieves a block from content.db.
func (w *Worker) getBlock(ctx context.Context, blockID string) (*blocks.Block, error) {
	var block blocks.Block
	var parentID, contentHTML sql.NullString

	row := w.contentDB.QueryRowContext(ctx, `
		SELECT id, parent_id, type, content, content_html, position, hash,
		       created_at, updated_at, created_by, published
		FROM blocks WHERE id = ? AND deleted_at IS NULL`, blockID)

	err := row.Scan(&block.ID, &parentID, &block.Type, &block.Content, &contentHTML,
		&block.Position, &block.Hash, &block.CreatedAt, &block.UpdatedAt,
		&block.CreatedBy, &block.Published)
	if err != nil {
		return nil, err
	}

	if parentID.Valid {
		block.ParentID = &parentID.String
	}
	if contentHTML.Valid {
		block.ContentHTML = contentHTML.String
	}

	return &block, nil
}

// createResponse creates a bot response block.
func (w *Worker) createResponse(ctx context.Context, requestBlockID string, resp *GenerateResponse) error {
	// Create a session for the bot
	sess, err := w.sessionMgr.Create(ctx, w.cfg.BotUserID, "bot")
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	// Create response block
	responseBlock := &blocks.Block{
		ID:        blocks.NewBlockID(),
		ParentID:  &requestBlockID, // Response is child of request
		Type:      blocks.TypeBotResponse,
		Content:   resp.Content,
		CreatedBy: w.cfg.BotUserID,
		Published: true,
	}

	if err := w.sessionMgr.InsertBlock(ctx, sess.ID, responseBlock); err != nil {
		return fmt.Errorf("insert response block: %w", err)
	}

	// Create reasoning block if available
	if len(resp.Reasoning) > 0 {
		reasoningJSON, _ := json.Marshal(resp.Reasoning)
		reasoningBlock := &blocks.Block{
			ID:        blocks.NewBlockID(),
			ParentID:  &responseBlock.ID,
			Type:      "bot_reasoning",
			Content:   string(reasoningJSON),
			CreatedBy: w.cfg.BotUserID,
			Published: true,
		}

		if err := w.sessionMgr.InsertBlock(ctx, sess.ID, reasoningBlock); err != nil {
			w.logger.Warn("failed to insert reasoning block", "error", err)
		}
	}

	// Submit session for merge
	if err := w.sessionMgr.Submit(ctx, sess.ID); err != nil {
		return fmt.Errorf("submit session: %w", err)
	}

	return nil
}

// updateStatus updates a block's status attribute.
func (w *Worker) updateStatus(ctx context.Context, blockID, status string) error {
	_, err := w.contentDB.ExecContext(ctx, `
		INSERT OR REPLACE INTO attrs (block_id, name, value) VALUES (?, 'status', ?)`,
		blockID, status)
	return err
}

// markFailed marks a request as failed.
func (w *Worker) markFailed(ctx context.Context, blockID, errorMsg string) error {
	if err := w.updateStatus(ctx, blockID, "failed"); err != nil {
		return err
	}
	_, err := w.contentDB.ExecContext(ctx, `
		INSERT OR REPLACE INTO attrs (block_id, name, value) VALUES (?, 'error', ?)`,
		blockID, errorMsg)
	return err
}

// Stats returns worker statistics.
type Stats struct {
	Running           bool   `json:"running"`
	Provider          string `json:"provider"`
	RequestsProcessed int64  `json:"requests_processed"`
	RequestsFailed    int64  `json:"requests_failed"`
}

// GetStats returns worker statistics.
func (w *Worker) GetStats() *Stats {
	return &Stats{
		Running:           w.running,
		Provider:          w.provider.Name(),
		RequestsProcessed: w.requestsProcessed,
		RequestsFailed:    w.requestsFailed,
	}
}

// Close closes the worker.
func (w *Worker) Close() error {
	w.Stop()
	return w.contentDB.Close()
}

// MockProvider is a mock LLM provider for testing.
type MockProvider struct{}

// Name returns the provider name.
func (m *MockProvider) Name() string {
	return "mock"
}

// Generate generates a mock response.
func (m *MockProvider) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	return &GenerateResponse{
		Content:      fmt.Sprintf("Mock response to: %s", req.Prompt[:min(50, len(req.Prompt))]),
		Model:        "mock-1.0",
		TokensUsed:   100,
		FinishReason: "stop",
		Reasoning:    []string{"This is a mock response", "No actual LLM was used"},
	}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
