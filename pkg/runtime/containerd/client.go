package containerd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirrobot01/dbnest/pkg/runtime/types"
)

const (
	// Namespace is the containerd namespace for DBNest
	Namespace = "dbnest"
)

// Client wraps the containerd SDK client
type Client struct {
	cli     *containerd.Client
	network string
}

// Verify Client implements types.Client interface
var _ types.Client = (*Client)(nil)

// NewClient creates a new containerd SDK client
func NewClient(socketPath string, networkName string) (*Client, error) {
	cli, err := containerd.New(socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}

	c := &Client{
		cli:     cli,
		network: networkName,
	}

	return c, nil
}

// Close closes the containerd client
func (c *Client) Close() error {
	return c.cli.Close()
}

// ctx returns a context with the DBNest namespace
func (c *Client) ctx(parent context.Context) context.Context {
	return namespaces.WithNamespace(parent, Namespace)
}

// Ping checks if containerd is accessible
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.cli.Version(c.ctx(ctx))
	return err
}

// PullImage pulls a container image
func (c *Client) PullImage(ctx context.Context, imageName string) error {
	// Normalize image name for containerd
	// containerd requires fully qualified names like docker.io/library/postgres:16
	normalizedName := normalizeImageName(imageName)

	// Use native snapshotter which works better in Docker-in-Docker environments
	_, err := c.cli.Pull(c.ctx(ctx), normalizedName,
		containerd.WithPullUnpack,
		containerd.WithPullSnapshotter("native"),
	)
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", imageName, err)
	}
	return nil
}

// normalizeImageName converts Docker Hub short names to fully qualified references
func normalizeImageName(name string) string {
	// If already fully qualified, return as-is
	if strings.Contains(name, "/") && strings.Contains(strings.Split(name, "/")[0], ".") {
		return name
	}

	// Add docker.io prefix
	if !strings.Contains(name, "/") {
		// Official image like "postgres:16" -> "docker.io/library/postgres:16"
		return "docker.io/library/" + name
	}

	// User image like "user/repo:tag" -> "docker.io/user/repo:tag"
	return "docker.io/" + name
}

// CreateContainer creates a new container
func (c *Client) CreateContainer(ctx context.Context, cfg *types.ContainerConfig) (string, error) {
	ctx = c.ctx(ctx)

	// Get image (use normalized name)
	imageName := normalizeImageName(cfg.Image)
	image, err := c.cli.GetImage(ctx, imageName)
	if err != nil {
		return "", fmt.Errorf("image %s not found: %w", cfg.Image, err)
	}

	// Build OCI spec options
	specOpts := []oci.SpecOpts{
		oci.WithImageConfig(image),
		oci.WithEnv(cfg.Env),
	}

	// Add custom command if specified
	if len(cfg.Cmd) > 0 {
		specOpts = append(specOpts, oci.WithProcessArgs(cfg.Cmd...))
	}

	// Add mounts
	for hostPath, containerPath := range cfg.Volumes {
		source := hostPath
		
		// If source doesn't start with / or ., assume it's a named volume
		// Emulate named volumes for containerd by using a standard host directory
		if !strings.HasPrefix(source, "/") && !strings.HasPrefix(source, ".") {
			source = filepath.Join("/var/lib/dbnest/volumes", hostPath)
			// Ensure directory exists
			if err := os.MkdirAll(source, 0755); err != nil {
				return "", fmt.Errorf("failed to create volume directory %s: %w", source, err)
			}
		}

		specOpts = append(specOpts, oci.WithMounts([]specs.Mount{
			{
				Type:        "bind",
				Source:      source,
				Destination: containerPath,
				Options:     []string{"rbind", "rw"},
			},
		}))
	}

	// Add resource limits
	if cfg.MemoryLimit > 0 || cfg.CPULimit > 0 {
		specOpts = append(specOpts, func(_ context.Context, _ oci.Client, _ *containers.Container, s *oci.Spec) error {
			if s.Linux == nil {
				s.Linux = &specs.Linux{}
			}
			if s.Linux.Resources == nil {
				s.Linux.Resources = &specs.LinuxResources{}
			}
			if cfg.MemoryLimit > 0 {
				if s.Linux.Resources.Memory == nil {
					s.Linux.Resources.Memory = &specs.LinuxMemory{}
				}
				s.Linux.Resources.Memory.Limit = &cfg.MemoryLimit
			}
			if cfg.CPULimit > 0 {
				if s.Linux.Resources.CPU == nil {
					s.Linux.Resources.CPU = &specs.LinuxCPU{}
				}
				period := uint64(100000)
				quota := int64(cfg.CPULimit * float64(period))
				s.Linux.Resources.CPU.Period = &period
				s.Linux.Resources.CPU.Quota = &quota
			}
			return nil
		})
	}

	// Create container with native snapshotter (works in Docker-in-Docker)
	container, err := c.cli.NewContainer(
		ctx,
		cfg.Name,
		containerd.WithImage(image),
		containerd.WithSnapshotter("native"),
		containerd.WithNewSnapshot(cfg.Name+"-snapshot", image),
		containerd.WithNewSpec(specOpts...),
		containerd.WithContainerLabels(cfg.Labels),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	return container.ID(), nil
}

// StartContainer starts a container
func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	ctx = c.ctx(ctx)

	container, err := c.cli.LoadContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("container not found: %w", err)
	}

	// Create task (the running process)
	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	if err := task.Start(ctx); err != nil {
		return fmt.Errorf("failed to start task: %w", err)
	}

	return nil
}

