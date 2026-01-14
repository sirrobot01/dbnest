package database

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/sirrobot01/dbnest/pkg/runtime"
	"github.com/sirrobot01/dbnest/pkg/storage"
)

// CreateRequest holds parameters for creating a database
type CreateRequest struct {
	Name         string `json:"name"`
	Engine       string `json:"engine"`
	Version      string `json:"version"`
	Username     string `json:"username"`
	Password     string `json:"password"` // Optional, auto-generated if empty
	Database     string `json:"database"`
	Port         int    `json:"port,omitempty"`
	StorageLimit int64  `json:"storageLimit"`         // MB
	MemoryLimit  int64  `json:"memoryLimit"`          // MB
	Network      string `json:"network,omitempty"`    // Docker network name
	ExposePort   *bool  `json:"exposePort,omitempty"` // Whether to expose port to host (default: true)

	// Restore from backup
	RestoreFromBackupID string `json:"restoreFromBackupId,omitempty"` // Optional backup to restore from

	// Data Seeding
	SeedSource  string `json:"seedSource,omitempty"`  // "none", "url", "file", "text"
	SeedContent string `json:"seedContent,omitempty"` // URL or raw SQL content
}

// Manager handles database operations
type Manager struct {
	store          storage.Storage
	client         runtime.Client // Interface type, not concrete
	portLock       sync.Mutex     // Protects port allocation
	metricsHistory *MetricsHistory
}

// validNameRegex matches alphanumeric names with underscores/hyphens
var validNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// sanitizeName validates and returns a safe name for SQL identifiers
func sanitizeName(name string) (string, error) {
	if len(name) < 1 || len(name) > 63 {
		return "", fmt.Errorf("name must be 1-63 characters")
	}
	if !validNameRegex.MatchString(name) {
		return "", fmt.Errorf("name must start with a letter and contain only alphanumeric, underscore, or hyphen")
	}
	return name, nil
}

// NewManager creates a new database manager
func NewManager(store storage.Storage, dockerClient runtime.Client) *Manager {
	return &Manager{
		store:          store,
		client:         dockerClient,
		metricsHistory: NewMetricsHistory(),
	}
}

// findAvailablePortLocked finds an available port starting from the given port
// Must be called with portLock held
func (m *Manager) findAvailablePortLocked(startPort int) int {
	usedPorts := make(map[int]bool)
	for _, db := range m.store.ListDatabases() {
		usedPorts[db.Port] = true
	}

	port := startPort
	maxAttempts := 1000 // Prevent infinite loop
	for i := 0; i < maxAttempts; i++ {
		// Skip if already used by another DBnest database
		if usedPorts[port] {
			port++
			continue
		}

		// Check if port is actually available on the host
		if isPortAvailable(port) {
			return port
		}

		port++
		if port > 65535 {
			port = startPort
		}
	}
	return port // Return anyway, container will fail with clear error
}

// isPortAvailable checks if a port is available on the host
func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// Create creates a new database instance
func (m *Manager) Create(ctx context.Context, req *CreateRequest) (*storage.DatabaseInstance, error) {
	// Auto-generate password if not provided
	if req.Password == "" {
		req.Password = uuid.New().String()[:16]
	}

	return m.createDedicatedDatabase(ctx, req)
}

