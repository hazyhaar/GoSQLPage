// Package gc provides garbage collection for GoSQLPage v2.1.
// Handles cleanup of abandoned sessions, old audit logs, and database maintenance.
package gc

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Config holds GC configuration.
type Config struct {
	SessionsDir           string
	AuditDBPath           string
	ContentDBPath         string
	FailedDir             string
	DoneDir               string
	BackupDir             string
	IntervalHours         int
	AbandonedDays         int
	MergedDays            int
	FailedArchiveDays     int
	AuditRetentionDays    int
	AuditArchiveAfterDays int
	VacuumThreshold       int
	VacuumStartHour       int
	VacuumEndHour         int
	CacheExpireHours      int
	Logger                *slog.Logger
}

// GC is the garbage collector.
type GC struct {
	cfg     Config
	running bool
	mu      sync.Mutex
	stopCh  chan struct{}
	logger  *slog.Logger
	lastRun time.Time

	// Stats
	sessionsCleanedTotal int64
	auditArchivedTotal   int64
	vacuumsRunTotal      int64
}

// New creates a new garbage collector.
func New(cfg Config) *GC {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.IntervalHours <= 0 {
		cfg.IntervalHours = 6
	}
	if cfg.AbandonedDays <= 0 {
		cfg.AbandonedDays = 7
	}
	if cfg.MergedDays <= 0 {
		cfg.MergedDays = 1
	}
	if cfg.FailedArchiveDays <= 0 {
		cfg.FailedArchiveDays = 30
	}
	if cfg.AuditRetentionDays <= 0 {
		cfg.AuditRetentionDays = 90
	}
	if cfg.AuditArchiveAfterDays <= 0 {
		cfg.AuditArchiveAfterDays = 30
	}
	if cfg.VacuumThreshold <= 0 {
		cfg.VacuumThreshold = 20
	}
	if cfg.CacheExpireHours <= 0 {
		cfg.CacheExpireHours = 24
	}

	return &GC{
		cfg:    cfg,
		stopCh: make(chan struct{}),
		logger: cfg.Logger,
	}
}

// Start starts the garbage collector.
func (g *GC) Start(ctx context.Context) {
	g.mu.Lock()
	if g.running {
		g.mu.Unlock()
		return
	}
	g.running = true
	g.mu.Unlock()

	g.logger.Info("GC started", "interval_hours", g.cfg.IntervalHours)

	// Run immediately on start
	g.run(ctx)

	ticker := time.NewTicker(time.Duration(g.cfg.IntervalHours) * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			g.Stop()
			return
		case <-g.stopCh:
			return
		case <-ticker.C:
			g.run(ctx)
		}
	}
}

// Stop stops the garbage collector.
func (g *GC) Stop() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.running {
		return
	}

	close(g.stopCh)
	g.running = false
	g.logger.Info("GC stopped")
}

// run executes a GC cycle.
func (g *GC) run(ctx context.Context) {
	g.lastRun = time.Now()
	g.logger.Info("GC cycle started")

	// Clean abandoned sessions
	abandoned, err := g.cleanAbandonedSessions(ctx)
	if err != nil {
		g.logger.Error("clean abandoned sessions", "error", err)
	} else {
		g.sessionsCleanedTotal += int64(abandoned)
	}

	// Clean merged sessions
	merged, err := g.cleanMergedSessions(ctx)
	if err != nil {
		g.logger.Error("clean merged sessions", "error", err)
	} else {
		g.sessionsCleanedTotal += int64(merged)
	}

	// Archive old failed sessions
	archived, err := g.archiveFailedSessions(ctx)
	if err != nil {
		g.logger.Error("archive failed sessions", "error", err)
	}

	// Clean old audit logs
	auditCleaned, err := g.cleanAuditLogs(ctx)
	if err != nil {
		g.logger.Error("clean audit logs", "error", err)
	} else {
		g.auditArchivedTotal += int64(auditCleaned)
	}

	// Vacuum databases if needed and in time window
	hour := time.Now().Hour()
	if hour >= g.cfg.VacuumStartHour && hour < g.cfg.VacuumEndHour {
		if err := g.vacuumDatabases(ctx); err != nil {
			g.logger.Error("vacuum databases", "error", err)
		}
	}

	g.logger.Info("GC cycle completed",
		"abandoned_cleaned", abandoned,
		"merged_cleaned", merged,
		"failed_archived", archived,
		"audit_cleaned", auditCleaned)
}

// cleanAbandonedSessions removes sessions that have been inactive too long.
func (g *GC) cleanAbandonedSessions(ctx context.Context) (int, error) {
	entries, err := os.ReadDir(g.cfg.SessionsDir)
	if err != nil {
		return 0, err
	}

	threshold := time.Now().AddDate(0, 0, -g.cfg.AbandonedDays)
	cleaned := 0

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".db" {
			continue
		}

		path := filepath.Join(g.cfg.SessionsDir, entry.Name())

		// Check session status and last activity
		db, err := sql.Open("sqlite", path+"?mode=ro")
		if err != nil {
			continue
		}

		var status, lastActivity string
		row := db.QueryRowContext(ctx, "SELECT status, last_activity FROM _session_meta LIMIT 1")
		if err := row.Scan(&status, &lastActivity); err != nil {
			db.Close()
			continue
		}
		db.Close()

		// Only clean active sessions that are abandoned
		if status != "active" {
			continue
		}

		activityTime, err := time.Parse(time.RFC3339, lastActivity)
		if err != nil {
			continue
		}

		if activityTime.Before(threshold) {
			g.logger.Info("removing abandoned session", "file", entry.Name())
			if err := os.Remove(path); err != nil {
				g.logger.Error("remove abandoned session", "file", entry.Name(), "error", err)
				continue
			}
			cleaned++
		}
	}

	return cleaned, nil
}