// StopContainer stops a container
func (c *Client) StopContainer(ctx context.Context, containerID string) error {
	ctx = c.ctx(ctx)

	container, err := c.cli.LoadContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("container not found: %w", err)
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		return nil // No running task
	}

	// Send SIGTERM
	if err := task.Kill(ctx, syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to kill task: %w", err)
	}

	// Wait for exit with timeout
	exitCh, err := task.Wait(ctx)
	if err != nil {
		return err
	}

	select {
	case <-exitCh:
	case <-time.After(10 * time.Second):
		task.Kill(ctx, syscall.SIGKILL)
	}

	_, err = task.Delete(ctx)
	return err
}

// RemoveContainer removes a container
func (c *Client) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	ctx = c.ctx(ctx)

	container, err := c.cli.LoadContainer(ctx, containerID)
	if err != nil {
		return nil // Already removed
	}

	// Stop task if running
	if task, err := container.Task(ctx, nil); err == nil {
		if force {
			task.Kill(ctx, syscall.SIGKILL)
		}
		task.Delete(ctx, containerd.WithProcessKill)
	}

	return container.Delete(ctx, containerd.WithSnapshotCleanup)
}

// GetContainerStatus returns the container's running status
func (c *Client) GetContainerStatus(ctx context.Context, containerID string) (string, error) {
	ctx = c.ctx(ctx)

	container, err := c.cli.LoadContainer(ctx, containerID)
	if err != nil {
		return "error", nil
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		return "stopped", nil
	}

	status, err := task.Status(ctx)
	if err != nil {
		return "error", nil
	}

	switch status.Status {
	case containerd.Running:
		return "running", nil
	case containerd.Created, containerd.Pausing:
		return "creating", nil
	case containerd.Stopped, containerd.Paused:
		return "stopped", nil
	default:
		return "error", nil
	}
}

// GetContainerStats returns container resource usage statistics
func (c *Client) GetContainerStats(ctx context.Context, containerID string) (*types.ContainerStats, error) {
	ctx = c.ctx(ctx)

	container, err := c.cli.LoadContainer(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("container not found: %w", err)
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("no running task: %w", err)
	}

	metrics, err := task.Metrics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	// Parse metrics (containerd returns protobuf)
	_ = metrics // TODO: Parse containerd metrics properly
	
	// Return basic stats for now
	return &types.ContainerStats{
		CPUPercent:    0,
		MemoryUsage:   0,
		MemoryLimit:   0,
		MemoryPercent: 0,
		NetworkRx:     0,
		NetworkTx:     0,
	}, nil
}

// GetContainerLogs retrieves the last N lines of container logs
func (c *Client) GetContainerLogs(ctx context.Context, containerID string, tail int) (string, error) {
	// containerd doesn't store logs like Docker
	// Applications should use a logging driver
	return "", fmt.Errorf("containerd does not support log retrieval directly; use a logging driver")
}

// ListContainers lists all DBNest-managed containers
func (c *Client) ListContainers(ctx context.Context) ([]string, error) {
	ctx = c.ctx(ctx)

	containers, err := c.cli.Containers(ctx, "labels.\"dbnest.managed\"==true")
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, container := range containers {
		ids = append(ids, container.ID())
	}
	return ids, nil
}

