// Package backup provides automated backup for GoSQLPage v2.1.
package backup

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Manager handles automated backups.
type Manager struct {
	cfg       Config
	running   bool
	mu        sync.Mutex
	stopCh    chan struct{}
	logger    *slog.Logger
	lastRun   time.Time

	// Stats
	backupsTotal  int64
	backupsFailed int64
}

// Config holds backup configuration.
type Config struct {
	BackupDir      string
	Databases      []DatabaseConfig
	IntervalHours  int
	RetentionDays  int
	MaxBackups     int
	CompressBackups bool
	Logger         *slog.Logger
}

// DatabaseConfig holds database backup configuration.
type DatabaseConfig struct {
	Name string
	Path string
}

// New creates a new backup manager.
func New(cfg Config) (*Manager, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.IntervalHours <= 0 {
		cfg.IntervalHours = 24
	}
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = 30
	}
	if cfg.MaxBackups <= 0 {
		cfg.MaxBackups = 10
	}

	if err := os.MkdirAll(cfg.BackupDir, 0755); err != nil {
		return nil, fmt.Errorf("create backup dir: %w", err)
	}

	return &Manager{
		cfg:    cfg,
		stopCh: make(chan struct{}),
		logger: cfg.Logger,
	}, nil
}

// Start starts the backup scheduler.
func (m *Manager) Start(ctx context.Context) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	m.logger.Info("backup manager started", "interval_hours", m.cfg.IntervalHours)

	// Run immediately on start
	m.runBackup(ctx)

	ticker := time.NewTicker(time.Duration(m.cfg.IntervalHours) * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.Stop()
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.runBackup(ctx)
		}
	}
}

// Stop stops the backup manager.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	close(m.stopCh)
	m.running = false
	m.logger.Info("backup manager stopped")
}

// runBackup runs a backup cycle.
func (m *Manager) runBackup(ctx context.Context) {
	m.lastRun = time.Now()
	timestamp := m.lastRun.Format("20060102_150405")

	m.logger.Info("starting backup cycle", "timestamp", timestamp)

	var failed int
	for _, dbCfg := range m.cfg.Databases {
		select {
		case <-ctx.Done():
			return
		default:
		}

		backupPath := filepath.Join(m.cfg.BackupDir, fmt.Sprintf("%s_%s.db", timestamp, dbCfg.Name))
		if err := m.backupDatabase(ctx, dbCfg.Path, backupPath); err != nil {
			m.logger.Error("backup failed", "database", dbCfg.Name, "error", err)
			failed++
			continue
		}

		m.logger.Info("backup completed", "database", dbCfg.Name, "path", backupPath)
	}

	if failed > 0 {
		m.backupsFailed++
	} else {
		m.backupsTotal++
	}

	// Cleanup old backups
	m.cleanupOldBackups()
}

// backupDatabase creates a backup of a database using SQLite's backup API.
func (m *Manager) backupDatabase(ctx context.Context, srcPath, dstPath string) error {
	// Open source database
	srcDB, err := sql.Open("sqlite", srcPath+"?mode=ro")
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer srcDB.Close()

	// Create destination file
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer dstFile.Close()

	// Use VACUUM INTO for atomic backup (SQLite 3.27+)
	_, err = srcDB.ExecContext(ctx, fmt.Sprintf("VACUUM INTO '%s'", dstPath))
	if err != nil {
		// Fallback to file copy if VACUUM INTO not supported
		return m.copyFile(srcPath, dstPath)
	}

	return nil
}

