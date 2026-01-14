package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/sirrobot01/dbnest/pkg/runtime/types"
)

// Client implements the types.Client interface using container runtime CLIs.
// Supports docker, podman, and nerdctl (containerd).
type Client struct {
	binary  string // Runtime binary: "docker", "podman", or "nerdctl"
	network string
}

// Verify Client implements types.Client interface
var _ types.Client = (*Client)(nil)

// NewClient creates a new CLI client for a container runtime.
// binary should be "docker", "podman", or "nerdctl"
func NewClient(binary, networkName string) (*Client, error) {
	c := &Client{
		binary:  binary,
		network: networkName,
	}

	// Verify CLI is available
	if _, err := exec.LookPath(binary); err != nil {
		return nil, fmt.Errorf("%s CLI not found: %w", binary, err)
	}

	// Ensure network exists
	if err := c.ensureNetwork(context.Background()); err != nil {
		return nil, err
	}

	return c, nil
}

// Close is a no-op for CLI client
func (c *Client) Close() error {
	return nil
}

// runCommand executes a runtime command and returns stdout
func (c *Client) runCommand(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, c.binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s failed: %w, stderr: %s", c.binary, args[0], err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

// ensureNetwork creates the DBNest network if it doesn't exist
func (c *Client) ensureNetwork(ctx context.Context) error {
	_, err := c.runCommand(ctx, "network", "inspect", c.network)
	if err == nil {
		return nil
	}

	_, err = c.runCommand(ctx, "network", "create",
		"--driver", "bridge",
		"--label", "dbnest.managed=true",
		c.network)
	return err
}

// Ping checks if the runtime is accessible
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.runCommand(ctx, "info", "--format", "{{.ID}}")
	return err
}

// PullImage pulls a container image
func (c *Client) PullImage(ctx context.Context, imageName string) error {
	_, err := c.runCommand(ctx, "pull", imageName)
	return err
}

// CreateContainer creates a new container
func (c *Client) CreateContainer(ctx context.Context, cfg *types.ContainerConfig) (string, error) {
	args := []string{"create", "--name", cfg.Name}

	args = append(args, "--network", c.network)

	for _, env := range cfg.Env {
		args = append(args, "-e", env)
	}

	for containerPort, hostPort := range cfg.PortBindings {
		args = append(args, "-p", fmt.Sprintf("%s:%s", hostPort, containerPort))
	}

	for hostPath, containerPath := range cfg.Volumes {
		args = append(args, "-v", fmt.Sprintf("%s:%s", hostPath, containerPath))
	}

	if cfg.MemoryLimit > 0 {
		args = append(args, "--memory", fmt.Sprintf("%d", cfg.MemoryLimit))
	}
	if cfg.CPULimit > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%.2f", cfg.CPULimit))
	}

	for k, v := range cfg.Labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, "--restart", "unless-stopped")
	args = append(args, cfg.Image)

	// Append command args if specified
	if len(cfg.Cmd) > 0 {
		args = append(args, cfg.Cmd...)
	}

	containerID, err := c.runCommand(ctx, args...)
	if err != nil {
		return "", err
	}
	return containerID, nil
}

// StartContainer starts a container
func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	_, err := c.runCommand(ctx, "start", containerID)
	return err
}

// StopContainer stops a container
func (c *Client) StopContainer(ctx context.Context, containerID string) error {
	_, err := c.runCommand(ctx, "stop", "-t", "10", containerID)
	return err
}

// RemoveContainer removes a container
func (c *Client) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	args := []string{"rm", "-v"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, containerID)
	_, err := c.runCommand(ctx, args...)
	return err
}

// GetContainerStatus returns the container's running status
func (c *Client) GetContainerStatus(ctx context.Context, containerID string) (string, error) {
	output, err := c.runCommand(ctx, "inspect", "--format", "{{.State.Status}}", containerID)
	if err != nil {
		if strings.Contains(err.Error(), "No such") {
			return "error", nil
		}
		return "", err
	}

	switch output {
	case "running":
		return "running", nil
	case "paused", "exited", "dead":
		return "stopped", nil
	case "restarting", "created":
		return "creating", nil
	default:
		return "error", nil
	}
}

// GetContainerStats returns container resource usage statistics
func (c *Client) GetContainerStats(ctx context.Context, containerID string) (*types.ContainerStats, error) {
	output, err := c.runCommand(ctx, "stats", "--no-stream", "--format",
		`{"cpu":"{{.CPUPerc}}","mem_usage":"{{.MemUsage}}","net_io":"{{.NetIO}}"}`,
		containerID)
	if err != nil {
		return nil, err
	}

	var raw struct {
		CPU      string `json:"cpu"`
		MemUsage string `json:"mem_usage"`
		NetIO    string `json:"net_io"`
	}
	if err := json.Unmarshal([]byte(output), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse stats: %w", err)
	}

	stats := &types.ContainerStats{}

	if cpu := strings.TrimSuffix(raw.CPU, "%"); cpu != "" {
		if v, err := strconv.ParseFloat(cpu, 64); err == nil {
			stats.CPUPercent = v
		}
	}

	if parts := strings.Split(raw.MemUsage, " / "); len(parts) == 2 {
		stats.MemoryUsage = parseBytes(parts[0])
		stats.MemoryLimit = parseBytes(parts[1])
		if stats.MemoryLimit > 0 {
			stats.MemoryPercent = float64(stats.MemoryUsage) / float64(stats.MemoryLimit) * 100
		}
	}

	if parts := strings.Split(raw.NetIO, " / "); len(parts) == 2 {
		stats.NetworkRx = parseBytes(parts[0])
		stats.NetworkTx = parseBytes(parts[1])
	}

	return stats, nil
}

