package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
	"github.com/sirrobot01/dbnest/pkg/auth"
	"github.com/sirrobot01/dbnest/pkg/database"
	"github.com/sirrobot01/dbnest/pkg/runtime"
	"github.com/sirrobot01/dbnest/pkg/storage"
)

// Server handles API requests
type Server struct {
	db     *database.Manager
	store  storage.Storage
	docker runtime.Client
}

// contextKey is a custom type for context keys
type contextKey string

const userContextKey contextKey = "user"

// NewServer creates a new API server
func NewServer(db *database.Manager, store storage.Storage, dockerClient runtime.Client) *Server {
	return &Server{
		db:     db,
		store:  store,
		docker: dockerClient,
	}
}

// Handler returns a handler for all API routes
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public routes (no auth required)
		r.Get("/health", s.handleHealthCheck)

		// Auth routes (always accessible)
		r.Route("/auth", func(r chi.Router) {
			r.Get("/status", s.handleAuthStatus)
			r.Post("/register", s.handleRegister)
			r.Post("/login", s.handleLogin)
			r.Post("/logout", s.handleLogout)
			r.Get("/me", s.handleGetCurrentUser)
		})

		// Protected routes (auth middleware when enabled)
		r.Group(func(r chi.Router) {
			// Apply auth middleware if auth is enabled
			r.Use(s.authMiddleware)

			// Database routes
			r.Route("/databases", func(r chi.Router) {
				r.Get("/", s.handleListDatabases)
				r.Post("/", s.handleCreateDatabase)
				r.Get("/{id}", s.handleGetDatabase)
				r.Delete("/{id}", s.handleDeleteDatabase)
				r.Post("/{id}/start", s.handleStartDatabase)
				r.Post("/{id}/stop", s.handleStopDatabase)
				r.Post("/{id}/backup", s.handleCreateBackup)
				r.Post("/{id}/restore", s.handleRestoreBackup)
				r.Get("/{id}/metrics", s.handleGetMetrics)
				r.Get("/{id}/metrics/history", s.handleGetMetricsHistory)
				r.Get("/{id}/health", s.handleHealthCheckDatabase)
				// Credentials and connection strings
				r.Get("/{id}/credentials", s.handleGetCredentials)
				r.Get("/{id}/connection-strings", s.handleGetConnectionStrings)
				r.Get("/{id}/logs", s.handleGetLogs)
				// Backup settings for scheduler
				r.Put("/{id}/backup-settings", s.handleUpdateBackupSettings)
				// Upscale/downscale resources
				r.Patch("/{id}/resources", s.handleUpdateResources)
			})

			// Bulk operations
			r.Route("/databases/bulk", func(r chi.Router) {
				r.Post("/start", s.handleBulkStart)
				r.Post("/stop", s.handleBulkStop)
				r.Post("/delete", s.handleBulkDelete)
			})

			// Backup routes
			r.Get("/backups", s.handleListBackups)
			r.Get("/backups/{id}/download", s.handleDownloadBackup)
			r.Get("/backups/{id}/info", s.handleGetBackupInfo)
			r.Delete("/backups/{id}", s.handleDeleteBackup)

			// Network routes
			r.Get("/networks", s.handleListNetworks)
			r.Post("/networks", s.handleCreateNetwork)
			r.Delete("/networks/{name}", s.handleDeleteNetwork)

			// Topology route
			r.Get("/topology", s.handleGetTopology)
		})
	})

	return r
}

// Response helpers
func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func errorResponse(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]string{"error": message})
}

// Health check handler
func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "healthy",
		"version": "1.0.0",
	})
}

// Database handlers

func (s *Server) handleListDatabases(w http.ResponseWriter, r *http.Request) {
	databases := s.db.List()
	jsonResponse(w, http.StatusOK, databases)
}