// copyFile copies a file.
func (m *Manager) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// cleanupOldBackups removes old backups based on retention policy.
func (m *Manager) cleanupOldBackups() {
	entries, err := os.ReadDir(m.cfg.BackupDir)
	if err != nil {
		m.logger.Error("read backup dir", "error", err)
		return
	}

	// Group backups by timestamp
	type backupSet struct {
		timestamp string
		files     []string
		modTime   time.Time
	}

	sets := make(map[string]*backupSet)

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".db" {
			continue
		}

		// Parse timestamp from filename (format: 20060102_150405_name.db)
		name := entry.Name()
		if len(name) < 15 {
			continue
		}
		timestamp := name[:15]

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if sets[timestamp] == nil {
			sets[timestamp] = &backupSet{
				timestamp: timestamp,
				modTime:   info.ModTime(),
			}
		}
		sets[timestamp].files = append(sets[timestamp].files, entry.Name())
	}

	// Sort by timestamp (newest first)
	var timestamps []string
	for ts := range sets {
		timestamps = append(timestamps, ts)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(timestamps)))

	// Remove old backups
	cutoff := time.Now().AddDate(0, 0, -m.cfg.RetentionDays)
	removed := 0

	for i, ts := range timestamps {
		set := sets[ts]

		// Keep MaxBackups most recent
		if i >= m.cfg.MaxBackups || set.modTime.Before(cutoff) {
			for _, file := range set.files {
				path := filepath.Join(m.cfg.BackupDir, file)
				if err := os.Remove(path); err != nil {
					m.logger.Error("remove old backup", "path", path, "error", err)
				} else {
					removed++
				}
			}
		}
	}

	if removed > 0 {
		m.logger.Info("cleaned up old backups", "removed", removed)
	}
}

// RunNow triggers an immediate backup.
func (m *Manager) RunNow(ctx context.Context) error {
	m.runBackup(ctx)
	return nil
}

// ListBackups lists available backups.
type BackupInfo struct {
	Timestamp string    `json:"timestamp"`
	Files     []string  `json:"files"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

// ListBackups returns a list of available backups.
func (m *Manager) ListBackups() ([]*BackupInfo, error) {
	entries, err := os.ReadDir(m.cfg.BackupDir)
	if err != nil {
		return nil, err
	}

	// Group by timestamp
	backups := make(map[string]*BackupInfo)

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".db" {
			continue
		}

		name := entry.Name()
		if len(name) < 15 {
			continue
		}
		timestamp := name[:15]

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if backups[timestamp] == nil {
			backups[timestamp] = &BackupInfo{
				Timestamp: timestamp,
				CreatedAt: info.ModTime(),
			}
		}

		backups[timestamp].Files = append(backups[timestamp].Files, name)
		backups[timestamp].SizeBytes += info.Size()
	}

	// Convert to slice and sort
	var result []*BackupInfo
	for _, b := range backups {
		result = append(result, b)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp > result[j].Timestamp
	})

	return result, nil
}

// Restore restores databases from a backup.
func (m *Manager) Restore(ctx context.Context, timestamp string, targetDir string) error {
	if targetDir == "" {
		return fmt.Errorf("target directory required")
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	entries, err := os.ReadDir(m.cfg.BackupDir)
	if err != nil {
		return err
	}

	restored := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if len(name) < 15 || name[:15] != timestamp {
			continue
		}

		src := filepath.Join(m.cfg.BackupDir, name)
		dst := filepath.Join(targetDir, name[16:]) // Remove timestamp prefix

		if err := m.copyFile(src, dst); err != nil {
			return fmt.Errorf("restore %s: %w", name, err)
		}

		m.logger.Info("restored database", "from", src, "to", dst)
		restored++
	}

	if restored == 0 {
		return fmt.Errorf("no backups found for timestamp: %s", timestamp)
	}

	return nil
}

// Stats returns backup statistics.
type Stats struct {
	Running       bool      `json:"running"`
	LastRunAt     time.Time `json:"last_run_at"`
	NextRunAt     time.Time `json:"next_run_at"`
	BackupsTotal  int64     `json:"backups_total"`
	BackupsFailed int64     `json:"backups_failed"`
	BackupDir     string    `json:"backup_dir"`
}

// GetStats returns backup statistics.
func (m *Manager) GetStats() *Stats {
	nextRun := m.lastRun.Add(time.Duration(m.cfg.IntervalHours) * time.Hour)

	return &Stats{
		Running:       m.running,
		LastRunAt:     m.lastRun,
		NextRunAt:     nextRun,
		BackupsTotal:  m.backupsTotal,
		BackupsFailed: m.backupsFailed,
		BackupDir:     m.cfg.BackupDir,
	}
}
