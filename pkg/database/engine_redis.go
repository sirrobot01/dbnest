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
	RegisterEngine(&RedisEngine{})
}

// RedisEngine implements the Engine interface for Redis
type RedisEngine struct{}

func (e *RedisEngine) Name() string {
	return "Redis"
}

func (e *RedisEngine) Type() string {
	return "redis"
}

func (e *RedisEngine) Image() string {
	return "redis"
}

func (e *RedisEngine) DefaultPort() int {
	return 6379
}

func (e *RedisEngine) DataPath() string {
	return "/data"
}

func (e *RedisEngine) Versions() []string {
	return []string{"7", "7.2", "6", "6.2"}
}

func (e *RedisEngine) EnvVars(username, password, database string) []string {
	// Redis doesn't use environment variables for auth
	// Password is set via container command args
	return nil
}

func (e *RedisEngine) ContainerCmd(password string) []string {
	if password != "" {
		return []string{"redis-server", "--requirepass", password}
	}
	return nil
}

func (e *RedisEngine) Backup(ctx context.Context, dockerClient runtime.Client, db *storage.DatabaseInstance, backupPath string) error {
	// Trigger a background save
	var authArgs []string
	if db.Password != "" {
		authArgs = []string{"-a", db.Password}
	}

	cmd := append([]string{"redis-cli"}, authArgs...)
	cmd = append(cmd, "BGSAVE")

	_, err := dockerClient.Exec(ctx, db.ContainerID, cmd, nil)
	if err != nil {
		return fmt.Errorf("BGSAVE failed: %w", err)
	}

	// Wait for save to complete
	waitCmd := append([]string{"redis-cli"}, authArgs...)
	waitCmd = append(waitCmd, "LASTSAVE")

	// Copy the dump.rdb file
	if err := os.MkdirAll(filepath.Dir(backupPath), 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	copyCmd := []string{"cat", "/data/dump.rdb"}
	data, err := dockerClient.Exec(ctx, db.ContainerID, copyCmd, nil)
	if err != nil {
		return fmt.Errorf("failed to read dump.rdb: %w", err)
	}

	if err := os.WriteFile(backupPath, []byte(data), 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	return nil
}

func (e *RedisEngine) Restore(ctx context.Context, dockerClient runtime.Client, db *storage.DatabaseInstance, backupPath string) error {
	// For Redis, restoring requires stopping the server, replacing dump.rdb, and restarting
	// This is complex in a container environment, so we provide a simple implementation
	return fmt.Errorf("redis restore requires container restart - use Docker volume restore instead")
}

func (e *RedisEngine) ExecuteQuery(ctx context.Context, dockerClient runtime.Client, db *storage.DatabaseInstance, query string) (*QueryResult, error) {
	// Redis uses commands, not SQL queries
	// Parse command respecting quoted strings
	args := parseRedisCommand(query)
	if len(args) == 0 {
		return &QueryResult{Error: "Empty command"}, nil
	}

	cmd := []string{"redis-cli"}
	if db.Password != "" {
		cmd = append(cmd, "-a", db.Password)
	}
	cmd = append(cmd, args...)

	output, err := dockerClient.Exec(ctx, db.ContainerID, cmd, nil)
	if err != nil {
		return &QueryResult{Error: fmt.Sprintf("Command failed: %v", err)}, nil
	}

	// Redis returns plain text
	trimmedOutput := strings.TrimSpace(output)

	// Check for error responses
	if strings.HasPrefix(trimmedOutput, "ERR ") || strings.HasPrefix(trimmedOutput, "(error)") {
		return &QueryResult{Error: trimmedOutput, RowCount: 0}, nil
	}

	result := &QueryResult{
		Message:  trimmedOutput,
		Rows:     [][]interface{}{},
		RowCount: 0,
	}

	// Try to parse multi-line output as rows
	lines := strings.Split(trimmedOutput, "\n")
	if len(lines) > 1 {
		for _, line := range lines {
			result.Rows = append(result.Rows, []interface{}{line})
		}
		result.RowCount = len(lines)
		result.Columns = []string{"value"}
		result.Message = "" // Clear message when showing as table
	}

	return result, nil
}

// parseRedisCommand splits a Redis command respecting quoted strings
func parseRedisCommand(input string) []string {
	var args []string
	var current strings.Builder
	inQuotes := false
	quoteChar := rune(0)

	for _, r := range input {
		switch {
		case (r == '"' || r == '\'') && !inQuotes:
			inQuotes = true
			quoteChar = r
		case r == quoteChar && inQuotes:
			inQuotes = false
			quoteChar = 0
		case r == ' ' && !inQuotes:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

func (e *RedisEngine) ConnectionStrings(db *storage.DatabaseInstance) *ConnectionStrings {
	var uri string
	if db.Password != "" {
		uri = fmt.Sprintf("redis://:%s@%s:%d", "<password>", db.Host, db.Port)
	} else {
		uri = fmt.Sprintf("redis://%s:%d", db.Host, db.Port)
	}

	return &ConnectionStrings{
		URI: uri,
		Python: fmt.Sprintf(`import redis
r = redis.Redis(
    host='%s',
    port=%d,
    password='<password>',
    decode_responses=True
)`, db.Host, db.Port),
		Node: fmt.Sprintf(`const Redis = require('ioredis');
const redis = new Redis({
    host: '%s',
    port: %d,
    password: '<password>'
});`, db.Host, db.Port),
		Go: fmt.Sprintf(`import "github.com/redis/go-redis/v9"
rdb := redis.NewClient(&redis.Options{
    Addr:     "%s:%d",
    Password: "<password>",
    DB:       0,
})`, db.Host, db.Port),
		Java: fmt.Sprintf(`import redis.clients.jedis.Jedis;
Jedis jedis = new Jedis("%s", %d);
jedis.auth("<password>");`, db.Host, db.Port),
		Ruby: fmt.Sprintf(`require 'redis'
redis = Redis.new(
    host: '%s',
    port: %d,
    password: '<password>'
)`, db.Host, db.Port),
		PHP: fmt.Sprintf(`$redis = new Redis();
$redis->connect('%s', %d);
$redis->auth('<password>');`, db.Host, db.Port),
	}
}

func (e *RedisEngine) CLICommand(username, password, database string) []string {
	cmd := []string{"redis-cli"}
	if password != "" {
		cmd = append(cmd, "-a", password)
	}
	cmd = append(cmd, "--pipe")
	return cmd
}
