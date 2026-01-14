package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/sirrobot01/dbnest/pkg/runtime/types"
)

// Client wraps the Docker SDK client
type Client struct {
	cli     *client.Client
	network string
}

// Verify Client implements types.Client interface
var _ types.Client = (*Client)(nil)

// NewClient creates a new Docker SDK client
func NewClient(socketPath string, networkName string) (*Client, error) {
	host := "unix://" + socketPath

	cli, err := client.NewClientWithOpts(
		client.WithHost(host),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	c := &Client{
		cli:     cli,
		network: networkName,
	}

	// Ensure network exists
	if err := c.ensureNetwork(context.Background()); err != nil {
		cli.Close()
		return nil, err
	}

	return c, nil
}

// Close closes the Docker client
func (c *Client) Close() error {
	return c.cli.Close()
}

// Ping checks if Docker is accessible
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.cli.Ping(ctx)
	return err
}

// ensureNetwork creates the DBNest network if it doesn't exist
func (c *Client) ensureNetwork(ctx context.Context) error {
	networks, err := c.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return err
	}

	for _, n := range networks {
		if n.Name == c.network {
			return nil
		}
	}

	_, err = c.cli.NetworkCreate(ctx, c.network, network.CreateOptions{
		Driver: "bridge",
		Labels: map[string]string{
			"dbnest.managed": "true",
		},
	})
	return err
}

// PullImage pulls a Docker image
func (c *Client) PullImage(ctx context.Context, imageName string) error {
	reader, err := c.cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", imageName, err)
	}
	defer reader.Close()
	_, err = io.Copy(io.Discard, reader)
	return err
}

// CreateContainer creates a new container
func (c *Client) CreateContainer(ctx context.Context, cfg *types.ContainerConfig) (string, error) {
	exposedPorts := nat.PortSet{}
	portBindings := nat.PortMap{}

	for containerPort, hostPort := range cfg.PortBindings {
		port := nat.Port(containerPort)
		exposedPorts[port] = struct{}{}
		portBindings[port] = []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: hostPort},
		}
	}

	var mounts []mount.Mount
	for source, containerPath := range cfg.Volumes {
		// Determine mount type: named volume vs bind mount
		mountType := mount.TypeBind
		if !strings.HasPrefix(source, "/") && !strings.HasPrefix(source, ".") {
			// Named volume (e.g., "dbnest-vol-xxx")
			mountType = mount.TypeVolume
		}
		mounts = append(mounts, mount.Mount{
			Type:   mountType,
			Source: source,
			Target: containerPath,
		})
	}

	containerCfg := &container.Config{
		Image:        cfg.Image,
		Cmd:          cfg.Cmd,
		Env:          cfg.Env,
		ExposedPorts: exposedPorts,
		Labels:       cfg.Labels,
	}

	hostCfg := &container.HostConfig{
		PortBindings:  portBindings,
		Mounts:        mounts,
		NetworkMode:   container.NetworkMode(c.network),
		RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
	}

	if cfg.MemoryLimit > 0 {
		hostCfg.Memory = cfg.MemoryLimit
	}
	if cfg.CPULimit > 0 {
		hostCfg.NanoCPUs = int64(cfg.CPULimit * 1e9)
	}

	resp, err := c.cli.ContainerCreate(ctx, containerCfg, hostCfg, nil, nil, cfg.Name)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	return resp.ID, nil
}

// StartContainer starts a container
func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	return c.cli.ContainerStart(ctx, containerID, container.StartOptions{})
}

// StopContainer stops a container
func (c *Client) StopContainer(ctx context.Context, containerID string) error {
	timeout := 10
	return c.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})
}

// RemoveContainer removes a container
func (c *Client) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	return c.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force:         force,
		RemoveVolumes: true,
	})
}

// GetContainerStatus returns the container's running status
func (c *Client) GetContainerStatus(ctx context.Context, containerID string) (string, error) {
	info, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		if client.IsErrNotFound(err) {
			return "error", nil
		}
		return "", err
	}

	if info.State.Running {
		return "running", nil
	}
	if info.State.Paused {
		return "stopped", nil
	}
	if info.State.Restarting {
		return "creating", nil
	}
	if info.State.Dead || info.State.OOMKilled {
		return "error", nil
	}
	return "stopped", nil
}