func (s *Server) handleCreateDatabase(w http.ResponseWriter, r *http.Request) {
	var req database.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validation
	if req.Name == "" {
		errorResponse(w, http.StatusBadRequest, "Name is required")
		return
	}
	if req.Engine == "" {
		errorResponse(w, http.StatusBadRequest, "Engine is required")
		return
	}

	// Username and database are always required (password is optional - auto-generated if empty)
	if req.Username == "" {
		errorResponse(w, http.StatusBadRequest, "Username is required")
		return
	}
	if req.Database == "" {
		errorResponse(w, http.StatusBadRequest, "Database name is required")
		return
	}

	db, err := s.db.Create(r.Context(), &req)
	if err != nil {
		log.Error().Err(err).Str("name", req.Name).Str("engine", req.Engine).Msg("Failed to create database")
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	log.Info().Str("id", db.ID).Str("name", db.Name).Str("engine", db.Engine).Msg("Database creation initiated")
	jsonResponse(w, http.StatusCreated, db)
}

func (s *Server) handleGetDatabase(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		errorResponse(w, http.StatusBadRequest, "Database ID is required")
		return
	}

	db, err := s.db.Get(id)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "Database not found")
		return
	}

	jsonResponse(w, http.StatusOK, db)
}

func (s *Server) handleDeleteDatabase(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		errorResponse(w, http.StatusBadRequest, "Database ID is required")
		return
	}

	if err := s.db.Delete(r.Context(), id); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStartDatabase(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		errorResponse(w, http.StatusBadRequest, "Database ID is required")
		return
	}

	if err := s.db.Start(r.Context(), id); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	db, _ := s.db.Get(id)
	jsonResponse(w, http.StatusOK, db)
}

func (s *Server) handleStopDatabase(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		errorResponse(w, http.StatusBadRequest, "Database ID is required")
		return
	}

	if err := s.db.Stop(r.Context(), id); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	db, _ := s.db.Get(id)
	jsonResponse(w, http.StatusOK, db)
}

func (s *Server) handleCreateBackup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		errorResponse(w, http.StatusBadRequest, "Database ID is required")
		return
	}

	backup, err := s.db.CreateBackup(r.Context(), id)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusAccepted, backup)
}

