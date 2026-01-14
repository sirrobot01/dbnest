package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
	LogLevelTrace LogLevel = "trace"
)

// Config holds all application configuration
type Config struct {
	LogLevel LogLevel
	Port     int
	DataDir  string
	Socket   string // Docker socket path (only used for docker runtime with SDK mode)
	Runtime  string // Container runtime: "docker", "podman", or "containerd"
}

// DockerNetwork returns the default Docker network name
func (c *Config) DockerNetwork() string {
	return "dbnest"
}

// StoragePath returns the path to the bbolt database file
func (c *Config) StoragePath() string {
	return filepath.Join(c.DataDir, "dbnest.db")
}

// Addr returns the HTTP server address
func (c *Config) Addr() string {
	if c.Port == 0 {
		return ":8080"
	}
	return fmt.Sprintf(":%d", c.Port)
}

// FromArgs creates a Config from CLI arguments
func FromArgs() *Config {
	port := flag.Int("port", 8080, "HTTP server port")
	dataDir := flag.String("data", "./data", "Data directory for storage")
	socket := flag.String("socket", "", "Docker socket path (only used for docker runtime with SDK mode)")
	runtime := flag.String("runtime", "docker", "Container runtime: docker, podman, or containerd")
	logLevel := flag.String("log-level", "info", "Logging level (info, debug, error, trace)")
	flag.Parse()

	if *dataDir == "" {
		*dataDir = "./data"
	}
	if *runtime == "" {
		*runtime = "docker"
	}
	if *logLevel == "" {
		*logLevel = "info"
	}

	return &Config{
		Port:     *port,
		DataDir:  *dataDir,
		Socket:   *socket,
		Runtime:  *runtime,
		LogLevel: LogLevel(*logLevel),
	}
}

// Validate validates the configuration and creates necessary directories
func (c *Config) Validate() error {
	// Ensure data directory exists
	if err := os.MkdirAll(c.DataDir, 0755); err != nil {
		return err
	}
	return nil
}
