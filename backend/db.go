package backend

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
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

// BackupDB snapshots the database via VACUUM INTO, then compresses and prunes
// old backups in the background. keep=0 disables. Must be called after ConnectDB.
func BackupDB(backupDir string, keep int) error {
	if keep == 0 {
		return nil
	}
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}

	name := fmt.Sprintf("db.sqlite.%s", time.Now().Format("20060102-150405"))
	tmp := filepath.Join(backupDir, name)
	dest := tmp + ".gz"

	Log.Infow("backing up database", "dest", dest)
	if err := db.Exec("VACUUM INTO ?", tmp).Error; err != nil {
		return fmt.Errorf("vacuum into: %w", err)
	}

	go func() {
		if err := gzipFile(tmp, dest); err != nil {
			Log.Warnw("failed to compress backup", "dest", dest, "error", err)
			os.Remove(tmp)
			return
		}
		os.Remove(tmp)
		entries, _ := filepath.Glob(filepath.Join(backupDir, "db.sqlite.*.gz"))
		sort.Strings(entries)
		for _, old := range entries[:max(0, len(entries)-keep)] {
			if err := os.Remove(old); err != nil {
				Log.Warnw("failed to remove old backup", "path", old, "error", err)
			}
		}
	}()
	return nil
}

func gzipFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	gz, err := gzip.NewWriterLevel(out, gzip.BestCompression)
	if err != nil {
		return err
	}
	defer gz.Close()

	_, err = io.Copy(gz, in)
	return err
}

// BackfillLegacyEmbeddingsOnStart deletes any JSON-encoded embedding rows so
// they are regenerated in binary format by the normal backfill process.
func BackfillLegacyEmbeddingsOnStart() error {
	result := db.Unscoped().Where("substr(data, 1, 1) = X'5B'").Delete(&Embedding{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		Log.Infow("purged legacy JSON embeddings for backfill", "count", result.RowsAffected)
	}
	return nil
}

func InitAndMigrateDB() error {
	Log.Info("running DB migrations")
	return db.AutoMigrate(
		&Artifact{},
		&ArtifactSuggestion{},
		&EmbeddingJob{},
		&Embedding{},
		&GlobalConfig{},
		&Record{},
		&ScannedCode{},
		&SuggestionJob{},
		&User{},
	)
}

type Model struct {
	ID        uint           `gorm:"primarykey"`
	CreatedAt time.Time      `json:"-"`
	UpdatedAt time.Time      `json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
