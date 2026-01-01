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
	RegisterEngine(&MariaDBEngine{})
}

// MariaDBEngine implements the Engine interface for MariaDB
// MariaDB is MySQL-compatible, so it uses similar tools
type MariaDBEngine struct{}

func (e *MariaDBEngine) Name() string {
	return "MariaDB"
}

func (e *MariaDBEngine) Type() string {
	return "mariadb"
}

func (e *MariaDBEngine) Image() string {
	return "mariadb"
}

func (e *MariaDBEngine) DefaultPort() int {
	return 3306
}

func (e *MariaDBEngine) DataPath() string {
	return "/var/lib/mysql"
}

func (e *MariaDBEngine) Versions() []string {
	return []string{"11", "10.11", "10.6", "10.5"}
}

func (e *MariaDBEngine) EnvVars(username, password, database string) []string {
	return []string{
		"MARIADB_ROOT_PASSWORD=" + password,
		"MARIADB_DATABASE=" + database,
		"MARIADB_USER=" + username,
		"MARIADB_PASSWORD=" + password,
	}
}

func (e *MariaDBEngine) ContainerCmd(password string) []string {
	return nil // use image default
}

func (e *MariaDBEngine) Backup(ctx context.Context, dockerClient runtime.Client, db *storage.DatabaseInstance, backupPath string) error {
	cmd := []string{
		"mariadb-dump",
		"-u", db.Username,
		"-p" + db.Password,
		db.Database,
	}

	if err := os.MkdirAll(filepath.Dir(backupPath), 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	output, err := dockerClient.Exec(ctx, db.ContainerID, cmd, nil)
	if err != nil {
		return fmt.Errorf("mariadb-dump failed: %w", err)
	}

	if err := os.WriteFile(backupPath, []byte(output), 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	return nil
}

func (e *MariaDBEngine) Restore(ctx context.Context, dockerClient runtime.Client, db *storage.DatabaseInstance, backupPath string) error {
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	cmd := []string{
		"mariadb",
		"-u", db.Username,
		"-p" + db.Password,
		db.Database,
	}

	output, err := dockerClient.ExecWithStdin(ctx, db.ContainerID, cmd, data, nil)
	if err != nil {
		return fmt.Errorf("mariadb restore failed: %w, output: %s", err, output)
	}

	return nil
}

func (e *MariaDBEngine) ExecuteQuery(ctx context.Context, dockerClient runtime.Client, db *storage.DatabaseInstance, query string) (*QueryResult, error) {
	cmd := []string{
		"mariadb",
		"-u", db.Username,
		"-p" + db.Password,
		"-B", // Batch mode (tab-separated, includes headers)
		db.Database,
		"-e", query,
	}

	output, err := dockerClient.Exec(ctx, db.ContainerID, cmd, nil)
	if err != nil {
		return &QueryResult{Error: fmt.Sprintf("Query failed: %v", err)}, nil
	}

	result := &QueryResult{
		Columns: []string{},
		Rows:    [][]interface{}{},
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		result.Message = "Query executed successfully (no output)"
		return result, nil
	}

	for i, line := range lines {
		if line == "" {
			continue
		}
		cols := strings.Split(line, "\t")

		if i == 0 {
			// First line is headers
			for _, col := range cols {
				result.Columns = append(result.Columns, col)
			}
		} else {
			// Data rows
			row := make([]interface{}, len(cols))
			for j, col := range cols {
				if col == "NULL" {
					row[j] = nil
				} else {
					row[j] = col
				}
			}
			result.Rows = append(result.Rows, row)
		}
	}
	result.RowCount = len(result.Rows)

	return result, nil
}

func (e *MariaDBEngine) ConnectionStrings(db *storage.DatabaseInstance) *ConnectionStrings {
	uri := fmt.Sprintf("mysql://%s:<password>@%s:%d/%s", db.Username, db.Host, db.Port, db.Database)

	return &ConnectionStrings{
		URI: uri,
		Python: fmt.Sprintf(`import mariadb
conn = mariadb.connect(
    host="%s",
    port=%d,
    user="%s",
    password="<password>",
    database="%s"
)`, db.Host, db.Port, db.Username, db.Database),
		Node: fmt.Sprintf(`const mariadb = require('mariadb');
const pool = mariadb.createPool({
    host: '%s',
    port: %d,
    user: '%s',
    password: '<password>',
    database: '%s'
});`, db.Host, db.Port, db.Username, db.Database),
		Go: fmt.Sprintf(`import (
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
)
db, err := sql.Open("mysql", "%s:<password>@tcp(%s:%d)/%s")`,
			db.Username, db.Host, db.Port, db.Database),
		Java: fmt.Sprintf(`String url = "jdbc:mariadb://%s:%d/%s";
Connection conn = DriverManager.getConnection(url, "%s", "<password>");`,
			db.Host, db.Port, db.Database, db.Username),
		Ruby: fmt.Sprintf(`require 'mysql2'
client = Mysql2::Client.new(
    host: '%s',
    port: %d,
    username: '%s',
    password: '<password>',
    database: '%s'
)`, db.Host, db.Port, db.Username, db.Database),
		PHP: fmt.Sprintf(`$pdo = new PDO(
    'mysql:host=%s;port=%d;dbname=%s',
    '%s',
    '<password>'
);`, db.Host, db.Port, db.Database, db.Username),
	}
}

func (e *MariaDBEngine) CLICommand(username, password, database string) []string {
	return []string{
		"mariadb",
		"-u", username,
		"-p" + password,
		database,
	}
}
