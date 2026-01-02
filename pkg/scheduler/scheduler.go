package scheduler

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
	"github.com/sirrobot01/dbnest/pkg/database"
	"github.com/sirrobot01/dbnest/pkg/storage"
)

// Scheduler handles automatic backup jobs and container status sync
type Scheduler struct {
	store    storage.Storage
	manager  *database.Manager
	cron     *cron.Cron
	mu       sync.RWMutex
	jobIDs   map[string]cron.EntryID // databaseID -> cronEntryID
	stopChan chan struct{}
	syncing  atomic.Bool // Guards against overlapping status sync runs
}

// New creates a new scheduler
func New(store storage.Storage, manager *database.Manager) *Scheduler {
	return &Scheduler{
		store:    store,
		manager:  manager,
		cron:     cron.New(cron.WithSeconds()),
		jobIDs:   make(map[string]cron.EntryID),
		stopChan: make(chan struct{}),
	}
}

// Start begins the scheduler and syncs database schedules
func (s *Scheduler) Start() error {
	log.Info().Msg("Starting scheduler")

	// Sync all database schedules
	if err := s.syncSchedules(); err != nil {
		return err
	}

	// Add container status sync job (every 10 seconds)
	if _, err := s.cron.AddFunc("@every 10s", s.syncContainerStatus); err != nil {
		return err
	}

	// Start cron
	s.cron.Start()

	// Run backup schedule sync loop (every 5 minutes)
	go s.syncLoop()

	// Do initial status sync
	go s.syncContainerStatus()

	return nil
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() {
	close(s.stopChan)
	ctx := s.cron.Stop()
	<-ctx.Done()
	log.Info().Msg("Scheduler stopped")
}

// syncLoop periodically syncs database backup schedules
func (s *Scheduler) syncLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.syncSchedules(); err != nil {
				log.Error().Err(err).Msg("Failed to sync backup schedules")
			}
		case <-s.stopChan:
			return
		}
	}
}

// syncContainerStatus queries all containers and updates status if changed
func (s *Scheduler) syncContainerStatus() {
	// Guard: skip if already running
	if !s.syncing.CompareAndSwap(false, true) {
		log.Debug().Msg("Status sync already in progress, skipping")
		return
	}
	defer s.syncing.Store(false)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s.manager.SyncAllStatuses(ctx)
}

// syncSchedules syncs the cron jobs with database backup settings
func (s *Scheduler) syncSchedules() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	databases := s.store.ListDatabases()

	// Track which databases have active schedules
	activeDBs := make(map[string]bool)

	for _, db := range databases {
		activeDBs[db.ID] = true

		if !db.BackupEnabled || db.BackupSchedule == "" {
			// Remove existing job if backup is disabled
			if entryID, exists := s.jobIDs[db.ID]; exists {
				s.cron.Remove(entryID)
				delete(s.jobIDs, db.ID)
				log.Debug().Str("db", db.ID).Msg("Removed backup schedule")
			}
			continue
		}

		// Check if schedule changed
		existingEntryID, exists := s.jobIDs[db.ID]
		if exists {
			entry := s.cron.Entry(existingEntryID)
			if entry.Valid() {
				// Job already exists, skip unless we need to update
				// (For simplicity, we always recreate - could optimize with schedule comparison)
				continue
			}
		}

		// Add new cron job
		dbID := db.ID // capture for closure
		schedule := db.BackupSchedule
		entryID, err := s.cron.AddFunc(schedule, func() {
			s.runBackup(dbID)
		})
		if err != nil {
			log.Error().Err(err).Str("db", db.ID).Str("schedule", schedule).Msg("Failed to add backup schedule")
			continue
		}

		s.jobIDs[db.ID] = entryID
		log.Info().Str("db", db.ID).Str("schedule", schedule).Msg("Added backup schedule")
	}

	// Remove jobs for deleted databases
	for dbID, entryID := range s.jobIDs {
		if !activeDBs[dbID] {
			s.cron.Remove(entryID)
			delete(s.jobIDs, dbID)
			log.Debug().Str("db", dbID).Msg("Removed orphaned backup schedule")
		}
	}

	return nil
}

// runBackup executes a backup for a database and applies retention policy
func (s *Scheduler) runBackup(databaseID string) {
	ctx := context.Background()
	log.Info().Str("db", databaseID).Msg("Running scheduled backup")

	// Get database to check if still enabled
	db, err := s.store.GetDatabase(databaseID)
	if err != nil {
		log.Error().Err(err).Str("db", databaseID).Msg("Failed to get database for backup")
		return
	}

	if !db.BackupEnabled {
		log.Debug().Str("db", databaseID).Msg("Backup disabled, skipping")
		return
	}

	if db.Status != "running" {
		log.Debug().Str("db", databaseID).Str("status", db.Status).Msg("Database not running, skipping backup")
		return
	}

	// Create backup
	backup, err := s.manager.CreateBackup(ctx, databaseID)
	if err != nil {
		log.Error().Err(err).Str("db", databaseID).Msg("Failed to create scheduled backup")
		return
	}

	log.Info().Str("db", databaseID).Str("backup", backup.ID).Msg("Scheduled backup created")

	// Update last backup time
	now := time.Now()
	db.LastBackupAt = &now
	if err := s.store.UpdateDatabase(db); err != nil {
		log.Error().Err(err).Str("db", databaseID).Msg("Failed to update last backup time")
	}

	// Apply retention policy
	go s.applyRetention(databaseID)
}

// applyRetention removes old backups beyond the retention count
func (s *Scheduler) applyRetention(databaseID string) {
	db, err := s.store.GetDatabase(databaseID)
	if err != nil || db.BackupRetentionCount <= 0 {
		return
	}

	backups := s.store.ListBackups(databaseID)
	if len(backups) <= db.BackupRetentionCount {
		return
	}

	// Sort by creation time (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	// Delete old backups beyond retention count
	for i := db.BackupRetentionCount; i < len(backups); i++ {
		backup := backups[i]
		if err := s.store.DeleteBackup(backup.ID); err != nil {
			log.Error().Err(err).Str("backup", backup.ID).Msg("Failed to delete old backup")
		} else {
			log.Debug().Str("backup", backup.ID).Str("db", databaseID).Msg("Deleted old backup (retention policy)")
		}
	}
}

// RefreshSchedule forces a refresh of a specific database's schedule
func (s *Scheduler) RefreshSchedule(databaseID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove existing job
	if entryID, exists := s.jobIDs[databaseID]; exists {
		s.cron.Remove(entryID)
		delete(s.jobIDs, databaseID)
	}

	// Get database
	db, err := s.store.GetDatabase(databaseID)
	if err != nil {
		return err
	}

	if !db.BackupEnabled || db.BackupSchedule == "" {
		return nil
	}

	// Add new job
	dbID := db.ID
	schedule := db.BackupSchedule
	entryID, err := s.cron.AddFunc(schedule, func() {
		s.runBackup(dbID)
	})
	if err != nil {
		return err
	}

	s.jobIDs[databaseID] = entryID
	log.Info().Str("db", databaseID).Str("schedule", schedule).Msg("Refreshed backup schedule")
	return nil
}
