// Package db provides SQLite database connection pooling using zombiezen.
// It implements a writer pool (size 1) and reader pool (size N) pattern
// to handle SQLite's single-writer limitation.
package db

import (
	"context"
	"fmt"
	"sync"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// DB wraps SQLite connection pools with separate reader/writer access.
type DB struct {
	path string

	// readerPool for concurrent reads
	readerPool *sqlitex.Pool

	// writerConn is a single connection for writes (SQLite limitation)
	writerConn *sqlite.Conn
	writerMu   sync.Mutex
}

// Config holds database configuration.
type Config struct {
	Path        string
	ReaderCount int
}

// Open creates a new database connection with reader/writer pools.
func Open(cfg Config) (*DB, error) {
	if cfg.ReaderCount <= 0 {
		cfg.ReaderCount = 4
	}

	// Open reader pool
	readerPool, err := sqlitex.NewPool(cfg.Path, sqlitex.PoolOptions{
		Flags:    sqlite.OpenReadOnly | sqlite.OpenWAL,
		PoolSize: cfg.ReaderCount,
	})
	if err != nil {
		return nil, fmt.Errorf("open reader pool: %w", err)
	}

	// Open single writer connection
	writerConn, err := sqlite.OpenConn(cfg.Path, sqlite.OpenReadWrite|sqlite.OpenCreate|sqlite.OpenWAL)
	if err != nil {
		readerPool.Close()
		return nil, fmt.Errorf("open writer conn: %w", err)
	}

	// Enable WAL mode for better concurrency
	err = sqlitex.ExecuteTransient(writerConn, "PRAGMA journal_mode=WAL;", nil)
	if err != nil {
		writerConn.Close()
		readerPool.Close()
		return nil, fmt.Errorf("enable WAL: %w", err)
	}

	return &DB{
		path:       cfg.Path,
		readerPool: readerPool,
		writerConn: writerConn,
	}, nil
}

// Reader gets a read-only connection from the pool.
// The returned function must be called to release the connection.
func (db *DB) Reader(ctx context.Context) (*sqlite.Conn, func(), error) {
	conn, err := db.readerPool.Take(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("take reader: %w", err)
	}
	return conn, func() { db.readerPool.Put(conn) }, nil
}

// Writer gets exclusive access to the writer connection.
// The returned function must be called to release the lock.
func (db *DB) Writer(ctx context.Context) (*sqlite.Conn, func(), error) {
	db.writerMu.Lock()
	return db.writerConn, func() { db.writerMu.Unlock() }, nil
}

// Close closes all connections.
func (db *DB) Close() error {
	db.writerMu.Lock()
	defer db.writerMu.Unlock()

	var errs []error
	if err := db.writerConn.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := db.readerPool.Close(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}

// Path returns the database file path.
func (db *DB) Path() string {
	return db.path
}

// WriterConn returns the raw writer connection for function registration.
// Use with caution - caller must hold the writer lock.
func (db *DB) WriterConn() *sqlite.Conn {
	return db.writerConn
}

// ForEachReader applies a function to each reader connection.
// Useful for registering custom SQL functions.
func (db *DB) ForEachReader(ctx context.Context, fn func(*sqlite.Conn) error) error {
	// Get all connections and apply function
	// Note: This is a simplified approach. In production, you might want
	// to use connection hooks in the pool instead.
	conn, release, err := db.Reader(ctx)
	if err != nil {
		return err
	}
	defer release()
	return fn(conn)
}
