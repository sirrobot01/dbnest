package database

import (
	"context"

	"github.com/sirrobot01/dbnest/pkg/runtime"
	"github.com/sirrobot01/dbnest/pkg/storage"
)

// QueryResult represents the result of a database query
type QueryResult struct {
	Columns  []string        `json:"columns,omitempty"`
	Rows     [][]interface{} `json:"rows,omitempty"`
	Message  string          `json:"message,omitempty"`
	Error    string          `json:"error,omitempty"`
	RowCount int             `json:"rowCount"`
}

// ConnectionStrings holds connection strings for various languages
type ConnectionStrings struct {
	URI    string `json:"uri"`
	Python string `json:"python"`
	Node   string `json:"node"`
	Go     string `json:"go"`
	Java   string `json:"java"`
	Ruby   string `json:"ruby"`
	PHP    string `json:"php"`
}

// Engine defines the interface for database engine implementations
// Each database type (PostgreSQL, MySQL, etc) implements this interface
type Engine interface {
	Name() string
	Type() string // e.g., "postgresql", "mysql", "redis"
	Image() string
	DefaultPort() int
	DataPath() string
	Versions() []string

	EnvVars(username, password, database string) []string
	// ContainerCmd returns custom command/args to run the container (optional, nil = use image default)
	ContainerCmd(password string) []string

	// Backup and restore
	Backup(ctx context.Context, client runtime.Client, db *storage.DatabaseInstance, backupPath string) error
	Restore(ctx context.Context, client runtime.Client, db *storage.DatabaseInstance, backupPath string) error

	ExecuteQuery(ctx context.Context, docker runtime.Client, db *storage.DatabaseInstance, query string) (*QueryResult, error)

	ConnectionStrings(db *storage.DatabaseInstance) *ConnectionStrings

	// CLICommand returns the command to execute a script via stdin
	CLICommand(username, password, database string) []string
}
