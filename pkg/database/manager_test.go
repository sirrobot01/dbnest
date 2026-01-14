package database

import (
	"context"
	"testing"
	"time"

	"github.com/sirrobot01/dbnest/pkg/runtime"
	"github.com/sirrobot01/dbnest/pkg/storage"
)

// MockDockerClient implements runtime.Client for testing
type MockDockerClient struct {
	LastContainerID string
	LastExecCmd     []string
	LastExecInput   string
}

func (m *MockDockerClient) Close() error { return nil }
func (m *MockDockerClient) Ping(ctx context.Context) error { return nil }
func (m *MockDockerClient) PullImage(ctx context.Context, imageName string) error { return nil }
func (m *MockDockerClient) CreateContainer(ctx context.Context, cfg *runtime.ContainerConfig) (string, error) {
	m.LastContainerID = "test-container-id"
	return "test-container-id", nil
}
func (m *MockDockerClient) StartContainer(ctx context.Context, id string) error { return nil }
func (m *MockDockerClient) StopContainer(ctx context.Context, id string) error { return nil }
func (m *MockDockerClient) RemoveContainer(ctx context.Context, id string, force bool) error { return nil }
func (m *MockDockerClient) GetContainerStatus(ctx context.Context, id string) (string, error) { return "running", nil }
func (m *MockDockerClient) GetContainerStats(ctx context.Context, id string) (*runtime.ContainerStats, error) {
	return &runtime.ContainerStats{}, nil
}
func (m *MockDockerClient) GetContainerLogs(ctx context.Context, id string, tail int) (string, error) {
	return "test logs", nil
}
func (m *MockDockerClient) ListContainers(ctx context.Context) ([]string, error) { return []string{}, nil }
func (m *MockDockerClient) ListNetworks(ctx context.Context) ([]runtime.NetworkInfo, error) { return []runtime.NetworkInfo{}, nil }
func (m *MockDockerClient) CreateNetwork(ctx context.Context, name string) (*runtime.NetworkInfo, error) {
	return &runtime.NetworkInfo{ID: "test-net", Name: name}, nil
}
func (m *MockDockerClient) DeleteNetwork(ctx context.Context, id string) error { return nil }
func (m *MockDockerClient) ExecInContainer(ctx context.Context, id string, cmd []string) (string, error) { return "", nil }
func (m *MockDockerClient) Exec(ctx context.Context, id string, cmd []string, env []string) (string, error) { return "", nil }
func (m *MockDockerClient) ExecWithStdin(ctx context.Context, id string, cmd []string, stdin []byte, env []string) (string, error) {
	m.LastExecCmd = cmd
	m.LastExecInput = string(stdin)
	return "", nil
}
func (m *MockDockerClient) UpdateContainerResources(ctx context.Context, id string, memoryLimit int64, cpuLimit float64) error { return nil }
func (m *MockDockerClient) DeleteVolume(ctx context.Context, name string) error { return nil }


func setupTestManager(t *testing.T) (*Manager, *storage.BoltStorage, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	store, err := storage.NewBoltStorage(tmpDir+"/test.db", tmpDir)
	if err != nil {
		t.Fatalf("failed to create test storage: %v", err)
	}

	mockDocker := &MockDockerClient{}
	manager := NewManager(store, mockDocker)

	cleanup := func() {
		store.Close()
	}

	return manager, store, cleanup
}

func TestCreateDatabase(t *testing.T) {
	manager, store, cleanup := setupTestManager(t)
	defer cleanup()

	req := &CreateRequest{
		Name:         "test-db",
		Engine:       "postgresql",
		Version:      "16",
		Username:     "admin",
		Database:     "test",
		StorageLimit: 1024,
		MemoryLimit:  512,
	}

	db, err := manager.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	// Capture values immediately (before background goroutine modifies them)
	name := db.Name
	dbID := db.ID

	if name != req.Name {
		t.Errorf("expected name %s, got %s", req.Name, name)
	}

	// Wait for background provisioning to complete
	time.Sleep(100 * time.Millisecond)

	// Re-fetch from storage to check final state (avoids race with goroutine)
	dbFromStore, err := store.GetDatabase(dbID)
	if err != nil {
		t.Fatalf("failed to get database from store: %v", err)
	}

	if dbFromStore.Status != "running" {
		t.Errorf("expected status running after provisioning, got %s", dbFromStore.Status)
	}
}

func TestGetLogs(t *testing.T) {
	manager, store, cleanup := setupTestManager(t)
	defer cleanup()

	// Create a mock database in storage
	db := &storage.DatabaseInstance{
		ID:          "test-id",
		Name:        "test-db",
		Engine:      "postgresql",
		ContainerID: "test-container-id",
		Status:      "running",
		CreatedAt:   time.Now(),
	}
	if err := store.CreateDatabase(db); err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	logs, err := manager.GetLogs(context.Background(), "test-id")
	if err != nil {
		t.Fatalf("failed to get logs: %v", err)
	}

	if logs != "test logs" {
		t.Errorf("expected logs 'test logs', got '%s'", logs)
	}
}

