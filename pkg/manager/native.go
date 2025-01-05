package manager

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/rgzr/sshtun"

	"ssh-tunell-manager/pkg/config"
	"ssh-tunell-manager/pkg/logger"
)

var _ SSHTunnelManager = &nativeSSHTunnelManager{}

type nativeSSHTunnelManager struct {
	tunnels []config.Tunnel
}

func newNativeSSHTunnelManager(tunnels []config.Tunnel) *nativeSSHTunnelManager {
	return &nativeSSHTunnelManager{
		tunnels: tunnels,
	}
}

func (m *nativeSSHTunnelManager) Run(ctx context.Context) error {
	wg := &sync.WaitGroup{}

	for _, t := range m.tunnels {
		tunnel := sshtun.New(t.BindPort, t.Host, t.HostPort)
		tunnel.SetUser(t.User)
		tunnel.SetRemoteHost(t.HostIP)
		tunnel.SetLocalHost(t.BindIP)
		if t.PrivateKeyPath != "" && t.PassPhrasePath != "" {
			b, err := os.ReadFile(t.PassPhrasePath)
			if err != nil {
				return fmt.Errorf("failed to read passphrase file: %w", err)
			}
			phrase := string(b)
			phrase = strings.Trim(phrase, "\n")
			tunnel.SetEncryptedKeyFile(t.PrivateKeyPath, phrase)
		} else if t.PrivateKeyPath != "" {
			tunnel.SetKeyFile(t.PrivateKeyPath)
		}

		wg.Add(1)
		go func() {
			log := slog.With(slog.String("name", t.Name))
			defer log.Info("Stopping SSHTunnel")
			defer wg.Done()

			for {
				log.Info("Starting SSHTunnel")
				err := tunnel.Start(ctx)
				if err != nil {
					log.Error("error running tunnel", logger.SlogErr(err))
				}
				select {
				case <-ctx.Done():
					return
				default:
					log.Info("SSHTunnel finished. Rerun...")
				}
			}
		}()
	}

	wg.Wait()
	return nil
}
