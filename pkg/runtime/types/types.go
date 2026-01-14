// Package types defines shared types for the runtime package hierarchy.
// This package exists to avoid import cycles between runtime and its sub-packages.
package types

import "context"

// Client defines the container runtime operations interface.
// Implementations: docker.Client, containerd.Client, cli.Client
type Client interface {
	// Lifecycle
	Close() error
	Ping(ctx context.Context) error

	// Image operations
	PullImage(ctx context.Context, imageName string) error

	// Container operations
	CreateContainer(ctx context.Context, cfg *ContainerConfig) (string, error)
	StartContainer(ctx context.Context, containerID string) error
	StopContainer(ctx context.Context, containerID string) error
	RemoveContainer(ctx context.Context, containerID string, force bool) error

	// Container inspection
	GetContainerStatus(ctx context.Context, containerID string) (string, error)
	GetContainerStats(ctx context.Context, containerID string) (*ContainerStats, error)
	GetContainerLogs(ctx context.Context, containerID string, tail int) (string, error)
	ListContainers(ctx context.Context) ([]string, error)

	// Network operations
	ListNetworks(ctx context.Context) ([]NetworkInfo, error)
	CreateNetwork(ctx context.Context, name string) (*NetworkInfo, error)
	DeleteNetwork(ctx context.Context, networkID string) error

	// Container interaction
	ExecInContainer(ctx context.Context, containerID string, cmd []string) (string, error)
	Exec(ctx context.Context, containerID string, cmd []string, env []string) (string, error)
	ExecWithStdin(ctx context.Context, containerID string, cmd []string, stdin []byte, env []string) (string, error)

	// Resource management
	UpdateContainerResources(ctx context.Context, containerID string, memoryLimit int64, cpuLimit float64) error

	// Volume management
	DeleteVolume(ctx context.Context, name string) error
}

// NetworkInfo holds information about a container network
type NetworkInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Driver string `json:"driver"`
}

// ContainerConfig holds configuration for creating a container
type ContainerConfig struct {
	Name         string
	Image        string
	Cmd          []string          // command/args to run (optional, overrides image default)
	Env          []string
	PortBindings map[string]string // containerPort/proto -> hostPort
	Volumes      map[string]string // hostPath -> containerPath
	MemoryLimit  int64             // bytes
	CPULimit     float64           // cores
	Labels       map[string]string
	Network      string // network name (optional)
	ExposePort   bool   // whether to bind port to host
}

// ContainerStats holds container resource statistics
type ContainerStats struct {
	CPUPercent    float64
	MemoryUsage   int64
	MemoryLimit   int64
	MemoryPercent float64
	NetworkRx     int64
	NetworkTx     int64
}
