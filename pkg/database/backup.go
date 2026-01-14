package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/sirrobot01/dbnest/pkg/storage"
)

// CreateBackup creates a backup of the database
func (m *Manager) CreateBackup(ctx context.Context, databaseID string) (*storage.Backup, error) {
	db, err := m.store.GetDatabase(databaseID)
	if err != nil {
		return nil, err
	}

	// Get engine for this database
	engine, err := GetEngine(db.Engine)
	if err != nil {
		return nil, fmt.Errorf("unsupported engine: %s", db.Engine)
	}

	backupID := "bk-" + uuid.New().String()[:8]
	backupDir := filepath.Join(m.store.DataDir(), "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	backupFile := filepath.Join(backupDir, fmt.Sprintf("%s-%s.dump", db.Name, backupID))

	// Create backup record
	backup := &storage.Backup{
		ID:           backupID,
		DatabaseID:   databaseID,
		DatabaseName: db.Name,
		CreatedAt:    time.Now(),
		Size:         0,
		Status:       "in-progress",
	}

	if err := m.store.CreateBackup(backup); err != nil {
		return nil, fmt.Errorf("failed to create backup record: %w", err)
	}

	// Run backup in background using the engine's Backup method
	go func() {
		log.Info().
			Str("id", backupID).
			Str("database", db.Name).
			Str("engine", db.Engine).
			Msg("Starting database backup")

		err := engine.Backup(context.Background(), m.client, db, backupFile)
		if err != nil {
			log.Error().
				Err(err).
				Str("id", backupID).
				Msg("Backup failed")

			backup.Status = "failed"
			m.store.UpdateBackup(backup)
			return
		}

		// Get file size
		if info, err := os.Stat(backupFile); err == nil {
			backup.Size = info.Size()
		}
		backup.FilePath = backupFile
		backup.Status = "completed"
		m.store.UpdateBackup(backup)

		log.Info().
			Str("id", backupID).
			Str("database", db.Name).
			Int64("size", backup.Size).
			Msg("Backup completed successfully")
	}()

	return backup, nil
}

// RestoreBackup restores a database from a backup
func (m *Manager) RestoreBackup(ctx context.Context, backupID, targetDatabaseID string) error {
	backup, err := m.store.GetBackup(backupID)
	if err != nil {
		return err
	}

	db, err := m.store.GetDatabase(targetDatabaseID)
	if err != nil {
		return err
	}

	// Get engine for this database
	engine, err := GetEngine(db.Engine)
	if err != nil {
		return fmt.Errorf("unsupported engine: %s", db.Engine)
	}

	log.Info().
		Str("backup_id", backupID).
		Str("database", db.Name).
		Str("engine", db.Engine).
		Msg("Starting database restore")

	// Use the engine's Restore method
	if err := engine.Restore(ctx, m.client, db, backup.FilePath); err != nil {
		log.Error().
			Err(err).
			Str("backup_id", backupID).
			Msg("Restore failed")
		return err
	}

	log.Info().
		Str("backup_id", backupID).
		Str("database", db.Name).
		Msg("Restore completed successfully")

	return nil
}
