// Package cache provides page caching with invalidation for GoSQLPage v2.1.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Cache provides page caching with LRU eviction and invalidation.
type Cache struct {
	dir       string
	maxSize   int64
	ttl       time.Duration
	enabled   bool
	logger    *slog.Logger

	mu        sync.RWMutex
	entries   map[string]*entry
	size      int64
	hits      int64
	misses    int64

	// Block to pages mapping for invalidation
	blockPages map[string][]string // blockID -> []pageKeys
}

type entry struct {
	key       string
	path      string
	size      int64
	createdAt time.Time
	accessedAt time.Time
	blocks    []string // blocks that this page depends on
}

// Config holds cache configuration.
type Config struct {
	Dir       string
	MaxSizeMB int64
	TTLHours  int
	Enabled   bool
	Logger    *slog.Logger
}

// New creates a new cache.
func New(cfg Config) (*Cache, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.MaxSizeMB <= 0 {
		cfg.MaxSizeMB = 100 // 100MB default
	}
	if cfg.TTLHours <= 0 {
		cfg.TTLHours = 24
	}

	if err := os.MkdirAll(cfg.Dir, 0755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}

	c := &Cache{
		dir:        cfg.Dir,
		maxSize:    cfg.MaxSizeMB * 1024 * 1024,
		ttl:        time.Duration(cfg.TTLHours) * time.Hour,
		enabled:    cfg.Enabled,
		logger:     cfg.Logger,
		entries:    make(map[string]*entry),
		blockPages: make(map[string][]string),
	}

	// Load existing cache entries
	if err := c.loadExisting(); err != nil {
		cfg.Logger.Warn("failed to load existing cache", "error", err)
	}

	return c, nil
}

// loadExisting loads existing cache entries from disk.
func (c *Cache) loadExisting() error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		key := e.Name()
		if filepath.Ext(key) == ".html" {
			key = key[:len(key)-5]
		}

		c.entries[key] = &entry{
			key:        key,
			path:       filepath.Join(c.dir, e.Name()),
			size:       info.Size(),
			createdAt:  info.ModTime(),
			accessedAt: info.ModTime(),
		}
		c.size += info.Size()
	}

	c.logger.Info("loaded cache entries", "count", len(c.entries), "size_mb", c.size/(1024*1024))
	return nil
}

// KeyForPage generates a cache key for a page.
func (c *Cache) KeyForPage(path string, params map[string]string) string {
	h := sha256.New()
	h.Write([]byte(path))

	// Sort and hash params for deterministic key
	for k, v := range params {
		h.Write([]byte(k))
		h.Write([]byte(v))
	}

	return hex.EncodeToString(h.Sum(nil))[:16]
}

// Get retrieves a cached page.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool) {
	if !c.enabled {
		return nil, false
	}

	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return nil, false
	}

	// Check TTL
	if time.Since(e.createdAt) > c.ttl {
		c.Delete(key)
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return nil, false
	}

	// Read from disk
	data, err := os.ReadFile(e.path)
	if err != nil {
		c.Delete(key)
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return nil, false
	}

	// Update access time
	c.mu.Lock()
	e.accessedAt = time.Now()
	c.hits++
	c.mu.Unlock()

	return data, true
}

// Set stores a page in the cache.
func (c *Cache) Set(ctx context.Context, key string, data []byte, dependsOnBlocks []string) error {
	if !c.enabled {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to evict
	for c.size+int64(len(data)) > c.maxSize && len(c.entries) > 0 {
		c.evictOldest()
	}

	// Write to disk
	path := filepath.Join(c.dir, key+".html")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write cache file: %w", err)
	}

	// Update entry
	e := &entry{
		key:        key,
		path:       path,
		size:       int64(len(data)),
		createdAt:  time.Now(),
		accessedAt: time.Now(),
		blocks:     dependsOnBlocks,
	}

	// Remove old entry size if replacing
	if old, ok := c.entries[key]; ok {
		c.size -= old.size
		// Remove old block mappings
		for _, blockID := range old.blocks {
			c.removeBlockMapping(blockID, key)
		}
	}

	c.entries[key] = e
	c.size += e.size

	// Add block mappings
	for _, blockID := range dependsOnBlocks {
		c.blockPages[blockID] = append(c.blockPages[blockID], key)
	}

	return nil
}

// Delete removes a cache entry.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.entries[key]
	if !ok {
		return
	}

	os.Remove(e.path)
	c.size -= e.size
	delete(c.entries, key)

	// Remove block mappings
	for _, blockID := range e.blocks {
		c.removeBlockMapping(blockID, key)
	}
}