// parseBytes parses a human-readable byte string like "1.5GiB", "100MiB", "2.3kB"
func parseBytes(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "--" {
		return 0
	}

	re := regexp.MustCompile(`^([\d.]+)\s*([A-Za-z]+)$`)
	matches := re.FindStringSubmatch(s)
	if len(matches) != 3 {
		return 0
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}

	unit := strings.ToLower(matches[2])
	var multiplier float64 = 1

	switch unit {
	case "b":
		multiplier = 1
	case "kb", "kib":
		multiplier = 1024
	case "mb", "mib":
		multiplier = 1024 * 1024
	case "gb", "gib":
		multiplier = 1024 * 1024 * 1024
	case "tb", "tib":
		multiplier = 1024 * 1024 * 1024 * 1024
	}

	return int64(value * multiplier)
}

// GetContainerLogs retrieves the last N lines of container logs
func (c *Client) GetContainerLogs(ctx context.Context, containerID string, tail int) (string, error) {
	if tail <= 0 {
		tail = 100
	}
	return c.runCommand(ctx, "logs", "--tail", fmt.Sprintf("%d", tail), containerID)
}

// ListContainers lists all DBNest-managed containers
func (c *Client) ListContainers(ctx context.Context) ([]string, error) {
	output, err := c.runCommand(ctx, "ps", "-a",
		"--filter", "label=dbnest.managed=true",
		"--format", "{{.ID}}")
	if err != nil {
		return nil, err
	}

	if output == "" {
		return nil, nil
	}
	return strings.Split(output, "\n"), nil
}

// ListNetworks returns all available networks
func (c *Client) ListNetworks(ctx context.Context) ([]types.NetworkInfo, error) {
	output, err := c.runCommand(ctx, "network", "ls", "--format", "{{.ID}}\t{{.Name}}\t{{.Driver}}")
	if err != nil {
		return nil, err
	}

	if output == "" {
		return nil, nil
	}

	var networks []types.NetworkInfo
	for _, line := range strings.Split(output, "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) >= 3 {
			networks = append(networks, types.NetworkInfo{
				ID:     parts[0],
				Name:   parts[1],
				Driver: parts[2],
			})
		}
	}
	return networks, nil
}

// CreateNetwork creates a new bridge network
func (c *Client) CreateNetwork(ctx context.Context, name string) (*types.NetworkInfo, error) {
	output, err := c.runCommand(ctx, "network", "create", "--driver", "bridge", "--label", "dbnest.managed=true", name)
	if err != nil {
		return nil, fmt.Errorf("failed to create network %s: %w", name, err)
	}

	networkID := strings.TrimSpace(output)
	return &types.NetworkInfo{
		ID:     networkID,
		Name:   name,
		Driver: "bridge",
	}, nil
}

// DeleteNetwork removes a network
func (c *Client) DeleteNetwork(ctx context.Context, networkID string) error {
	_, err := c.runCommand(ctx, "network", "rm", networkID)
	if err != nil {
		return fmt.Errorf("failed to delete network: %w", err)
	}
	return nil
}

// ExecInContainer executes a command in a container
func (c *Client) ExecInContainer(ctx context.Context, containerID string, cmd []string) (string, error) {
	args := append([]string{"exec", containerID}, cmd...)
	return c.runCommand(ctx, args...)
}

// Exec executes a command in a container with environment variables
func (c *Client) Exec(ctx context.Context, containerID string, cmd []string, env []string) (string, error) {
	args := []string{"exec"}
	for _, e := range env {
		args = append(args, "-e", e)
	}
	args = append(args, containerID)
	args = append(args, cmd...)
	return c.runCommand(ctx, args...)
}

// ExecWithStdin executes a command with stdin input
func (c *Client) ExecWithStdin(ctx context.Context, containerID string, cmd []string, stdin []byte, env []string) (string, error) {
	args := []string{"exec", "-i"}
	for _, e := range env {
		args = append(args, "-e", e)
	}
	args = append(args, containerID)
	args = append(args, cmd...)

	execCmd := exec.CommandContext(ctx, c.binary, args...)
	execCmd.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr

	if err := execCmd.Run(); err != nil {
		return "", fmt.Errorf("%s exec failed: %w, stderr: %s", c.binary, err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

// UpdateContainerResources updates memory and CPU limits for a running container
func (c *Client) UpdateContainerResources(ctx context.Context, containerID string, memoryLimit int64, cpuLimit float64) error {
	args := []string{"update"}

	if memoryLimit > 0 {
		args = append(args, "--memory", fmt.Sprintf("%d", memoryLimit))
	}
	if cpuLimit > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%.2f", cpuLimit))
	}

	args = append(args, containerID)
	_, err := c.runCommand(ctx, args...)
	return err
}

// DeleteVolume removes a volume
func (c *Client) DeleteVolume(ctx context.Context, name string) error {
	_, err := c.runCommand(ctx, "volume", "rm", name)
	return err
}
