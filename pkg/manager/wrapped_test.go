package manager

import (
	"slices"
	"testing"
	"time"

	"ssh-tunell-manager/pkg/config"
)

func TestWrappedCommandUsesAgentAndConnectionTimeout(t *testing.T) {
	t.Parallel()

	manager := newWrappedSSHTunnelManager(nil)
	_, args := manager.makeSShTunelCommandArgs(&config.Tunnel{
		User:              "user",
		Host:              "gateway.example.com",
		HostIP:            "127.0.0.1",
		HostPort:          22,
		BindIP:            "127.0.0.1",
		BindPort:          2022,
		PrivateKeyPath:    "/ignored/key",
		SSHAgent:          true,
		ConnectionTimeout: 1500 * time.Millisecond,
	})

	if !containsAdjacent(args, "-o", "ConnectTimeout=2") {
		t.Errorf("arguments %q do not contain rounded connection timeout", args)
	}
	if containsAdjacent(args, "-o", "IdentitiesOnly=no") {
		t.Errorf("arguments %q override SSH config identity selection", args)
	}
	if slices.Contains(args, "-i") {
		t.Errorf("arguments %q unexpectedly contain a private key", args)
	}
}

func TestWrappedDynamicForwardingCommand(t *testing.T) {
	t.Parallel()

	manager := newWrappedSSHTunnelManager(nil)
	name, args := manager.makeSShTunelCommandArgs(&config.Tunnel{
		Name:        "socks",
		ForwardType: config.Dynamic,
		User:        "user",
		Host:        "gateway.example.com",
		BindIP:      "localhost",
		BindPort:    12334,
	})

	want := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "ExitOnForwardFailure=yes",
		"-D", "localhost:12334",
		"-q", "-C", "-N",
		"-o", "IdentityAgent=none",
		"user@gateway.example.com",
	}
	if name != sshCMD {
		t.Errorf("command = %q, want %q", name, sshCMD)
	}
	if !slices.Equal(args, want) {
		t.Errorf("arguments = %q, want %q", args, want)
	}
}

func TestWrappedManagedAgentUsesOnlyConfiguredIdentity(t *testing.T) {
	t.Parallel()

	manager := newWrappedSSHTunnelManager(nil)
	manager.managedAgentSocket = "/tmp/managed-agent.sock"
	_, args := manager.makeSShTunelCommandArgs(&config.Tunnel{
		User:           "user",
		Host:           "gateway.example.com",
		HostIP:         "127.0.0.1",
		HostPort:       22,
		BindIP:         "127.0.0.1",
		BindPort:       2022,
		PrivateKeyPath: "/keys/id_ed25519",
	})

	if containsAdjacent(args, "-o", "IdentityAgent=none") {
		t.Errorf("arguments %q disable the managed SSH agent", args)
	}
	if !containsAdjacent(args, "-o", "IdentitiesOnly=yes") {
		t.Errorf("arguments %q do not restrict managed agent identities", args)
	}
	if !containsAdjacent(args, "-i", "/keys/id_ed25519") {
		t.Errorf("arguments %q do not select the configured identity", args)
	}
}

func TestTailBufferKeepsBoundedSuffix(t *testing.T) {
	t.Parallel()

	buffer := newTailBuffer(5)
	if _, err := buffer.Write([]byte("1234")); err != nil {
		t.Fatalf("first Write() error = %v", err)
	}
	if _, err := buffer.Write([]byte("567")); err != nil {
		t.Fatalf("second Write() error = %v", err)
	}
	if got := buffer.String(); got != "34567" {
		t.Errorf("buffer = %q, want %q", got, "34567")
	}
}

func TestSetEnvironmentValueReplacesExistingValue(t *testing.T) {
	t.Parallel()

	environment := setEnvironmentValue([]string{"PATH=/bin", "SSH_AUTH_SOCK=/old"}, "SSH_AUTH_SOCK", "/new")
	if slices.Contains(environment, "SSH_AUTH_SOCK=/old") {
		t.Errorf("environment %q retains old value", environment)
	}
	if !slices.Contains(environment, "SSH_AUTH_SOCK=/new") {
		t.Errorf("environment %q does not contain new value", environment)
	}
}

func containsAdjacent(values []string, first, second string) bool {
	for i := 0; i < len(values)-1; i++ {
		if values[i] == first && values[i+1] == second {
			return true
		}
	}
	return false
}