// InvalidateBlock invalidates all pages that depend on a block.
func (c *Cache) InvalidateBlock(blockID string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	pages, ok := c.blockPages[blockID]
	if !ok {
		return 0
	}

	count := 0
	for _, key := range pages {
		if e, ok := c.entries[key]; ok {
			os.Remove(e.path)
			c.size -= e.size
			delete(c.entries, key)
			count++
		}
	}

	delete(c.blockPages, blockID)
	c.logger.Debug("invalidated cache for block", "block_id", blockID, "pages_invalidated", count)
	return count
}

// InvalidateBlocks invalidates all pages for multiple blocks.
func (c *Cache) InvalidateBlocks(blockIDs []string) int {
	total := 0
	for _, id := range blockIDs {
		total += c.InvalidateBlock(id)
	}
	return total
}

// evictOldest evicts the least recently accessed entry.
func (c *Cache) evictOldest() {
	var oldest *entry
	for _, e := range c.entries {
		if oldest == nil || e.accessedAt.Before(oldest.accessedAt) {
			oldest = e
		}
	}

	if oldest != nil {
		os.Remove(oldest.path)
		c.size -= oldest.size
		delete(c.entries, oldest.key)

		for _, blockID := range oldest.blocks {
			c.removeBlockMapping(blockID, oldest.key)
		}

		c.logger.Debug("evicted cache entry", "key", oldest.key)
	}
}

// removeBlockMapping removes a page from a block's mapping.
func (c *Cache) removeBlockMapping(blockID, pageKey string) {
	pages := c.blockPages[blockID]
	for i, p := range pages {
		if p == pageKey {
			c.blockPages[blockID] = append(pages[:i], pages[i+1:]...)
			break
		}
	}
	if len(c.blockPages[blockID]) == 0 {
		delete(c.blockPages, blockID)
	}
}

// Clear clears the entire cache.
func (c *Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return err
	}

	for _, e := range entries {
		os.Remove(filepath.Join(c.dir, e.Name()))
	}

	c.entries = make(map[string]*entry)
	c.blockPages = make(map[string][]string)
	c.size = 0

	c.logger.Info("cache cleared")
	return nil
}

// Stats returns cache statistics.
type Stats struct {
	Enabled   bool    `json:"enabled"`
	Entries   int     `json:"entries"`
	SizeBytes int64   `json:"size_bytes"`
	SizeMB    float64 `json:"size_mb"`
	MaxSizeMB float64 `json:"max_size_mb"`
	Hits      int64   `json:"hits"`
	Misses    int64   `json:"misses"`
	HitRatio  float64 `json:"hit_ratio"`
}

// GetStats returns cache statistics.
func (c *Cache) GetStats() *Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	hitRatio := float64(0)
	if total > 0 {
		hitRatio = float64(c.hits) / float64(total)
	}

	return &Stats{
		Enabled:   c.enabled,
		Entries:   len(c.entries),
		SizeBytes: c.size,
		SizeMB:    float64(c.size) / (1024 * 1024),
		MaxSizeMB: float64(c.maxSize) / (1024 * 1024),
		Hits:      c.hits,
		Misses:    c.misses,
		HitRatio:  hitRatio,
	}
}

// Warmup pre-generates cache for frequently accessed pages.
func (c *Cache) Warmup(ctx context.Context, generator func(path string) ([]byte, []string, error), paths []string) error {
	for _, path := range paths {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		key := c.KeyForPage(path, nil)
		if _, ok := c.Get(ctx, key); ok {
			continue // Already cached
		}

		data, blocks, err := generator(path)
		if err != nil {
			c.logger.Warn("warmup failed for path", "path", path, "error", err)
			continue
		}

		if err := c.Set(ctx, key, data, blocks); err != nil {
			c.logger.Warn("warmup cache set failed", "path", path, "error", err)
		}
	}

	return nil
}

// Writer returns a writer that caches the output.
type Writer struct {
	cache  *Cache
	key    string
	blocks []string
	buf    []byte
	w      io.Writer
}

// NewWriter creates a new cache writer.
func (c *Cache) NewWriter(w io.Writer, key string, blocks []string) *Writer {
	return &Writer{
		cache:  c,
		key:    key,
		blocks: blocks,
		w:      w,
	}
}

// Write implements io.Writer.
func (cw *Writer) Write(p []byte) (n int, err error) {
	cw.buf = append(cw.buf, p...)
	return cw.w.Write(p)
}

// Close saves the buffered data to cache.
func (cw *Writer) Close() error {
	return cw.cache.Set(context.Background(), cw.key, cw.buf, cw.blocks)
}
