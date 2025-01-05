package manager

import (
	"context"
	"errors"

	"ssh-tunell-manager/pkg/config"
)

type SSHTunnelManager interface {
	Run(ctx context.Context) error
}

func NewSSHTunnelManager(t config.Type, tunnels []config.Tunnel) (SSHTunnelManager, error) {
	switch t {
	case config.Native:
		return newNativeSSHTunnelManager(tunnels), nil
	case config.Wrapped:
		return newWrappedSSHTunnelManager(tunnels), nil
	default:
		return nil, errors.New("unknown Type")
	}
}
