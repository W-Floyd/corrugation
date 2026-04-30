package backend

import (
	"context"
	"errors"
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

func InitAndMigrateDB() (err error) {
	Log.Info("running DB migrations")
	db.AutoMigrate(
		&Artifact{},
		&EmbeddingJob{},
		&Embedding{},
		&GlobalConfig{},
		&Record{},
		&User{},
	)
	return nil
}

type Model struct {
	ID        uint           `gorm:"primarykey"`
	CreatedAt time.Time      `json:"-"`
	UpdatedAt time.Time      `json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