func (s *Server) handleRestoreBackup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		errorResponse(w, http.StatusBadRequest, "Database ID is required")
		return
	}

	var req struct {
		BackupID string `json:"backupId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.BackupID == "" {
		errorResponse(w, http.StatusBadRequest, "Backup ID is required")
		return
	}

	if err := s.db.RestoreBackup(r.Context(), req.BackupID, id); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "restored"})
}

func (s *Server) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		errorResponse(w, http.StatusBadRequest, "Database ID is required")
		return
	}

	db, err := s.db.Get(id)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "Database not found")
		return
	}

	// All databases are dedicated now - get container stats
	if db.ContainerID == "" {
		errorResponse(w, http.StatusBadRequest, "Database has no container")
		return
	}

	stats, err := s.db.GetContainerStats(r.Context(), db.ContainerID)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Record metrics for history
	s.db.RecordMetrics(id, database.MetricsPoint{
		Timestamp:     time.Now(),
		CPUPercent:    stats.CPUPercent,
		MemoryUsage:   stats.MemoryUsage,
		MemoryLimit:   stats.MemoryLimit,
		MemoryPercent: stats.MemoryPercent,
		StorageUsed:   db.StorageUsed,
		Connections:   db.Connections,
		NetworkRx:     stats.NetworkRx,
		NetworkTx:     stats.NetworkTx,
	})

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"cpuPercent":    stats.CPUPercent,
		"memoryUsage":   stats.MemoryUsage,
		"memoryLimit":   stats.MemoryLimit,
		"memoryPercent": stats.MemoryPercent,
		"networkRx":     stats.NetworkRx,
		"networkTx":     stats.NetworkTx,
		"storageUsed":   db.StorageUsed,
		"connections":   db.Connections,
	})
}

func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		errorResponse(w, http.StatusBadRequest, "Database ID is required")
		return
	}

	logs, err := s.db.GetLogs(r.Context(), id)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"logs": logs})
}

// Backup handlers

func (s *Server) handleListBackups(w http.ResponseWriter, r *http.Request) {
	databaseID := r.URL.Query().Get("databaseId")
	backups := s.store.ListBackups(databaseID)
	jsonResponse(w, http.StatusOK, backups)
}

func (s *Server) handleDownloadBackup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		errorResponse(w, http.StatusBadRequest, "Backup ID is required")
		return
	}

	backup, err := s.store.GetBackup(id)
	if err != nil || backup == nil {
		errorResponse(w, http.StatusNotFound, "Backup not found")
		return
	}

	// Get backup file path
	backupPath := s.store.GetBackupPath(id)
	if backupPath == "" {
		errorResponse(w, http.StatusNotFound, "Backup file not found")
		return
	}

	// Set headers for download
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-%s.backup", backup.DatabaseName, backup.ID))

	http.ServeFile(w, r, backupPath)
}

// handleListNetworks returns all available Docker networks
func (s *Server) handleListNetworks(w http.ResponseWriter, r *http.Request) {
	if s.docker == nil {
		jsonResponse(w, http.StatusOK, []interface{}{})
		return
	}

	networks, err := s.docker.ListNetworks(r.Context())
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, networks)
}

// handleCreateNetwork creates a new Docker network
func (s *Server) handleCreateNetwork(w http.ResponseWriter, r *http.Request) {
	if s.docker == nil {
		errorResponse(w, http.StatusInternalServerError, "Docker not available")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		errorResponse(w, http.StatusBadRequest, "Network name is required")
		return
	}

	// Prefix with dbnest-
	networkName := "dbnest-" + req.Name

	network, err := s.docker.CreateNetwork(r.Context(), networkName)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusCreated, network)
}

// handleDeleteNetwork deletes a Docker network
func (s *Server) handleDeleteNetwork(w http.ResponseWriter, r *http.Request) {
	if s.docker == nil {
		errorResponse(w, http.StatusInternalServerError, "Docker not available")
		return
	}

	name := chi.URLParam(r, "name")
	if name == "" {
		errorResponse(w, http.StatusBadRequest, "Network name is required")
		return
	}

	if err := s.docker.DeleteNetwork(r.Context(), name); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// TopologyNode represents a database in the topology
type TopologyNode struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Engine  string `json:"engine"`
	Status  string `json:"status"`
	Network string `json:"network"`
}

// TopologyNetwork represents a network with its databases
type TopologyNetwork struct {
	Name      string         `json:"name"`
	Databases []TopologyNode `json:"databases"`
}

// handleGetTopology returns network topology for visualization
func (s *Server) handleGetTopology(w http.ResponseWriter, r *http.Request) {
	databases := s.store.ListDatabases()

	// Group databases by network
	networkMap := make(map[string][]TopologyNode)

	for _, db := range databases {
		networkName := db.Network
		if networkName == "" {
			networkName = "default"
		}

		node := TopologyNode{
			ID:      db.ID,
			Name:    db.Name,
			Engine:  db.Engine,
			Status:  db.Status,
			Network: networkName,
		}

		networkMap[networkName] = append(networkMap[networkName], node)
	}

	// Convert to slice
	var topology []TopologyNetwork
	for name, dbs := range networkMap {
		topology = append(topology, TopologyNetwork{
			Name:      name,
			Databases: dbs,
		})
	}

	jsonResponse(w, http.StatusOK, topology)
}

func (s *Server) handleHealthCheckDatabase(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		errorResponse(w, http.StatusBadRequest, "Database ID is required")
		return
	}

	db, err := s.db.Get(id)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "Database not found")
		return
	}

	health := map[string]interface{}{
		"status":      db.Status,
		"healthy":     db.Status == "running",
		"containerId": db.ContainerID,
		"engine":      db.Engine,
		"host":        db.Host,
		"port":        db.Port,
	}

	// If running, try to check actual connectivity
	if db.Status == "running" && db.ContainerID != "" {
		// Get engine and run a simple health query
		engine, err := database.GetEngine(db.Engine)
		if err == nil {
			var testQuery string
			switch db.Engine {
			case "postgresql":
				testQuery = "SELECT 1"
			case "mysql", "mariadb":
				testQuery = "SELECT 1"
			case "redis":
				testQuery = "PING"
			}

			if testQuery != "" {
				result, err := engine.ExecuteQuery(r.Context(), s.docker, db, testQuery)
				if err != nil || (result != nil && result.Error != "") {
					health["healthy"] = false
					health["connectionError"] = "Failed to execute health check query"
				} else {
					health["connectionVerified"] = true
				}
			}
		}
	}

	jsonResponse(w, http.StatusOK, health)
}

// Auth middleware

// authMiddleware checks for valid session token and adds user to context
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to get token from Authorization header first
		token := ""
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}

		// Fall back to cookie
		if token == "" {
			cookie, err := r.Cookie("session")
			if err == nil {
				token = cookie.Value
			}
		}

		if token == "" {
			errorResponse(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		// Validate session
		session, err := s.store.GetSessionByToken(token)
		if err != nil {
			errorResponse(w, http.StatusUnauthorized, "Invalid session")
			return
		}

		// Check if session expired
		if time.Now().After(session.ExpiresAt) {
			s.store.DeleteSession(session.ID)
			errorResponse(w, http.StatusUnauthorized, "Session expired")
			return
		}

		// Get user
		user, err := s.store.GetUser(session.UserID)
		if err != nil {
			errorResponse(w, http.StatusUnauthorized, "User not found")
			return
		}

		// Add user to context
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Auth handlers

// handleAuthStatus returns auth configuration status
func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"enabled":    true,
		"configured": s.store.UserCount() > 0,
	})
}

// handleRegister creates the first user (only works when no users exist)
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	// Registration only works if no users exist yet
	if s.store.UserCount() > 0 {
		errorResponse(w, http.StatusForbidden, "Registration closed. Users already exist.")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Username == "" {
		errorResponse(w, http.StatusBadRequest, "Username is required")
		return
	}
	if req.Password == "" {
		errorResponse(w, http.StatusBadRequest, "Password is required")
		return
	}
	if len(req.Password) < 8 {
		errorResponse(w, http.StatusBadRequest, "Password must be at least 8 characters")
		return
	}

	// Hash password
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	// Create user
	user := &storage.User{
		ID:           auth.GenerateID(),
		Username:     req.Username,
		PasswordHash: hash,
		CreatedAt:    time.Now(),
	}

	if err := s.store.CreateUser(user); err != nil {
		errorResponse(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	// Return user without password hash
	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"id":        user.ID,
		"username":  user.Username,
		"createdAt": user.CreatedAt,
	})
}

// handleLogin authenticates a user and creates a session
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		errorResponse(w, http.StatusBadRequest, "Username and password are required")
		return
	}

	// Find user
	user, err := s.store.GetUserByUsername(req.Username)
	if err != nil {
		errorResponse(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	// Check password
	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		errorResponse(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	// Generate session token
	token, err := auth.GenerateToken()
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "Failed to generate session")
		return
	}

	// Parse session duration
	duration := 24 * time.Hour

	// Create session
	session := &storage.Session{
		ID:        auth.GenerateID(),
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: time.Now().Add(duration),
		CreatedAt: time.Now(),
	}

	if err := s.store.CreateSession(session); err != nil {
		errorResponse(w, http.StatusInternalServerError, "Failed to create session")
		return
	}

	// Set cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(duration.Seconds()),
	})

	// Return user info
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"id":        user.ID,
		"username":  user.Username,
		"createdAt": user.CreatedAt,
		"token":     token, // Also return token for clients that prefer header auth
	})
}

// handleLogout invalidates the current session
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	// Try to get token from Authorization header or cookie
	token := ""
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token = strings.TrimPrefix(authHeader, "Bearer ")
	}
	if token == "" {
		cookie, err := r.Cookie("session")
		if err == nil {
			token = cookie.Value
		}
	}

	// Delete session if found
	if token != "" {
		session, err := s.store.GetSessionByToken(token)
		if err == nil {
			s.store.DeleteSession(session.ID)
		}
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	w.WriteHeader(http.StatusNoContent)
}

// handleGetCurrentUser returns the currently authenticated user
func (s *Server) handleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	// Try to get token from Authorization header or cookie
	token := ""
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token = strings.TrimPrefix(authHeader, "Bearer ")
	}
	if token == "" {
		cookie, err := r.Cookie("session")
		if err == nil {
			token = cookie.Value
		}
	}

	if token == "" {
		errorResponse(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	// Validate session
	session, err := s.store.GetSessionByToken(token)
	if err != nil {
		errorResponse(w, http.StatusUnauthorized, "Invalid session")
		return
	}

	// Check if session expired
	if time.Now().After(session.ExpiresAt) {
		s.store.DeleteSession(session.ID)
		errorResponse(w, http.StatusUnauthorized, "Session expired")
		return
	}

	// Get user
	user, err := s.store.GetUser(session.UserID)
	if err != nil {
		errorResponse(w, http.StatusUnauthorized, "User not found")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"id":        user.ID,
		"username":  user.Username,
		"createdAt": user.CreatedAt,
	})
}

// handleUpdateBackupSettings updates backup settings for a database
func (s *Server) handleUpdateBackupSettings(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		errorResponse(w, http.StatusBadRequest, "Database ID is required")
		return
	}

	var req struct {
		BackupEnabled        bool   `json:"backupEnabled"`
		BackupSchedule       string `json:"backupSchedule"`
		BackupRetentionCount int    `json:"backupRetentionCount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	db, err := s.store.GetDatabase(id)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "Database not found")
		return
	}

	db.BackupEnabled = req.BackupEnabled
	db.BackupSchedule = req.BackupSchedule
	db.BackupRetentionCount = req.BackupRetentionCount

	if err := s.store.UpdateDatabase(db); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, db)
}

