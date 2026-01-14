package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/sirrobot01/dbnest/pkg/runtime/cli"
	"github.com/sirrobot01/dbnest/pkg/runtime/containerd"
	"github.com/sirrobot01/dbnest/pkg/runtime/docker"
)

// RuntimeBinary maps runtime names to CLI binaries
var RuntimeBinary = map[string]string{
	"docker":     "docker",
	"podman":     "podman",
	"containerd": "nerdctl",
}

// DefaultSockets maps runtime names to default socket paths
var DefaultSockets = map[string]string{
	"docker":     "/var/run/docker.sock",
	"containerd": "/run/containerd/containerd.sock",
}

// New creates a new container runtime client.
// runtime: "docker", "podman", or "containerd"
// If socketPath is provided and matches the runtime, uses SDK mode.
// Otherwise uses CLI mode with the appropriate binary.
func New(runtime, socketPath, networkName string) (Client, error) {
	// Default to docker
	if runtime == "" {
		runtime = "docker"
	}

	// Validate runtime
	if _, ok := RuntimeBinary[runtime]; !ok {
		return nil, fmt.Errorf("unknown runtime: %s (valid: docker, podman, containerd)", runtime)
	}

	// If socket provided, try SDK mode for supported runtimes
	if socketPath != "" {
		switch runtime {
		case "docker":
			return newDockerSDKClient(socketPath, networkName)
		case "containerd":
			return newContainerdSDKClient(socketPath, networkName)
		}
	}

	// Fall back to CLI mode
	return newCLIClient(runtime, networkName)
}

// newDockerSDKClient validates socket and creates Docker SDK client
func newDockerSDKClient(socketPath, networkName string) (Client, error) {
	if err := validateSocket(socketPath); err != nil {
		return nil, err
	}

	log.Info().
		Str("runtime", "docker").
		Str("mode", "SDK").
		Str("socket", socketPath).
		Msg("Initializing container runtime")

	client, err := docker.NewClient(socketPath, networkName)
	if err != nil {
		return nil, err
	}

	if err := pingWithTimeout(client, socketPath, "docker"); err != nil {
		client.Close()
		return nil, err
	}

	log.Info().
		Str("runtime", "docker").
		Str("socket", socketPath).
		Msg("Container runtime connected successfully")

	return client, nil
}

// newContainerdSDKClient validates socket and creates containerd SDK client
func newContainerdSDKClient(socketPath, networkName string) (Client, error) {
	if err := validateSocket(socketPath); err != nil {
		return nil, err
	}

	log.Info().
		Str("runtime", "containerd").
		Str("mode", "SDK").
		Str("socket", socketPath).
		Msg("Initializing container runtime")

	client, err := containerd.NewClient(socketPath, networkName)
	if err != nil {
		return nil, err
	}

	if err := pingWithTimeout(client, socketPath, "containerd"); err != nil {
		client.Close()
		return nil, err
	}

	log.Info().
		Str("runtime", "containerd").
		Str("socket", socketPath).
		Msg("Container runtime connected successfully")

	return client, nil
}

// newCLIClient validates binary and creates CLI client
func newCLIClient(runtime, networkName string) (Client, error) {
	binary := RuntimeBinary[runtime]

	binaryPath, err := exec.LookPath(binary)
	if err != nil {
		return nil, fmt.Errorf("%s CLI not found in PATH: install %s or specify a socket path", binary, binary)
	}

	log.Info().
		Str("runtime", runtime).
		Str("mode", "CLI").
		Str("binary", binaryPath).
		Msg("Initializing container runtime")

	client, err := cli.NewClient(binary, networkName)
	if err != nil {
		return nil, err
	}

	if err := pingWithTimeout(client, "", runtime); err != nil {
		return nil, err
	}

	log.Info().
		Str("runtime", runtime).
		Str("binary", binaryPath).
		Msg("Container runtime connected successfully")

	return client, nil
}

// validateSocket checks if socket path exists and is accessible
func validateSocket(socketPath string) error {
	info, err := os.Stat(socketPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("socket not found at %s", socketPath)
		}
		return fmt.Errorf("cannot access socket at %s: %w", socketPath, err)
	}

	// Check if it's a socket or symlink to socket
	mode := info.Mode()
	if mode&os.ModeSocket == 0 && mode&os.ModeSymlink == 0 {
		// May still be valid on some systems, continue with warning
		log.Warn().
			Str("socket", socketPath).
			Str("mode", mode.String()).
			Msg("Socket path may not be a Unix socket")
	}

	return nil
}

// pingWithTimeout pings the runtime with a timeout
func pingWithTimeout(client Client, socketPath, runtime string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		if socketPath != "" {
			return fmt.Errorf("cannot connect to %s daemon at %s: %w", runtime, socketPath, err)
		}
		return fmt.Errorf("cannot connect to %s daemon: %w (is %s running?)", runtime, err, runtime)
	}
	return nil
}
