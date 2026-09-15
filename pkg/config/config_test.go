package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadSSHAgentAndConnectionTimeoutDefaults(t *testing.T) {
	t.Parallel()

	path := writeTestConfig(t, `
type: native
defaultUser: user
defaultBindIP: 127.0.0.1
defaultUseSSHAgent: true
defaultConnectionTimeout: 15s
tunnels:
  - name: inherited
    host: gateway.example.com
    hostIP: 127.0.0.1
    hostPort: 22
    bindPort: 2022
  - name: overridden
    host: gateway.example.com
    hostIP: 127.0.0.1
    hostPort: 22
    bindPort: 2023
    useSSHAgent: false
    connectionTimeout: 1500ms
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.Tunnels[0].SSHAgent {
		t.Error("defaultUseSSHAgent was not inherited")
	}
	if got := cfg.Tunnels[0].ConnectionTimeout; got != 15*time.Second {
		t.Errorf("inherited connection timeout = %v, want %v", got, 15*time.Second)
	}
	if cfg.Tunnels[1].SSHAgent {
		t.Error("useSSHAgent override was not applied")
	}
	if got := cfg.Tunnels[1].ConnectionTimeout; got != 1500*time.Millisecond {
		t.Errorf("overridden connection timeout = %v, want %v", got, 1500*time.Millisecond)
	}
}

func TestLoadRejectsInvalidConnectionTimeout(t *testing.T) {
	t.Parallel()

	path := writeTestConfig(t, "type: native\ndefaultConnectionTimeout: soon\n")
	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil, want invalid duration error")
	}
}

func TestDynamicForwardingValidation(t *testing.T) {
	t.Parallel()

	path := writeTestConfig(t, `
type: wrapped
defaultUser: user
defaultBindIP: localhost
tunnels:
  - name: socks
    forwardType: dynamic
    host: gateway.example.com
    bindPort: 12334
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if cfg.Tunnels[0].ForwardType != Dynamic {
		t.Errorf("forward type = %q, want %q", cfg.Tunnels[0].ForwardType, Dynamic)
	}

	cfg.Type = Native
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want native dynamic forwarding error")
	}
}

func writeTestConfig(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
