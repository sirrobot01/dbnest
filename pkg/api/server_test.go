package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirrobot01/dbnest/pkg/database"
	"github.com/sirrobot01/dbnest/pkg/runtime"
	"github.com/sirrobot01/dbnest/pkg/storage"
)

// MockDockerClient implements runtime.Client for testing
type MockDockerClient struct{}

func (m *MockDockerClient) Close() error                                          { return nil }
func (m *MockDockerClient) Ping(ctx context.Context) error                        { return nil }
func (m *MockDockerClient) PullImage(ctx context.Context, imageName string) error { return nil }
func (m *MockDockerClient) CreateContainer(ctx context.Context, cfg *runtime.ContainerConfig) (string, error) {
	return "test-container-id", nil
}
func (m *MockDockerClient) StartContainer(ctx context.Context, id string) error { return nil }
func (m *MockDockerClient) StopContainer(ctx context.Context, id string) error  { return nil }
func (m *MockDockerClient) RemoveContainer(ctx context.Context, id string, force bool) error {
	return nil
}
func (m *MockDockerClient) GetContainerStatus(ctx context.Context, id string) (string, error) {
	return "running", nil
}
func (m *MockDockerClient) GetContainerStats(ctx context.Context, id string) (*runtime.ContainerStats, error) {
	return &runtime.ContainerStats{}, nil
}
func (m *MockDockerClient) GetContainerLogs(ctx context.Context, id string, tail int) (string, error) {
	return "test logs", nil
}
func (m *MockDockerClient) ListContainers(ctx context.Context) ([]string, error) {
	return []string{}, nil
}
func (m *MockDockerClient) ListNetworks(ctx context.Context) ([]runtime.NetworkInfo, error) {
	return []runtime.NetworkInfo{}, nil
}
func (m *MockDockerClient) CreateNetwork(ctx context.Context, name string) (*runtime.NetworkInfo, error) {
	return &runtime.NetworkInfo{ID: "test-net", Name: name}, nil
}
func (m *MockDockerClient) DeleteNetwork(ctx context.Context, id string) error { return nil }
func (m *MockDockerClient) ExecInContainer(ctx context.Context, id string, cmd []string) (string, error) {
	return "", nil
}
func (m *MockDockerClient) Exec(ctx context.Context, id string, cmd []string, env []string) (string, error) {
	return "", nil
}
func (m *MockDockerClient) ExecWithStdin(ctx context.Context, id string, cmd []string, stdin []byte, env []string) (string, error) {
	return "", nil
}
func (m *MockDockerClient) UpdateContainerResources(ctx context.Context, id string, memoryLimit int64, cpuLimit float64) error {
	return nil
}
func (m *MockDockerClient) DeleteVolume(ctx context.Context, name string) error { return nil }

func setupTestServer(t *testing.T) (*Server, http.Handler, string, func()) {
	t.Helper()

	// Create temp storage
	tmpDir := t.TempDir()
	store, err := storage.New(tmpDir+"/test.db", tmpDir)
	if err != nil {
		t.Fatalf("failed to create test storage: %v", err)
	}

	server := NewServer(database.NewManager(store, &MockDockerClient{}), store, &MockDockerClient{})
	handler := server.Handler()

	// Create test user and session to generate token
	userID := "test-user-id"
	token := "test-token"
	
	user := &storage.User{
		ID: userID,
		Username: "testadmin",
		CreatedAt: time.Now(),
	}
	if err := store.CreateUser(user); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	
	session := &storage.Session{
		ID: "test-session-id",
		UserID: userID,
		Token: token,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		CreatedAt: time.Now(),
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("failed to create test session: %v", err)
	}

	cleanup := func() {
		store.Close()
	}

	return server, handler, token, cleanup
}

func TestHealthEndpoint(t *testing.T) {
	_, handler, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got '%v'", response["status"])
	}
}

func TestListDatabasesEmpty(t *testing.T) {
	_, handler, token, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/databases", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var databases []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &databases); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(databases) != 0 {
		t.Errorf("expected empty list, got %d items", len(databases))
	}
}

func TestAuthStatus(t *testing.T) {
	_, handler, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/auth/status", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	enabled, ok := response["enabled"].(bool)
	if !ok {
		t.Fatal("expected 'enabled' field in response")
	}

	if !enabled {
		t.Error("expected auth to be enabled")
	}
}

func TestCreateDatabaseValidation(t *testing.T) {
	_, handler, token, cleanup := setupTestServer(t)
	defer cleanup()

	tests := []struct {
		name           string
		body           map[string]interface{}
		expectedStatus int
	}{
		{
			name:           "empty body",
			body:           map[string]interface{}{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing engine",
			body: map[string]interface{}{
				"name": "test-db",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid engine",
			body: map[string]interface{}{
				"name":   "test-db",
				"engine": "mongodb",
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			req := httptest.NewRequest("POST", "/api/v1/databases", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d: %s", tc.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestDatabaseNotFound(t *testing.T) {
	_, handler, token, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/databases/nonexistent-id", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestListBackupsEmpty(t *testing.T) {
	_, handler, token, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/backups", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// Helper for creating test databases in storage
func createTestDatabase(t *testing.T, store storage.Storage, name string) *storage.DatabaseInstance {
	t.Helper()

	db := &storage.DatabaseInstance{
		ID:           "test-" + name,
		Name:         name,
		Engine:       "postgresql",
		Version:      "16",
		Status:       "running",
		Host:         "localhost",
		Port:         5432,
		Username:     "testuser",
		Database:     name,
		ContainerID:  "test-container-id",
		CreatedAt:    time.Now(),
		StorageUsed:  0,
		StorageLimit: 1073741824,
		MemoryLimit:  536870912,
		CPULimit:     1,
	}

	if err := store.CreateDatabase(db); err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	return db
}

func TestGetDatabaseFound(t *testing.T) {
	server, handler, token, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a test database directly in storage
	db := createTestDatabase(t, server.store, "testdb")

	req := httptest.NewRequest("GET", "/api/v1/databases/"+db.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["name"] != "testdb" {
		t.Errorf("expected name 'testdb', got '%v'", response["name"])
	}
}

func TestGetLogs(t *testing.T) {
	server, handler, token, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a test database directly in storage
	db := createTestDatabase(t, server.store, "logsdb")

	req := httptest.NewRequest("GET", "/api/v1/databases/"+db.ID+"/logs", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	logs, ok := response["logs"].(string)
	if !ok {
		t.Fatal("expected 'logs' field in response")
	}

	if logs != "test logs" {
		t.Errorf("expected logs 'test logs', got '%s'", logs)
	}
}
