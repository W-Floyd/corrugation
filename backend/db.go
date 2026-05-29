package backend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	db    *gorm.DB
	dbCtx context.Context
)

func ConnectDB(dbFilePath string) (err error) {
	if dbCtx == nil {
		dbCtx = context.Background()
	}
	Log.Infow("connecting to DB", "path", dbFilePath)
	if db != nil {
		return errors.New("db is already defined, will not override")
	}

	sqliteDB, err := gorm.Open(sqlite.Open(dbFilePath), &gorm.Config{
		Logger: newGORMLogger(),
	})
	if err != nil {
		return err
	}

	db = sqliteDB

	// Optimize connection pool for concurrent reads
	if dbPool, err := sqliteDB.DB(); err == nil {
		dbPool.SetMaxIdleConns(10)
		dbPool.SetMaxOpenConns(10)
		dbPool.SetConnMaxLifetime(0) // Connection reuses indefinitely
	}

	// Enable WAL mode for better concurrent read performance
	if err = db.Exec("PRAGMA journal_mode=WAL").Error; err != nil {
		Log.Warnw("Could not enable WAL mode", "error", err)
	}

	// Optimize for concurrent reads
	if err = db.Exec("PRAGMA cache_size=-64000").Error; err != nil {
		Log.Warnw("Could not set cache size", "error", err)
	}

	return
}

// BackupDB creates a timestamped copy of the database via VACUUM INTO, then
// prunes backups in backupDir beyond the keep most recent. keep=0 disables.
// Must be called after ConnectDB.
func BackupDB(backupDir string, keep int) error {
	if keep == 0 {
		return nil
	}
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}

	dest := filepath.Join(backupDir, fmt.Sprintf("db.sqlite.%s", time.Now().Format("20060102-150405")))
	Log.Infow("backing up database", "dest", dest)
	if err := db.Exec("VACUUM INTO ?", dest).Error; err != nil {
		return fmt.Errorf("vacuum into: %w", err)
	}

	entries, _ := filepath.Glob(filepath.Join(backupDir, "db.sqlite.*"))
	sort.Strings(entries)
	for _, old := range entries[:max(0, len(entries)-keep)] {
		if err := os.Remove(old); err != nil {
			Log.Warnw("failed to remove old backup", "path", old, "error", err)
		}
	}
	return nil
}

func InitAndMigrateDB() error {
	Log.Info("running DB migrations")
	return db.AutoMigrate(
		&Artifact{},
		&EmbeddingJob{},
		&Embedding{},
		&GlobalConfig{},
		&Record{},
		&ScannedCode{},
		&User{},
	)
}

type Model struct {
	ID        uint           `gorm:"primarykey"`
	CreatedAt time.Time      `json:"-"`
	UpdatedAt time.Time      `json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
