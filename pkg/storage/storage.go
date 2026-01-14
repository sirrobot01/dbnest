package storage

import (
	"time"
)

// DatabaseInstance represents a database instance
type DatabaseInstance struct {
	ID             string    `json:"id" msgpack:"id"`
	Name           string    `json:"name" msgpack:"name"`
	Engine         string    `json:"engine" msgpack:"engine"`
	Version        string    `json:"version" msgpack:"version"`
	Status         string    `json:"status" msgpack:"status"`
	Host           string    `json:"host" msgpack:"host"`
	Port           int       `json:"port" msgpack:"port"`
	Username       string    `json:"username" msgpack:"username"`
	Password       string    `json:"-" msgpack:"password"` // Never sent to frontend
	Database       string    `json:"database" msgpack:"database"`
	ContainerID    string    `json:"containerId,omitempty" msgpack:"container_id"`
	CreatedAt      time.Time `json:"createdAt" msgpack:"created_at"`
	StorageUsed    int64     `json:"storageUsed" msgpack:"storage_used"`   // bytes
	StorageLimit   int64     `json:"storageLimit" msgpack:"storage_limit"` // bytes
	MemoryLimit    int64     `json:"memoryLimit" msgpack:"memory_limit"`   // bytes
	CPULimit       float64   `json:"cpuLimit" msgpack:"cpu_limit"`
	Connections    int       `json:"connections" msgpack:"connections"`
	MaxConnections int       `json:"maxConnections" msgpack:"max_connections"`
	ErrorMessage   string    `json:"errorMessage,omitempty" msgpack:"error_message"` // Error details if creation failed

	// Container networking options
	ExposePort bool   `json:"exposePort" msgpack:"expose_port"`    // Whether to expose port to host
	Network    string `json:"network,omitempty" msgpack:"network"` // Docker network name

	// Backup scheduling fields (per-database)
	BackupEnabled        bool       `json:"backupEnabled" msgpack:"backup_enabled"`
	BackupSchedule       string     `json:"backupSchedule,omitempty" msgpack:"backup_schedule"`    // cron expression e.g. "0 2 * * *"
	BackupRetentionCount int        `json:"backupRetentionCount" msgpack:"backup_retention_count"` // keep last N backups
	LastBackupAt         *time.Time `json:"lastBackupAt,omitempty" msgpack:"last_backup_at"`
}

// Backup represents a database backup
type Backup struct {
	ID           string    `json:"id" msgpack:"id"`
	DatabaseID   string    `json:"databaseId" msgpack:"database_id"`
	DatabaseName string    `json:"databaseName" msgpack:"database_name"`
	CreatedAt    time.Time `json:"createdAt" msgpack:"created_at"`
	Size         int64     `json:"size" msgpack:"size"` // bytes
	Status       string    `json:"status" msgpack:"status"`
	FilePath     string    `json:"-" msgpack:"file_path"`
}

// User represents an authenticated user
type User struct {
	ID           string    `json:"id" msgpack:"id"`
	Username     string    `json:"username" msgpack:"username"`
	PasswordHash string    `json:"-" msgpack:"password_hash"` // Never sent to frontend
	CreatedAt    time.Time `json:"createdAt" msgpack:"created_at"`
}

// Session represents an authenticated user session
type Session struct {
	ID        string    `json:"id" msgpack:"id"`
	UserID    string    `json:"userId" msgpack:"user_id"`
	Token     string    `json:"-" msgpack:"token"` // Never sent to frontend
	ExpiresAt time.Time `json:"expiresAt" msgpack:"expires_at"`
	CreatedAt time.Time `json:"createdAt" msgpack:"created_at"`
}

// Storage defines the interface for data persistence
type Storage interface {
	Close() error
	DataDir() string

	// Database operations
	CreateDatabase(db *DatabaseInstance) error
	GetDatabase(id string) (*DatabaseInstance, error)
	ListDatabases() []*DatabaseInstance
	UpdateDatabase(db *DatabaseInstance) error
	DeleteDatabase(id string) error

	// Backup operations
	CreateBackup(backup *Backup) error
	GetBackup(id string) (*Backup, error)
	GetBackupPath(id string) string
	ListBackups(databaseID string) []*Backup
	UpdateBackup(backup *Backup) error
	DeleteBackup(id string) error

	// User operations
	CreateUser(user *User) error
	GetUser(id string) (*User, error)
	GetUserByUsername(username string) (*User, error)
	ListUsers() []*User
	UpdateUser(user *User) error
	DeleteUser(id string) error
	UserCount() int

	// Session operations
	CreateSession(session *Session) error
	GetSession(id string) (*Session, error)
	GetSessionByToken(token string) (*Session, error)
	DeleteSession(id string) error
	DeleteExpiredSessions() error

	// Settings operations
	GetSetting(key string) (string, error)
	SetSetting(key, value string) error
}

// New creates a new storage instance based on type
func New(path, dataDir string) (Storage, error) {
	return NewBoltStorage(path, dataDir)
}