// createDedicatedDatabase creates a database with its own container
// Returns immediately with status "creating", actual provisioning happens in background
func (m *Manager) createDedicatedDatabase(ctx context.Context, req *CreateRequest) (*storage.DatabaseInstance, error) {
	// Get engine from registry
	engine, err := GetEngine(req.Engine)
	if err != nil {
		return nil, fmt.Errorf("unsupported engine: %s", req.Engine)
	}

	// Generate ID
	id := "db-" + uuid.New().String()[:8]

	// Lock port allocation - keep lock until DB is saved to prevent race condition
	m.portLock.Lock()
	port := req.Port
	if port == 0 {
		port = m.findAvailablePortLocked(engine.DefaultPort())
	}

	// Create data directory with ABSOLUTE PATH
	baseDataDir, err := filepath.Abs(m.store.DataDir())
	if err != nil {
		m.portLock.Unlock()
		return nil, fmt.Errorf("failed to resolve data directory: %w", err)
	}
	dataDir := filepath.Join(baseDataDir, "databases", id)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		m.portLock.Unlock()
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Build image name with version
	imageName := engine.Image()
	if req.Version != "" {
		imageName = fmt.Sprintf("%s:%s", engine.Image(), req.Version)
	}

	// Create database record with "creating" status
	db := &storage.DatabaseInstance{
		ID:             id,
		Name:           req.Name,
		Engine:         req.Engine,
		Version:        req.Version,
		Status:         "creating",
		Host:           "localhost",
		Port:           port,
		Username:       req.Username,
		Password:       req.Password,
		Database:       req.Database,
		CreatedAt:      time.Now(),
		StorageUsed:    0,
		StorageLimit:   req.StorageLimit * 1024 * 1024, // Convert MB to bytes
		MemoryLimit:    req.MemoryLimit * 1024 * 1024,
		CPULimit:       1.0,
		Connections:    0,
		MaxConnections: 100,
		ExposePort:     req.ExposePort == nil || *req.ExposePort, // Default to true if not specified
		Network:        req.Network,
	}

	// Save to storage IMMEDIATELY (while still holding port lock)
	if err := m.store.CreateDatabase(db); err != nil {
		m.portLock.Unlock()
		return nil, fmt.Errorf("failed to save database: %w", err)
	}
	m.portLock.Unlock() // Now safe to release lock

	// Process container creation in background
	go m.provisionDedicatedDatabase(db, imageName, dataDir, port, engine, req.SeedSource, req.SeedContent)

	// Return immediately with "creating" status
	return db, nil
}

// provisionDedicatedDatabase runs in background to pull image and create/start container
func (m *Manager) provisionDedicatedDatabase(db *storage.DatabaseInstance, imageName, dataDir string, port int, engine Engine, seedSource, seedContent string) {
	ctx := context.Background()

	log.Info().
		Str("id", db.ID).
		Str("name", db.Name).
		Str("image", imageName).
		Int("port", port).
		Msg("Starting database provisioning")

	// Pull image (this can take a while for large images)
	log.Info().Str("id", db.ID).Str("image", imageName).Msg("Pulling Docker image (this may take a few minutes)...")
	if err := m.client.PullImage(ctx, imageName); err != nil {
		log.Error().Err(err).Str("id", db.ID).Str("image", imageName).Msg("Failed to pull image")
		db.Status = "error"
		db.ErrorMessage = fmt.Sprintf("Failed to pull image: %v", err)
		m.store.UpdateDatabase(db)
		return
	}
	log.Info().Str("id", db.ID).Str("image", imageName).Msg("Docker image pulled successfully")

	// Create container
	log.Info().Str("id", db.ID).Msg("Creating Docker container")
	containerCfg := &runtime.ContainerConfig{
		Name:  fmt.Sprintf("dbnest-%s", db.ID),
		Image: imageName,
		Cmd:   engine.ContainerCmd(db.Password),
		Env:   engine.EnvVars(db.Username, db.Password, db.Database),
		PortBindings: map[string]string{
			fmt.Sprintf("%d/tcp", engine.DefaultPort()): fmt.Sprintf("%d", port),
		},
		Volumes: map[string]string{
			fmt.Sprintf("dbnest-vol-%s", db.ID): engine.DataPath(),
		},
		MemoryLimit: db.MemoryLimit,
		CPULimit:    db.CPULimit,
		Labels: map[string]string{
			"dbnest.managed": "true",
			"dbnest.id":      db.ID,
		},
		ExposePort: db.ExposePort,
		Network:    db.Network,
	}

	containerID, err := m.client.CreateContainer(ctx, containerCfg)
	if err != nil {
		log.Error().Err(err).Str("id", db.ID).Msg("Failed to create container")
		db.Status = "error"
		db.ErrorMessage = fmt.Sprintf("Failed to create container: %v", err)
		m.store.UpdateDatabase(db)
		return
	}

	db.ContainerID = containerID
	log.Info().Str("id", db.ID).Str("container_id", containerID[:12]).Msg("Container created")

	// Start container
	log.Info().Str("id", db.ID).Msg("Starting container")
	if err := m.client.StartContainer(ctx, containerID); err != nil {
		log.Error().Err(err).Str("id", db.ID).Msg("Failed to start container")
		db.Status = "error"
		db.ErrorMessage = fmt.Sprintf("Failed to start container: %v", err)
		m.store.UpdateDatabase(db)
		return
	}

	db.Status = "running"
	db.ErrorMessage = "" // Clear any previous error
	m.store.UpdateDatabase(db)

	log.Info().
		Str("id", db.ID).
		Str("name", db.Name).
		Int("port", port).
		Msg("Database provisioned successfully")

	// Apply data seeding if requested
	if seedSource != "" && seedSource != "none" {
		go m.applySeed(db, seedSource, seedContent)
	}
}

