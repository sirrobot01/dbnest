package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirrobot01/dbnest/pkg/runtime"
	"github.com/sirrobot01/dbnest/pkg/storage"
)

func init() {
	RegisterEngine(&PostgreSQLEngine{})
}

// PostgreSQLEngine implements the Engine interface for PostgreSQL
type PostgreSQLEngine struct{}

func (e *PostgreSQLEngine) Name() string {
	return "PostgreSQL"
}

func (e *PostgreSQLEngine) Type() string {
	return "postgresql"
}

func (e *PostgreSQLEngine) Image() string {
	return "postgres"
}

func (e *PostgreSQLEngine) DefaultPort() int {
	return 5432
}

func (e *PostgreSQLEngine) DataPath() string {
	return "/var/lib/postgresql/data"
}

func (e *PostgreSQLEngine) Versions() []string {
	return []string{"16", "15", "14", "13", "12"}
}

func (e *PostgreSQLEngine) EnvVars(username, password, database string) []string {
	return []string{
		"POSTGRES_USER=" + username,
		"POSTGRES_PASSWORD=" + password,
		"POSTGRES_DB=" + database,
	}
}

func (e *PostgreSQLEngine) ContainerCmd(password string) []string {
	return nil // use image default
}

func (e *PostgreSQLEngine) Backup(ctx context.Context, dockerClient runtime.Client, db *storage.DatabaseInstance, backupPath string) error {
	// Use pg_dump to create a backup
	cmd := []string{
		"pg_dump",
		"-U", db.Username,
		"-d", db.Database,
		"-F", "c", // Custom format (compressed)
		"-f", "/backup/backup.dump",
	}

	// Create backup directory on host
	if err := os.MkdirAll(filepath.Dir(backupPath), 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	output, err := dockerClient.Exec(ctx, db.ContainerID, cmd, []string{"PGPASSWORD=" + db.Password})
	if err != nil {
		return fmt.Errorf("pg_dump failed: %w, output: %s", err, output)
	}

	// Copy backup file from container
	copyCmd := []string{"cat", "/backup/backup.dump"}
	data, err := dockerClient.Exec(ctx, db.ContainerID, copyCmd, nil)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	if err := os.WriteFile(backupPath, []byte(data), 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	return nil
}

func (e *PostgreSQLEngine) Restore(ctx context.Context, dockerClient runtime.Client, db *storage.DatabaseInstance, backupPath string) error {
	// Read backup file
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	// For simplicity, use psql with the backup
	// In production, you'd copy the file to container and use pg_restore
	cmd := []string{
		"pg_restore",
		"-U", db.Username,
		"-d", db.Database,
		"--clean",
		"--if-exists",
	}

	output, err := dockerClient.ExecWithStdin(ctx, db.ContainerID, cmd, data, []string{"PGPASSWORD=" + db.Password})
	if err != nil {
		return fmt.Errorf("pg_restore failed: %w, output: %s", err, output)
	}

	return nil
}

func (e *PostgreSQLEngine) ExecuteQuery(ctx context.Context, dockerClient runtime.Client, db *storage.DatabaseInstance, query string) (*QueryResult, error) {
	// Use psql to execute query - include headers for column names
	cmd := []string{
		"psql",
		"-U", db.Username,
		"-d", db.Database,
		"-A", // Unaligned output
		"-c", query,
	}

	output, err := dockerClient.Exec(ctx, db.ContainerID, cmd, []string{"PGPASSWORD=" + db.Password})
	if err != nil {
		return &QueryResult{Error: fmt.Sprintf("Query failed: %v", err)}, nil
	}

	// Parse output into rows - first line is headers
	result := &QueryResult{
		Columns: []string{},
		Rows:    [][]interface{}{},
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		result.Message = "Query executed successfully (no output)"
		return result, nil
	}

	// First line contains column headers (unless it's a row count like "(1 row)")
	for i, line := range lines {
		if line == "" {
			continue
		}
		// Skip row count footer like "(1 row)" or "(5 rows)"
		if strings.HasPrefix(line, "(") && strings.HasSuffix(line, ")") {
			continue
		}

		cols := strings.Split(line, "|")

		if i == 0 {
			// First line is headers
			for _, col := range cols {
				result.Columns = append(result.Columns, strings.TrimSpace(col))
			}
		} else {
			// Data rows
			row := make([]interface{}, len(cols))
			for j, col := range cols {
				trimmed := strings.TrimSpace(col)
				if trimmed == "" {
					row[j] = nil
				} else {
					row[j] = trimmed
				}
			}
			result.Rows = append(result.Rows, row)
		}
	}
	result.RowCount = len(result.Rows)

	return result, nil
}

func (e *PostgreSQLEngine) ConnectionStrings(db *storage.DatabaseInstance) *ConnectionStrings {
	uri := fmt.Sprintf("postgresql://%s:<password>@%s:%d/%s", db.Username, db.Host, db.Port, db.Database)

	return &ConnectionStrings{
		URI: uri,
		Python: fmt.Sprintf(`import psycopg2
conn = psycopg2.connect(
    host="%s",
    port=%d,
    user="%s",
    password="<password>",
    database="%s"
)`, db.Host, db.Port, db.Username, db.Database),
		Node: fmt.Sprintf(`const { Pool } = require('pg');
const pool = new Pool({
    host: '%s',
    port: %d,
    user: '%s',
    password: '<password>',
    database: '%s'
});`, db.Host, db.Port, db.Username, db.Database),
		Go: fmt.Sprintf(`import (
    "database/sql"
    _ "github.com/lib/pq"
)
db, err := sql.Open("postgres", "%s")`, uri),
		Java: fmt.Sprintf(`String url = "jdbc:postgresql://%s:%d/%s";
Connection conn = DriverManager.getConnection(url, "%s", "<password>");`,
			db.Host, db.Port, db.Database, db.Username),
		Ruby: fmt.Sprintf(`require 'pg'
conn = PG.connect(
    host: '%s',
    port: %d,
    user: '%s',
    password: '<password>',
    dbname: '%s'
)`, db.Host, db.Port, db.Username, db.Database),
		PHP: fmt.Sprintf(`$pdo = new PDO(
    'pgsql:host=%s;port=%d;dbname=%s',
    '%s',
    '<password>'
);`, db.Host, db.Port, db.Database, db.Username),
	}
}

// Helper to parse JSON output from psql
func (e *PostgreSQLEngine) CLICommand(username, password, database string) []string {
	return []string{
		"psql",
		"-U", username,
		"-d", database,
		"-f", "-", // Read from stdin
	}
}