// ListNetworks returns all available networks
// Note: containerd uses CNI for networking, this is a simplified implementation
func (c *Client) ListNetworks(ctx context.Context) ([]types.NetworkInfo, error) {
	// containerd uses CNI plugins, not built-in networking
	return []types.NetworkInfo{
		{ID: "default", Name: "bridge", Driver: "cni"},
	}, nil
}

// CreateNetwork creates a new network
// Note: For containerd, networks are managed via CNI configuration files
func (c *Client) CreateNetwork(ctx context.Context, name string) (*types.NetworkInfo, error) {
	// CNI networks are configured via files, not API
	return &types.NetworkInfo{
		ID:     name,
		Name:   name,
		Driver: "cni",
	}, nil
}

// DeleteNetwork removes a network
func (c *Client) DeleteNetwork(ctx context.Context, networkID string) error {
	// CNI networks are configured via files
	return nil
}

// ExecInContainer executes a command in a container
func (c *Client) ExecInContainer(ctx context.Context, containerID string, cmd []string) (string, error) {
	return c.Exec(ctx, containerID, cmd, nil)
}

// Exec executes a command in a container with environment variables
func (c *Client) Exec(ctx context.Context, containerID string, cmd []string, env []string) (string, error) {
	ctx = c.ctx(ctx)

	container, err := c.cli.LoadContainer(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("container not found: %w", err)
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("no running task: %w", err)
	}

	var stdout, stderr strings.Builder
	
	execID := fmt.Sprintf("exec-%d", time.Now().UnixNano())
	process, err := task.Exec(ctx, execID, &specs.Process{
		Args: cmd,
		Env:  env,
		Cwd:  "/",
	}, cio.NewCreator(
		cio.WithStreams(nil, &stdout, &stderr),
	))
	if err != nil {
		return "", fmt.Errorf("failed to exec: %w", err)
	}

	if err := process.Start(ctx); err != nil {
		return "", fmt.Errorf("failed to start exec: %w", err)
	}

	exitCh, err := process.Wait(ctx)
	if err != nil {
		return "", err
	}
	<-exitCh

	process.Delete(ctx)

	if stderr.Len() > 0 {
		return "", fmt.Errorf("exec error: %s", stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// ExecWithStdin executes a command with stdin input
func (c *Client) ExecWithStdin(ctx context.Context, containerID string, cmd []string, stdin []byte, env []string) (string, error) {
	ctx = c.ctx(ctx)

	container, err := c.cli.LoadContainer(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("container not found: %w", err)
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("no running task: %w", err)
	}

	var stdout, stderr strings.Builder
	stdinReader := strings.NewReader(string(stdin))
	
	execID := fmt.Sprintf("exec-%d", time.Now().UnixNano())
	process, err := task.Exec(ctx, execID, &specs.Process{
		Args: cmd,
		Env:  env,
		Cwd:  "/",
	}, cio.NewCreator(
		cio.WithStreams(io.NopCloser(stdinReader), &stdout, &stderr),
	))
	if err != nil {
		return "", fmt.Errorf("failed to exec: %w", err)
	}

	if err := process.Start(ctx); err != nil {
		return "", fmt.Errorf("failed to start exec: %w", err)
	}

	exitCh, err := process.Wait(ctx)
	if err != nil {
		return "", err
	}
	<-exitCh

	process.Delete(ctx)

	return strings.TrimSpace(stdout.String()), nil
}

// UpdateContainerResources updates memory and CPU limits for a running container
func (c *Client) UpdateContainerResources(ctx context.Context, containerID string, memoryLimit int64, cpuLimit float64) error {
	// containerd doesn't support live resource updates easily
	// This would require updating the container spec and restarting
	return fmt.Errorf("live resource updates not supported with containerd; restart container with new limits")
}

// DeleteVolume removes a volume (emulated for containerd)
func (c *Client) DeleteVolume(ctx context.Context, name string) error {
	volPath := filepath.Join("/var/lib/dbnest/volumes", name)
	if err := os.RemoveAll(volPath); err != nil {
		return fmt.Errorf("failed to remove volume directory %s: %w", volPath, err)
	}
	return nil
}