// GetContainerStats returns container resource usage statistics
func (c *Client) GetContainerStats(ctx context.Context, containerID string) (*types.ContainerStats, error) {
	stats, err := c.cli.ContainerStatsOneShot(ctx, containerID)
	if err != nil {
		return nil, err
	}
	defer stats.Body.Close()

	var statsJSON struct {
		CPUStats struct {
			CPUUsage struct {
				TotalUsage int64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage int64 `json:"system_cpu_usage"`
			OnlineCPUs     int   `json:"online_cpus"`
		} `json:"cpu_stats"`
		PreCPUStats struct {
			CPUUsage struct {
				TotalUsage int64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage int64 `json:"system_cpu_usage"`
		} `json:"precpu_stats"`
		MemoryStats struct {
			Usage int64 `json:"usage"`
			Limit int64 `json:"limit"`
		} `json:"memory_stats"`
		Networks map[string]struct {
			RxBytes int64 `json:"rx_bytes"`
			TxBytes int64 `json:"tx_bytes"`
		} `json:"networks"`
	}

	if err := json.NewDecoder(stats.Body).Decode(&statsJSON); err != nil {
		return nil, fmt.Errorf("failed to decode stats: %w", err)
	}

	cpuDelta := float64(statsJSON.CPUStats.CPUUsage.TotalUsage - statsJSON.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(statsJSON.CPUStats.SystemCPUUsage - statsJSON.PreCPUStats.SystemCPUUsage)
	cpuPercent := 0.0
	if systemDelta > 0 && cpuDelta > 0 {
		numCPUs := statsJSON.CPUStats.OnlineCPUs
		if numCPUs == 0 {
			numCPUs = 1
		}
		cpuPercent = (cpuDelta / systemDelta) * float64(numCPUs) * 100.0
	}

	var networkRx, networkTx int64
	for _, net := range statsJSON.Networks {
		networkRx += net.RxBytes
		networkTx += net.TxBytes
	}

	memPercent := 0.0
	if statsJSON.MemoryStats.Limit > 0 {
		memPercent = float64(statsJSON.MemoryStats.Usage) / float64(statsJSON.MemoryStats.Limit) * 100.0
	}

	return &types.ContainerStats{
		CPUPercent:    cpuPercent,
		MemoryUsage:   statsJSON.MemoryStats.Usage,
		MemoryLimit:   statsJSON.MemoryStats.Limit,
		MemoryPercent: memPercent,
		NetworkRx:     networkRx,
		NetworkTx:     networkTx,
	}, nil
}

// GetContainerLogs retrieves the last N lines of container logs
func (c *Client) GetContainerLogs(ctx context.Context, containerID string, tail int) (string, error) {
	if tail <= 0 {
		tail = 100
	}
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprintf("%d", tail),
	}
	reader, err := c.cli.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	output, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// ListContainers lists all DBNest-managed containers
func (c *Client) ListContainers(ctx context.Context) ([]string, error) {
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, ctr := range containers {
		if ctr.Labels["dbnest.managed"] == "true" {
			ids = append(ids, ctr.ID)
		}
	}
	return ids, nil
}

// ListNetworks returns all available Docker networks
func (c *Client) ListNetworks(ctx context.Context) ([]types.NetworkInfo, error) {
	networks, err := c.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result []types.NetworkInfo
	for _, n := range networks {
		result = append(result, types.NetworkInfo{
			ID:     n.ID,
			Name:   n.Name,
			Driver: n.Driver,
		})
	}
	return result, nil
}

// CreateNetwork creates a new Docker bridge network
func (c *Client) CreateNetwork(ctx context.Context, name string) (*types.NetworkInfo, error) {
	resp, err := c.cli.NetworkCreate(ctx, name, network.CreateOptions{
		Driver: "bridge",
		Labels: map[string]string{"dbnest.managed": "true"},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create network %s: %w", name, err)
	}

	return &types.NetworkInfo{
		ID:     resp.ID,
		Name:   name,
		Driver: "bridge",
	}, nil
}

// DeleteNetwork removes a Docker network
func (c *Client) DeleteNetwork(ctx context.Context, networkID string) error {
	if err := c.cli.NetworkRemove(ctx, networkID); err != nil {
		return fmt.Errorf("failed to delete network: %w", err)
	}
	return nil
}

// ExecInContainer executes a command in a container
func (c *Client) ExecInContainer(ctx context.Context, containerID string, cmd []string) (string, error) {
	exec, err := c.cli.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return "", err
	}

	resp, err := c.cli.ContainerExecAttach(ctx, exec.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", err
	}
	defer resp.Close()

	output, err := io.ReadAll(resp.Reader)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// Exec executes a command in a container with environment variables
func (c *Client) Exec(ctx context.Context, containerID string, cmd []string, env []string) (string, error) {
	exec, err := c.cli.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		Cmd:          cmd,
		Env:          env,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return "", err
	}

	resp, err := c.cli.ContainerExecAttach(ctx, exec.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", err
	}
	defer resp.Close()

	output, err := io.ReadAll(resp.Reader)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// ExecWithStdin executes a command with stdin input
func (c *Client) ExecWithStdin(ctx context.Context, containerID string, cmd []string, stdin []byte, env []string) (string, error) {
	exec, err := c.cli.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		Cmd:          cmd,
		Env:          env,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return "", err
	}

	resp, err := c.cli.ContainerExecAttach(ctx, exec.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", err
	}
	defer resp.Close()

	if _, err := resp.Conn.Write(stdin); err != nil {
		return "", fmt.Errorf("failed to write stdin: %w", err)
	}
	resp.CloseWrite()

	output, err := io.ReadAll(resp.Reader)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// UpdateContainerResources updates memory and CPU limits for a running container
func (c *Client) UpdateContainerResources(ctx context.Context, containerID string, memoryLimit int64, cpuLimit float64) error {
	updateConfig := container.UpdateConfig{
		Resources: container.Resources{},
	}

	if memoryLimit > 0 {
		updateConfig.Resources.Memory = memoryLimit
	}
	if cpuLimit > 0 {
		updateConfig.Resources.NanoCPUs = int64(cpuLimit * 1e9)
	}

	_, err := c.cli.ContainerUpdate(ctx, containerID, updateConfig)
	if err != nil {
		return fmt.Errorf("failed to update container resources: %w", err)
	}
	return nil
}

// DeleteVolume removes a Docker volume
func (c *Client) DeleteVolume(ctx context.Context, name string) error {
	return c.cli.VolumeRemove(ctx, name, true)
}