// applySeed runs in background to apply data seeding
func (m *Manager) applySeed(db *storage.DatabaseInstance, source, content string) {
	ctx := context.Background()
	log.Info().Str("id", db.ID).Str("source", source).Msg("Starting data seeding")

	// Wait for database to be ready
	// We'll try to connect periodically
	maxRetries := 30
	ready := false
	engine, _ := GetEngine(db.Engine) // Error handled in caller

	for i := 0; i < maxRetries; i++ {
		// Use a simple health check query via Exec
		testQuery := "SELECT 1"
		if db.Engine == "redis" {
			testQuery = "PING"
		}

		// We use the engine's ExecuteQuery which internally uses Exec/ExecWithStdin
		_, err := engine.ExecuteQuery(ctx, m.client, db, testQuery)
		if err == nil {
			ready = true
			break
		}
		time.Sleep(2 * time.Second)
	}

	if !ready {
		log.Error().Str("id", db.ID).Msg("Database not ready for seeding after timeout")
		return
	}

	// Fetch content if URL
	var sqlContent string
	if source == "url" {
		// TODO: Fetch from URL (implement simple Get)
		// For now assuming content IS the URL, but we need to fetch it
		// We'll skip URL fetching implementation for this step to keep it simple or add it if needed
		// Let's assume content is passed directly for "text" or "file" (read by frontend)
		log.Warn().Str("id", db.ID).Msg("URL seeding not fully implemented yet on backend, expect content passed directly")
		sqlContent = content
	} else {
		sqlContent = content
	}

	if sqlContent == "" {
		log.Warn().Str("id", db.ID).Msg("Empty seed content")
		return
	}

	// Execute seed
	log.Info().Str("id", db.ID).Int("bytes", len(sqlContent)).Msg("Executing seed script")

	// We use ExecWithStdin to pipe the SQL to the cli tool
	// Need to construct the command mainly, ExecuteQuery does raw query string
	// But for large SQL dump, we want to pipe it.
	// Engine interface might need an `ExecuteScript` method, or we construct it here.

	cmd := engine.CLICommand(db.Username, db.Password, db.Database)
	// CLICommand returns something like ["psql", "-U", ...]
	// We need to inject the SQL via stdin

	output, err := m.client.ExecWithStdin(ctx, db.ContainerID, cmd, []byte(sqlContent), nil)
	if err != nil {
		log.Error().Err(err).Str("id", db.ID).Msg("Failed to execute seed script")
		// Ideally we should record this error somewhere visible to user
	} else {
		log.Info().Str("id", db.ID).Msg("Data seeding completed successfully")
		log.Debug().Str("id", db.ID).Str("output", output).Msg("Seed output")
	}
}

// Get retrieves a database by ID
func (m *Manager) Get(id string) (*storage.DatabaseInstance, error) {
	return m.store.GetDatabase(id)
}

// List returns all databases
func (m *Manager) List() []*storage.DatabaseInstance {
	return m.store.ListDatabases()
}

// SyncAllStatuses queries container runtime for actual status and updates any that differ.
// This is called by the background status sync worker.
func (m *Manager) SyncAllStatuses(ctx context.Context) {
	databases := m.store.ListDatabases()
	for _, db := range databases {
		m.syncStatus(ctx, db)
	}
}

// syncStatus queries the container runtime for actual container state and updates db.Status if needed
func (m *Manager) syncStatus(ctx context.Context, db *storage.DatabaseInstance) {
	// Skip if no container or still creating
	if db.ContainerID == "" || db.Status == "creating" {
		return
	}

	actualStatus, err := m.client.GetContainerStatus(ctx, db.ContainerID)
	if err != nil {
		// If we can't query and it was running, mark as error
		if db.Status == "running" {
			log.Debug().Err(err).Str("id", db.ID).Msg("Container not accessible")
			db.Status = "error"
			db.ErrorMessage = "Container not accessible"
			m.store.UpdateDatabase(db)
		}
		return
	}

	// If actual status differs from stored status, update it
	if actualStatus != db.Status {
		log.Info().
			Str("id", db.ID).
			Str("old_status", db.Status).
			Str("new_status", actualStatus).
			Msg("Container status changed externally")

		db.Status = actualStatus
		if actualStatus == "running" {
			db.ErrorMessage = ""
		}
		m.store.UpdateDatabase(db)
	}
}

