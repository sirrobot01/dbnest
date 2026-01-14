// Package runtime provides the container runtime abstraction layer.
// It re-exports types from runtime/types for convenience.
package runtime

import (
	"github.com/sirrobot01/dbnest/pkg/runtime/types"
)

// Re-export types for external users
type (
	Client          = types.Client
	ContainerConfig = types.ContainerConfig
	ContainerStats  = types.ContainerStats
	NetworkInfo     = types.NetworkInfo
)
