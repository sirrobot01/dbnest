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
	RegisterEngine(&MySQLEngine{})
}

// MySQLEngine implements the Engine interface for MySQL
type MySQLEngine struct{}

func (e *MySQLEngine) Name() string {
	return "MySQL"
}

func (e *MySQLEngine) Type() string {
	return "mysql"
}

func (e *MySQLEngine) Image() string {
	return "mysql"
}

func (e *MySQLEngine) DefaultPort() int {
	return 3306
}

func (e *MySQLEngine) DataPath() string {
	return "/var/lib/mysql"
}

func (e *MySQLEngine) Versions() []string {
	return []string{"8.0", "8.4", "5.7"}
}

func (e *MySQLEngine) EnvVars(username, password, database string) []string {
	return []string{
		"MYSQL_ROOT_PASSWORD=" + password,
		"MYSQL_DATABASE=" + database,
		"MYSQL_USER=" + username,
		"MYSQL_PASSWORD=" + password,
	}
}

func (e *MySQLEngine) ContainerCmd(password string) []string {
	return nil // use image default
}

func (e *MySQLEngine) Backup(ctx context.Context, dockerClient runtime.Client, db *storage.DatabaseInstance, backupPath string) error {
	cmd := []string{
		"mysqldump",
		"-u", db.Username,
		"-p" + db.Password,
		db.Database,
	}

	if err := os.MkdirAll(filepath.Dir(backupPath), 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	output, err := dockerClient.Exec(ctx, db.ContainerID, cmd, nil)
	if err != nil {
		return fmt.Errorf("mysqldump failed: %w", err)
	}

	if err := os.WriteFile(backupPath, []byte(output), 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	return nil
}

func (e *MySQLEngine) Restore(ctx context.Context, dockerClient runtime.Client, db *storage.DatabaseInstance, backupPath string) error {
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	cmd := []string{
		"mysql",
		"-u", db.Username,
		"-p" + db.Password,
		db.Database,
	}

	output, err := dockerClient.ExecWithStdin(ctx, db.ContainerID, cmd, data, nil)
	if err != nil {
		return fmt.Errorf("mysql restore failed: %w, output: %s", err, output)
	}

	return nil
}

func (e *MySQLEngine) ExecuteQuery(ctx context.Context, client runtime.Client, db *storage.DatabaseInstance, query string) (*QueryResult, error) {
	cmd := []string{
		"mysql",
		"-u", db.Username,
		"-p" + db.Password,
		"-B", // Batch mode (tab-separated, includes headers)
		db.Database,
		"-e", query,
	}

	output, err := client.Exec(ctx, db.ContainerID, cmd, nil)
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

func (e *MySQLEngine) ConnectionStrings(db *storage.DatabaseInstance) *ConnectionStrings {
	uri := fmt.Sprintf("mysql://%s:<password>@%s:%d/%s", db.Username, db.Host, db.Port, db.Database)

	return &ConnectionStrings{
		URI: uri,
		Python: fmt.Sprintf(`import mysql.connector
conn = mysql.connector.connect(
    host="%s",
    port=%d,
    user="%s",
    password="<password>",
    database="%s"
)`, db.Host, db.Port, db.Username, db.Database),
		Node: fmt.Sprintf(`const mysql = require('mysql2');
const conn = mysql.createConnection({
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
		Java: fmt.Sprintf(`String url = "jdbc:mysql://%s:%d/%s";
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

func (e *MySQLEngine) CLICommand(username, password, database string) []string {
	return []string{
		"mysql",
		"-u", username,
		"-p" + password,
		database,
	}
}