// Start starts a stopped database
func (m *Manager) Start(ctx context.Context, id string) error {
	db, err := m.store.GetDatabase(id)
	if err != nil {
		return err
	}

	if db.ContainerID == "" {
		return fmt.Errorf("no container associated with database")
	}

	if err := m.client.StartContainer(ctx, db.ContainerID); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	db.Status = "running"
	return m.store.UpdateDatabase(db)
}

// Stop stops a running database
func (m *Manager) Stop(ctx context.Context, id string) error {
	db, err := m.store.GetDatabase(id)
	if err != nil {
		return err
	}

	if db.ContainerID == "" {
		return fmt.Errorf("no container associated with database")
	}

	if err := m.client.StopContainer(ctx, db.ContainerID); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	db.Status = "stopped"
	db.Connections = 0
	return m.store.UpdateDatabase(db)
}

// Delete deletes a database and its container
func (m *Manager) Delete(ctx context.Context, id string) error {
	db, err := m.store.GetDatabase(id)
	if err != nil {
		return err
	}

	// Remove container if exists
	if db.ContainerID != "" {
		if err := m.client.RemoveContainer(ctx, db.ContainerID, true); err != nil {
			fmt.Printf("Warning: failed to remove container: %v\n", err)
		}
	}

	// Remove volume
	volumeName := fmt.Sprintf("dbnest-vol-%s", id)
	if err := m.client.DeleteVolume(ctx, volumeName); err != nil {
		// Log but don't fail, volume might not exist
		fmt.Printf("Warning: failed to remove volume %s: %v\n", volumeName, err)
	}

	// Remove local data directory (if it exists)
	baseDataDir, _ := filepath.Abs(m.store.DataDir())
	dataDir := filepath.Join(baseDataDir, "databases", id)
	if err := os.RemoveAll(dataDir); err != nil {
		fmt.Printf("Warning: failed to remove data directory %s: %v\n", dataDir, err)
	}

	return m.store.DeleteDatabase(id)
}

// Clone creates a copy of an existing database
func (m *Manager) Clone(ctx context.Context, sourceID string, newName string) (*storage.DatabaseInstance, error) {
	// Get source database
	source, err := m.store.GetDatabase(sourceID)
	if err != nil {
		return nil, fmt.Errorf("source database not found: %w", err)
	}

	// Validate name
	if _, err := sanitizeName(newName); err != nil {
		return nil, fmt.Errorf("invalid name: %w", err)
	}

	// Create backup of source
	log.Info().Str("source", sourceID).Str("name", newName).Msg("Creating backup for clone")
	backup, err := m.CreateBackup(ctx, sourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup: %w", err)
	}

	// Wait for backup to complete (poll status)
	maxWait := 60 // seconds
	for i := 0; i < maxWait; i++ {
		backup, err = m.store.GetBackup(backup.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get backup status: %w", err)
		}
		if backup.Status == "completed" {
			break
		}
		if backup.Status == "failed" {
			return nil, fmt.Errorf("backup failed")
		}
		time.Sleep(time.Second)
	}

	if backup.Status != "completed" {
		return nil, fmt.Errorf("backup timed out")
	}

	// Create new database with same settings
	req := &CreateRequest{
		Name:                newName,
		Engine:              source.Engine,
		Version:             source.Version,
		Username:            source.Username,
		Password:            uuid.New().String()[:16], // New password
		Database:            source.Database,
		StorageLimit:        source.StorageLimit / (1024 * 1024), // Convert back to MB
		MemoryLimit:         source.MemoryLimit / (1024 * 1024),
		Network:             source.Network,
		RestoreFromBackupID: backup.ID,
	}

	log.Info().Str("name", newName).Str("backup", backup.ID).Msg("Creating cloned database")
	clone, err := m.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create clone: %w", err)
	}

	// Wait for container to be running then restore
	// Wait for database to be running
	containerWait := 120 // seconds
	for i := 0; i < containerWait; i++ {
		clone, err = m.store.GetDatabase(clone.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get clone status: %w", err)
		}
		if clone.Status == "running" {
			break
		}
		if clone.Status == "error" {
			return nil, fmt.Errorf("clone container failed: %s", clone.ErrorMessage)
		}
		time.Sleep(time.Second)
	}

	if clone.Status != "running" {
		return nil, fmt.Errorf("clone timed out waiting for container")
	}

	// Restore backup to clone
	log.Info().Str("clone", clone.ID).Str("backup", backup.ID).Msg("Restoring backup to clone")
	if err := m.RestoreBackup(ctx, backup.ID, clone.ID); err != nil {
		log.Warn().Err(err).Msg("Failed to restore backup to clone")
		// Don't fail - database was created, restore just didn't work
	}

	return clone, nil
}