// handleUpdateResources updates memory and CPU limits for a database (upscale/downscale)
func (s *Server) handleUpdateResources(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		errorResponse(w, http.StatusBadRequest, "Database ID is required")
		return
	}

	var req struct {
		MemoryLimit int64   `json:"memoryLimit"` // bytes
		CPULimit    float64 `json:"cpuLimit"`    // cores
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.MemoryLimit <= 0 && req.CPULimit <= 0 {
		errorResponse(w, http.StatusBadRequest, "At least one of memoryLimit or cpuLimit must be specified")
		return
	}

	db, err := s.db.UpdateResources(r.Context(), id, req.MemoryLimit, req.CPULimit)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, db)
}

// handleBulkStart starts multiple databases at once
func (s *Server) handleBulkStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.IDs) == 0 {
		errorResponse(w, http.StatusBadRequest, "No database IDs provided")
		return
	}

	var errors []string
	for _, id := range req.IDs {
		if err := s.db.Start(r.Context(), id); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", id, err))
		}
	}

	if len(errors) > 0 {
		jsonResponse(w, http.StatusPartialContent, map[string]interface{}{
			"message": "Some databases failed to start",
			"errors":  errors,
		})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"message": "All databases started"})
}

// handleBulkStop stops multiple databases at once
func (s *Server) handleBulkStop(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.IDs) == 0 {
		errorResponse(w, http.StatusBadRequest, "No database IDs provided")
		return
	}

	var errors []string
	for _, id := range req.IDs {
		if err := s.db.Stop(r.Context(), id); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", id, err))
		}
	}

	if len(errors) > 0 {
		jsonResponse(w, http.StatusPartialContent, map[string]interface{}{
			"message": "Some databases failed to stop",
			"errors":  errors,
		})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"message": "All databases stopped"})
}