// cleanMergedSessions removes sessions that have been merged.
func (g *GC) cleanMergedSessions(ctx context.Context) (int, error) {
	entries, err := os.ReadDir(g.cfg.DoneDir)
	if err != nil {
		return 0, err
	}

	threshold := time.Now().AddDate(0, 0, -g.cfg.MergedDays)
	cleaned := 0

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".db" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(threshold) {
			path := filepath.Join(g.cfg.DoneDir, entry.Name())
			if err := os.Remove(path); err != nil {
				g.logger.Error("remove merged session", "file", entry.Name(), "error", err)
				continue
			}
			cleaned++
		}
	}

	return cleaned, nil
}

// archiveFailedSessions moves old failed sessions to archive.
func (g *GC) archiveFailedSessions(ctx context.Context) (int, error) {
	entries, err := os.ReadDir(g.cfg.FailedDir)
	if err != nil {
		return 0, err
	}

	threshold := time.Now().AddDate(0, 0, -g.cfg.FailedArchiveDays)
	archived := 0

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".db" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(threshold) {
			path := filepath.Join(g.cfg.FailedDir, entry.Name())
			// For now, just delete old failed sessions
			// Could be archived to backup dir instead
			if err := os.Remove(path); err != nil {
				g.logger.Error("archive failed session", "file", entry.Name(), "error", err)
				continue
			}
			archived++
		}
	}

	return archived, nil
}

// cleanAuditLogs removes old audit log entries.
func (g *GC) cleanAuditLogs(ctx context.Context) (int, error) {
	db, err := sql.Open("sqlite", g.cfg.AuditDBPath)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	cutoff := time.Now().AddDate(0, 0, -g.cfg.AuditRetentionDays).Format(time.RFC3339)

	result, err := db.ExecContext(ctx, `
		DELETE FROM audit_log WHERE timestamp < ?`, cutoff)
	if err != nil {
		return 0, err
	}

	affected, _ := result.RowsAffected()

	// Also clean old merge logs
	db.ExecContext(ctx, `DELETE FROM merge_log WHERE timestamp < ?`, cutoff)

	return int(affected), nil
}

// vacuumDatabases runs VACUUM on databases if fragmentation is high.
func (g *GC) vacuumDatabases(ctx context.Context) error {
	databases := []string{
		g.cfg.ContentDBPath,
		g.cfg.AuditDBPath,
	}

	for _, dbPath := range databases {
		if err := g.vacuumIfNeeded(ctx, dbPath); err != nil {
			g.logger.Error("vacuum database", "path", dbPath, "error", err)
		}
	}

	return nil
}

// vacuumIfNeeded runs VACUUM on a database if fragmentation exceeds threshold.
func (g *GC) vacuumIfNeeded(ctx context.Context, dbPath string) error {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// Check fragmentation using page counts
	var pageCount, freePages int64
	row := db.QueryRowContext(ctx, "PRAGMA page_count")
	row.Scan(&pageCount)
	row = db.QueryRowContext(ctx, "PRAGMA freelist_count")
	row.Scan(&freePages)

	if pageCount == 0 {
		return nil
	}

	fragmentation := int(float64(freePages) / float64(pageCount) * 100)
	if fragmentation < g.cfg.VacuumThreshold {
		return nil
	}

	g.logger.Info("running vacuum", "path", dbPath, "fragmentation", fragmentation)
	_, err = db.ExecContext(ctx, "VACUUM")
	if err == nil {
		g.vacuumsRunTotal++
	}
	return err
}

// RunNow triggers an immediate GC cycle.
func (g *GC) RunNow(ctx context.Context) {
	g.run(ctx)
}

// Health returns the GC health status.
type Health struct {
	Status          string    `json:"status"`
	LastRunAt       time.Time `json:"last_run_at"`
	SessionsCleaned int64     `json:"sessions_cleaned"`
	AuditArchived   int64     `json:"audit_archived"`
	NextRunAt       time.Time `json:"next_run_at"`
}

// GetHealth returns the current health status.
func (g *GC) GetHealth() *Health {
	status := "ok"
	if !g.running {
		status = "stopped"
	}

	nextRun := g.lastRun.Add(time.Duration(g.cfg.IntervalHours) * time.Hour)

	return &Health{
		Status:          status,
		LastRunAt:       g.lastRun,
		SessionsCleaned: g.sessionsCleanedTotal,
		AuditArchived:   g.auditArchivedTotal,
		NextRunAt:       nextRun,
	}
}