// Repair attempts to fix a stuck database by recreating its container
func (m *Manager) Repair(ctx context.Context, id string) error {
	db, err := m.store.GetDatabase(id)
	if err != nil {
		return fmt.Errorf("database not found: %w", err)
	}

	log.Info().Str("id", id).Str("status", db.Status).Msg("Repairing database")

	// Try to remove existing container if any
	if db.ContainerID != "" {
		log.Debug().Str("container", db.ContainerID).Msg("Removing existing container")
		m.client.RemoveContainer(ctx, db.ContainerID, true)
	}

	// Get engine
	engine, err := GetEngine(db.Engine)
	if err != nil {
		return fmt.Errorf("unsupported engine: %w", err)
	}

	// Build image name
	imageName := engine.Image()
	if db.Version != "" {
		imageName = fmt.Sprintf("%s:%s", engine.Image(), db.Version)
	}

	// Get data directory
	baseDataDir, err := filepath.Abs(m.store.DataDir())
	if err != nil {
		return fmt.Errorf("failed to resolve data directory: %w", err)
	}
	dataDir := filepath.Join(baseDataDir, "databases", db.ID)

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Create new container
	containerCfg := &runtime.ContainerConfig{
		Name:  fmt.Sprintf("dbnest-%s", db.ID),
		Image: imageName,
		Cmd:   engine.ContainerCmd(db.Password),
		Env:   engine.EnvVars(db.Username, db.Password, db.Database),
		PortBindings: map[string]string{
			fmt.Sprintf("%d/tcp", engine.DefaultPort()): fmt.Sprintf("%d", db.Port),
		},
		Volumes: map[string]string{
			fmt.Sprintf("dbnest-vol-%s", db.ID): engine.DataPath(),
		},
		MemoryLimit: db.MemoryLimit,
		CPULimit:    db.CPULimit,
		Labels: map[string]string{
			"dbnest.managed": "true",
			"dbnest.id":      db.ID,
		},
		ExposePort: db.ExposePort,
		Network:    db.Network,
	}

	containerID, err := m.client.CreateContainer(ctx, containerCfg)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	db.ContainerID = containerID

	// Start container
	if err := m.client.StartContainer(ctx, containerID); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	db.Status = "running"
	db.ErrorMessage = ""
	return m.store.UpdateDatabase(db)
}

// GetMetricsHistory returns historical metrics for a database
func (m *Manager) GetMetricsHistory(dbID string) []MetricsPoint {
	return m.metricsHistory.Get(dbID)
}

// RecordMetrics records a metrics point for a database
func (m *Manager) RecordMetrics(dbID string, point MetricsPoint) {
	m.metricsHistory.Record(dbID, point)
}

// GetContainerStats returns stats for a container
func (m *Manager) GetContainerStats(ctx context.Context, containerID string) (*runtime.ContainerStats, error) {
	return m.client.GetContainerStats(ctx, containerID)
}

// GetLogs returns the logs for a database container
func (m *Manager) GetLogs(ctx context.Context, id string) (string, error) {
	db, err := m.store.GetDatabase(id)
	if err != nil {
		return "", err
	}

	if db.ContainerID == "" {
		return "", fmt.Errorf("no container associated with database")
	}

	return m.client.GetContainerLogs(ctx, db.ContainerID, 200) // Fetch last 200 lines
}

// UpdateResources updates the resource limits for a database
func (m *Manager) UpdateResources(ctx context.Context, id string, memoryLimit int64, cpuLimit float64) (*storage.DatabaseInstance, error) {
	db, err := m.store.GetDatabase(id)
	if err != nil {
		return nil, err
	}

	if memoryLimit > 0 {
		db.MemoryLimit = memoryLimit
	}
	if cpuLimit > 0 {
		db.CPULimit = cpuLimit
	}

	if err := m.store.UpdateDatabase(db); err != nil {
		return nil, err
	}
	return db, nil
}