// handleBulkDelete deletes multiple databases at once
func (s *Server) handleBulkDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.IDs) == 0 {
		errorResponse(w, http.StatusBadRequest, "No database IDs provided")
		return
	}

	var errors []string
	for _, id := range req.IDs {
		if err := s.db.Delete(r.Context(), id); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", id, err))
		}
	}

	if len(errors) > 0 {
		jsonResponse(w, http.StatusPartialContent, map[string]interface{}{
			"message": "Some databases failed to delete",
			"errors":  errors,
		})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"message": "All databases deleted"})
}

// handleDeleteBackup deletes a backup
func (s *Server) handleDeleteBackup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		errorResponse(w, http.StatusBadRequest, "Backup ID is required")
		return
	}

	if err := s.store.DeleteBackup(id); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleGetCredentials returns the database credentials including password
func (s *Server) handleGetCredentials(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		errorResponse(w, http.StatusBadRequest, "Database ID is required")
		return
	}

	db, err := s.store.GetDatabase(id)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "Database not found")
		return
	}

	// Return credentials (including password which is normally hidden)
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"username": db.Username,
		"password": db.Password,
		"database": db.Database,
		"host":     db.Host,
		"port":     db.Port,
		"engine":   db.Engine,
	})
}

// handleGetConnectionStrings returns connection strings for various languages/frameworks
func (s *Server) handleGetConnectionStrings(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		errorResponse(w, http.StatusBadRequest, "Database ID is required")
		return
	}

	db, err := s.store.GetDatabase(id)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "Database not found")
		return
	}

	strings := generateConnectionExamples(db)
	jsonResponse(w, http.StatusOK, strings)
}