func TestGeneratePassword(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	req := &CreateRequest{
		Name:         "test-db-auto-pass",
		Engine:       "postgresql",
		Username:     "admin",
		Database:     "test",
		StorageLimit: 1024,
		MemoryLimit:  512,
		// Password omitted
	}

	db, err := manager.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	if db.Password == "" {
		t.Error("expected auto-generated password, got empty string")
	}
}

func TestUpdateResources(t *testing.T) {
	manager, store, cleanup := setupTestManager(t)
	defer cleanup()

	db := &storage.DatabaseInstance{
		ID:          "test-update",
		Name:        "test-update-db",
		Engine:      "postgresql",
		MemoryLimit: 1024,
		CPULimit:    1.0,
		ContainerID: "test-container-id",
	}
	store.CreateDatabase(db)

	updatedDb, err := manager.UpdateResources(context.Background(), "test-update", 2048, 2.0)
	if err != nil {
		t.Fatalf("failed to update resources: %v", err)
	}

	if updatedDb.MemoryLimit != 2048 {
		t.Errorf("expected memory limit 2048, got %d", updatedDb.MemoryLimit)
	}
	if updatedDb.CPULimit != 2.0 {
		t.Errorf("expected cpu limit 2.0, got %f", updatedDb.CPULimit)
	}
}

func TestSeeding(t *testing.T) {
	manager, store, cleanup := setupTestManager(t)
	defer cleanup()
	
	// Access the mock client to check calls
	// We need to verify that we are using the same instance as Manager
	// The setupTestManager creates a new MockDockerClient locally but copies it by value? 
	// No, it passes pointer &MockDockerClient{}. But we need to keep a reference.
	// We need to modify setupTestManager to return the mock client too.
	
	// Re-implement setup here to get handle on mock
	tmpDir := t.TempDir()
	store, _ = storage.NewBoltStorage(tmpDir+"/test.db", tmpDir)
	mockDocker := &MockDockerClient{}
	manager = NewManager(store, mockDocker)
	defer store.Close()

	db := &storage.DatabaseInstance{
		ID:          "seed-test-id",
		Name:        "seed-test-db",
		Engine:      "postgresql",
		Username:    "testuser",
		Database:    "testdb",
		ContainerID: "test-container-id",
		Status:      "running",
	}

	seedContent := "INSERT INTO users VALUES (1);"
	
	// Executing applySeed directly (it's unexported but we are in package database)
	// It should succeed immediately because MockDockerClient.Exec returns nil error
	manager.applySeed(db, "text", seedContent)

	if mockDocker.LastExecInput != seedContent {
		t.Errorf("expected seed content '%s', got '%s'", seedContent, mockDocker.LastExecInput)
	}
	
	// Check psql command structure
	// Expected: psql -U testuser -d testdb -f -
	expectedCmdLen := 7 // psql, -U, user, -d, db, -f, -
	if len(mockDocker.LastExecCmd) != expectedCmdLen {
		t.Errorf("expected command lenth %d, got %d: %v", expectedCmdLen, len(mockDocker.LastExecCmd), mockDocker.LastExecCmd)
	}
	if mockDocker.LastExecCmd[0] != "psql" {
		t.Errorf("expected command psql, got %s", mockDocker.LastExecCmd[0])
	}
}

func TestEngineCLICommands(t *testing.T) {
	tests := []struct {
		engine string
		expect []string
	}{
		{"postgresql", []string{"psql", "-U", "u", "-d", "d", "-f", "-"}},
		{"mysql", []string{"mysql", "-u", "u", "-pp", "d"}},
		{"mariadb", []string{"mariadb", "-u", "u", "-pp", "d"}},
		{"redis", []string{"redis-cli", "-a", "p", "--pipe"}},
	}

	for _, tc := range tests {
		e, err := GetEngine(tc.engine)
		if err != nil {
			t.Errorf("failed to get engine %s: %v", tc.engine, err)
			continue
		}
		
		cmd := e.CLICommand("u", "p", "d")
		
		if len(cmd) != len(tc.expect) {
			t.Errorf("[%s] expected len %d, got %d: %v", tc.engine, len(tc.expect), len(cmd), cmd)
			continue
		}
		
		for i := range cmd {
			if cmd[i] != tc.expect[i] {
				t.Errorf("[%s] arg %d: expected %s, got %s", tc.engine, i, tc.expect[i], cmd[i])
			}
		}
	}
}