// handleGetBackupInfo returns detailed information about a backup
func (s *Server) handleGetBackupInfo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		errorResponse(w, http.StatusBadRequest, "Backup ID is required")
		return
	}

	backup, err := s.store.GetBackup(id)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "Backup not found")
		return
	}

	// Get the source database info if it still exists
	var dbEngine, dbVersion string
	if db, err := s.store.GetDatabase(backup.DatabaseID); err == nil {
		dbEngine = db.Engine
		dbVersion = db.Version
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"id":           backup.ID,
		"databaseId":   backup.DatabaseID,
		"databaseName": backup.DatabaseName,
		"createdAt":    backup.CreatedAt,
		"size":         backup.Size,
		"status":       backup.Status,
		"engine":       dbEngine,
		"version":      dbVersion,
	})
}

// handleGetMetricsHistory returns historical metrics for a database
func (s *Server) handleGetMetricsHistory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		errorResponse(w, http.StatusBadRequest, "Database ID is required")
		return
	}

	// Get metrics history from manager
	history := s.db.GetMetricsHistory(id)
	jsonResponse(w, http.StatusOK, history)
}

// ConnectionExample represents a code example for connecting to a database
type ConnectionExample struct {
	Title       string `json:"title"`
	Language    string `json:"language"` // for syntax highlighting: bash, python, javascript, java, go
	Code        string `json:"code"`
	Description string `json:"description"`
}

// generateConnectionExamples creates full code examples for different languages/tools
func generateConnectionExamples(db *storage.DatabaseInstance) []ConnectionExample {
	var examples []ConnectionExample

	// Return empty if database is still being created
	if db.ContainerID == "" {
		return examples
	}

	host := db.Host
	port := db.Port
	user := db.Username
	pass := db.Password
	dbName := db.Database

	// Helper to safely truncate container ID
	containerID := db.ContainerID
	if len(containerID) > 12 {
		containerID = containerID[:12]
	}

	switch db.Engine {
	case "postgresql":
		examples = append(examples, ConnectionExample{
			Title:       "Docker",
			Language:    "bash",
			Description: "Connect using the container's psql client",
			Code:        fmt.Sprintf("docker exec -it %s psql -U %s -d %s", containerID, user, dbName),
		})
		examples = append(examples, ConnectionExample{
			Title:       "CLI",
			Language:    "bash",
			Description: "Connect using local psql client",
			Code:        fmt.Sprintf("psql -h %s -p %d -U %s -d %s\n# Password: %s", host, port, user, dbName, pass),
		})
		examples = append(examples, ConnectionExample{
			Title:       "Python",
			Language:    "python",
			Description: "Connect using psycopg2",
			Code: fmt.Sprintf(`import psycopg2

conn = psycopg2.connect(
    host="%s",
    port=%d,
    user="%s",
    password="%s",
    database="%s"
)

cursor = conn.cursor()
cursor.execute("SELECT version();")
print(cursor.fetchone())
conn.close()`, host, port, user, pass, dbName),
		})
		examples = append(examples, ConnectionExample{
			Title:       "Node.js",
			Language:    "javascript",
			Description: "Connect using pg (node-postgres)",
			Code: fmt.Sprintf(`const { Pool } = require('pg');

const pool = new Pool({
  host: '%s',
  port: %d,
  user: '%s',
  password: '%s',
  database: '%s'
});

pool.query('SELECT NOW()', (err, res) => {
  console.log(res.rows[0]);
  pool.end();
});`, host, port, user, pass, dbName),
		})
		examples = append(examples, ConnectionExample{
			Title:       "Java",
			Language:    "java",
			Description: "Connect using JDBC",
			Code: fmt.Sprintf(`import java.sql.*;

public class PostgresExample {
    public static void main(String[] args) throws SQLException {
        String url = "jdbc:postgresql://%s:%d/%s";
        String user = "%s";
        String password = "%s";
        
        try (Connection conn = DriverManager.getConnection(url, user, password)) {
            Statement stmt = conn.createStatement();
            ResultSet rs = stmt.executeQuery("SELECT version()");
            while (rs.next()) {
                System.out.println(rs.getString(1));
            }
        }
    }
}`, host, port, dbName, user, pass),
		})
		examples = append(examples, ConnectionExample{
			Title:       "Go",
			Language:    "go",
			Description: "Connect using lib/pq",
			Code: fmt.Sprintf(`package main

import (
    "database/sql"
    "fmt"
    _ "github.com/lib/pq"
)

func main() {
    connStr := "host=%s port=%d user=%s password=%s dbname=%s sslmode=disable"
    db, err := sql.Open("postgres", connStr)
    if err != nil {
        panic(err)
    }
    defer db.Close()
    
    var version string
    db.QueryRow("SELECT version()").Scan(&version)
    fmt.Println(version)
}`, host, port, user, pass, dbName),
		})

	case "mysql", "mariadb":
		cliTool := "mysql"
		examples = append(examples, ConnectionExample{
			Title:       "Docker",
			Language:    "bash",
			Description: "Connect using the container's mysql client",
			Code:        fmt.Sprintf("docker exec -it %s mysql -u %s -p%s %s", containerID, user, pass, dbName),
		})
		examples = append(examples, ConnectionExample{
			Title:       "CLI",
			Language:    "bash",
			Description: "Connect using local mysql client",
			Code:        fmt.Sprintf("%s -h %s -P %d -u %s -p%s %s", cliTool, host, port, user, pass, dbName),
		})
		examples = append(examples, ConnectionExample{
			Title:       "Python",
			Language:    "python",
			Description: "Connect using PyMySQL",
			Code: fmt.Sprintf(`import pymysql

conn = pymysql.connect(
    host="%s",
    port=%d,
    user="%s",
    password="%s",
    database="%s"
)

cursor = conn.cursor()
cursor.execute("SELECT VERSION()")
print(cursor.fetchone())
conn.close()`, host, port, user, pass, dbName),
		})
		examples = append(examples, ConnectionExample{
			Title:       "Node.js",
			Language:    "javascript",
			Description: "Connect using mysql2",
			Code: fmt.Sprintf(`const mysql = require('mysql2');

const connection = mysql.createConnection({
  host: '%s',
  port: %d,
  user: '%s',
  password: '%s',
  database: '%s'
});

connection.query('SELECT VERSION()', (err, results) => {
  console.log(results);
  connection.end();
});`, host, port, user, pass, dbName),
		})
		examples = append(examples, ConnectionExample{
			Title:       "Java",
			Language:    "java",
			Description: "Connect using JDBC",
			Code: fmt.Sprintf(`import java.sql.*;

public class MySQLExample {
    public static void main(String[] args) throws SQLException {
        String url = "jdbc:mysql://%s:%d/%s";
        String user = "%s";
        String password = "%s";
        
        try (Connection conn = DriverManager.getConnection(url, user, password)) {
            Statement stmt = conn.createStatement();
            ResultSet rs = stmt.executeQuery("SELECT VERSION()");
            while (rs.next()) {
                System.out.println(rs.getString(1));
            }
        }
    }
}`, host, port, dbName, user, pass),
		})
		examples = append(examples, ConnectionExample{
			Title:       "Go",
			Language:    "go",
			Description: "Connect using go-sql-driver/mysql",
			Code: fmt.Sprintf(`package main

import (
    "database/sql"
    "fmt"
    _ "github.com/go-sql-driver/mysql"
)

func main() {
    dsn := "%s:%s@tcp(%s:%d)/%s"
    db, err := sql.Open("mysql", dsn)
    if err != nil {
        panic(err)
    }
    defer db.Close()
    
    var version string
    db.QueryRow("SELECT VERSION()").Scan(&version)
    fmt.Println(version)
}`, user, pass, host, port, dbName),
		})

	case "redis":
		if pass != "" {
			examples = append(examples, ConnectionExample{
				Title:       "Docker",
				Language:    "bash",
				Description: "Connect using the container's redis-cli",
				Code:        fmt.Sprintf("docker exec -it %s redis-cli -a %s", containerID, pass),
			})
			examples = append(examples, ConnectionExample{
				Title:       "CLI",
				Language:    "bash",
				Description: "Connect using local redis-cli",
				Code:        fmt.Sprintf("redis-cli -h %s -p %d -a %s", host, port, pass),
			})
			examples = append(examples, ConnectionExample{
				Title:       "Python",
				Language:    "python",
				Description: "Connect using redis-py",
				Code: fmt.Sprintf(`import redis

r = redis.Redis(
    host="%s",
    port=%d,
    password="%s",
    decode_responses=True
)

r.set("test_key", "Hello, Redis!")
print(r.get("test_key"))`, host, port, pass),
			})
			examples = append(examples, ConnectionExample{
				Title:       "Node.js",
				Language:    "javascript",
				Description: "Connect using ioredis",
				Code: fmt.Sprintf(`const Redis = require('ioredis');

const redis = new Redis({
  host: '%s',
  port: %d,
  password: '%s'
});

redis.set('test_key', 'Hello, Redis!');
redis.get('test_key').then(console.log);`, host, port, pass),
			})
		} else {
			examples = append(examples, ConnectionExample{
				Title:       "Docker",
				Language:    "bash",
				Description: "Connect using the container's redis-cli",
				Code:        fmt.Sprintf("docker exec -it %s redis-cli", containerID),
			})
			examples = append(examples, ConnectionExample{
				Title:       "CLI",
				Language:    "bash",
				Description: "Connect using local redis-cli",
				Code:        fmt.Sprintf("redis-cli -h %s -p %d", host, port),
			})
			examples = append(examples, ConnectionExample{
				Title:       "Python",
				Language:    "python",
				Description: "Connect using redis-py",
				Code: fmt.Sprintf(`import redis

r = redis.Redis(
    host="%s",
    port=%d,
    decode_responses=True
)

r.set("test_key", "Hello, Redis!")
print(r.get("test_key"))`, host, port),
			})
			examples = append(examples, ConnectionExample{
				Title:       "Node.js",
				Language:    "javascript",
				Description: "Connect using ioredis",
				Code: fmt.Sprintf(`const Redis = require('ioredis');

const redis = new Redis({
  host: '%s',
  port: %d
});

redis.set('test_key', 'Hello, Redis!');
redis.get('test_key').then(console.log);`, host, port),
			})
		}
	}

	return examples
}
