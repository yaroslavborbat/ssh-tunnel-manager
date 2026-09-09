package manager

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"ssh-tunell-manager/pkg/config"
	"ssh-tunell-manager/pkg/logger"
)

const (
	sshCMD           = "ssh"
	sshAddExp        = "ssh-add.exp"
	sshAgentCmd      = "ssh-agent"
	passPhraseEnv    = "PASSPHRASE"
	maxSSHErrorBytes = 8 * 1024
)

var _ SSHTunnelManager = &wrappedSSHTunnelManager{}

type wrappedSSHTunnelManager struct {
	tunnels            []config.Tunnel
	agentSocket        string
	managedAgentSocket string
}

func newWrappedSSHTunnelManager(tunnels []config.Tunnel) *wrappedSSHTunnelManager {
	return &wrappedSSHTunnelManager{
		tunnels: tunnels,
	}
}

func (m *wrappedSSHTunnelManager) Run(ctx context.Context) error {
	for _, tunnel := range m.tunnels {
		if tunnel.SSHAgent {
			m.agentSocket = os.Getenv("SSH_AUTH_SOCK")
			if m.agentSocket == "" {
				return fmt.Errorf("SSH_AUTH_SOCK is required when useSSHAgent is enabled")
			}
			break
		}
	}

	type phraseKey struct {
		phrase string
		key    string
	}
	phraseKeyMap := make(map[phraseKey]struct{})
	for _, t := range m.tunnels {
		if t.SSHAgent || t.PassPhrasePath == "" || t.PrivateKeyPath == "" {
			continue
		}
		pk := phraseKey{
			phrase: t.PassPhrasePath,
			key:    t.PrivateKeyPath,
		}
		if _, ok := phraseKeyMap[pk]; ok {
			continue
		}
		phraseKeyMap[pk] = struct{}{}
		if len(phraseKeyMap) == 1 {
			slog.Info("Start ssh-agent")
			if err := m.startAgent(ctx); err != nil {
				return err
			}
		}
		slog.Info("exec ssh-add", slog.String("key", pk.key))
		if err := m.sshAdd(pk.phrase, pk.key); err != nil {
			return err
		}
	}

	wg := &sync.WaitGroup{}

	for _, t := range m.tunnels {
		wg.Add(1)
		go func() {
			log := slog.With(slog.String("name", t.Name))
			defer log.Info("Stopping SSHTunnel")
			defer wg.Done()
			for {
				log.Info("Starting SSHTunnel")
				err := m.runTunnel(ctx, &t)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					log.Error("error running tunnel", logger.SlogErr(err))
				}
				log.Info("SSHTunnel finished. Rerun...")
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Second):
				}
			}
		}()
	}
	wg.Wait()
	return nil
}

func (m *wrappedSSHTunnelManager) startAgent(ctx context.Context) error {
	cmd := exec.Command(sshAgentCmd, "-s")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("error running ssh-agent: %w", err)
	}
	sshAuthSock, sshAgentPID, err := parseSSHAgentData(string(output))
	if err != nil {
		return fmt.Errorf("error parsing ssh-agent data: %w", err)
	}
	m.managedAgentSocket = sshAuthSock

	process, err := os.FindProcess(sshAgentPID)
	if err != nil {
		return fmt.Errorf("failed to find SSH_AGENT_PID: %w", err)
	}
	go func() {
		defer func() {
			if process != nil {
				if err := process.Kill(); err != nil {
					slog.Error("Failed to kill SSH agent process")
				}
			}
		}()
		<-ctx.Done()
	}()
	return nil
}

func (m *wrappedSSHTunnelManager) sshAdd(passPhrasePath, privateKeyPath string) error {
	if passPhrasePath == "" || privateKeyPath == "" {
		return nil
	}
	b, err := os.ReadFile(passPhrasePath)
	if err != nil {
		return err
	}
	phrase := string(b)
	phrase = strings.Trim(phrase, "\n")

	cmd := exec.Command(sshAddExp, privateKeyPath)
	env := os.Environ()
	env = append(
		env,
		fmt.Sprintf("%s=%s", passPhraseEnv, phrase),
	)
	if m.managedAgentSocket != "" {
		env = setEnvironmentValue(env, "SSH_AUTH_SOCK", m.managedAgentSocket)
	}
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("error running ssh add: %w. out: %s", err, string(out))
	}

	return nil
}

func (m *wrappedSSHTunnelManager) runTunnel(ctx context.Context, tunnel *config.Tunnel) error {
	name, args := m.makeSShTunelCommandArgs(tunnel)
	cmd := exec.CommandContext(ctx, name, args...)
	if tunnel.SSHAgent {
		cmd.Env = setEnvironmentValue(os.Environ(), "SSH_AUTH_SOCK", m.agentSocket)
	} else if m.managedAgentSocket != "" {
		cmd.Env = setEnvironmentValue(os.Environ(), "SSH_AUTH_SOCK", m.managedAgentSocket)
	} else {
		cmd.Env = setEnvironmentValue(os.Environ(), "SSH_AUTH_SOCK", "")
	}

	diagnostics := newTailBuffer(maxSSHErrorBytes)
	cmd.Stdout = io.Discard
	cmd.Stderr = diagnostics
	err := cmd.Run()
	if err == nil {
		return nil
	}

	output := strings.TrimSpace(diagnostics.String())
	if output == "" {
		return fmt.Errorf("ssh command failed without diagnostic output: %w", err)
	}
	return fmt.Errorf("ssh command failed: %w: %s", err, output)
}

// Example ssh -N user@example-host -L 127.0.0.1:2001:192.168.0.10:6443
func (m *wrappedSSHTunnelManager) makeSShTunelCommandArgs(tunnel *config.Tunnel) (string, []string) {
	name := sshCMD
	args := []string{
		"-o",
		"StrictHostKeyChecking=no",
		"-o",
		"ExitOnForwardFailure=yes",
		"-N",
	}
	if tunnel.ForwardType == config.Dynamic {
		args = append(
			args,
			"-D", fmt.Sprintf("%s:%d", tunnel.BindIP, tunnel.BindPort),
			"-C",
		)
	} else {
		args = append(
			args,
			"-L", fmt.Sprintf("%s:%d:%s:%d", tunnel.BindIP, tunnel.BindPort, tunnel.HostIP, tunnel.HostPort),
		)
	}
	if tunnel.ConnectionTimeout > 0 {
		seconds := int((tunnel.ConnectionTimeout-1)/time.Second + 1)
		args = append(args, "-o", fmt.Sprintf("ConnectTimeout=%d", seconds))
	}
	if !tunnel.SSHAgent {
		if m.managedAgentSocket == "" {
			args = append(args, "-o", "IdentityAgent=none")
		}
		if tunnel.PrivateKeyPath != "" {
			args = append(args, "-o", "IdentitiesOnly=yes", "-i", tunnel.PrivateKeyPath)
		}
	}
	args = append(args, fmt.Sprintf("%s@%s", tunnel.User, tunnel.Host))
	return name, args
}

type tailBuffer struct {
	buffer []byte
	limit  int
}

func newTailBuffer(limit int) *tailBuffer {
	return &tailBuffer{limit: limit}
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	written := len(p)
	if len(p) >= b.limit {
		b.buffer = append(b.buffer[:0], p[len(p)-b.limit:]...)
		return written, nil
	}
	if excess := len(b.buffer) + len(p) - b.limit; excess > 0 {
		copy(b.buffer, b.buffer[excess:])
		b.buffer = b.buffer[:len(b.buffer)-excess]
	}
	b.buffer = append(b.buffer, p...)
	return written, nil
}

func (b *tailBuffer) String() string {
	return string(b.buffer)
}

func setEnvironmentValue(environment []string, key, value string) []string {
	prefix := key + "="
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			result = append(result, entry)
		}
	}
	return append(result, prefix+value)
}

var (
	sshAuthSockRe = regexp.MustCompile(`SSH_AUTH_SOCK=([^;]+);`)
	sshAgentPIDRe = regexp.MustCompile(`SSH_AGENT_PID=(\d+);`)
)

func parseSSHAgentData(output string) (string, int, error) {
	sshAuthSockMatch := sshAuthSockRe.FindStringSubmatch(output)
	if len(sshAuthSockMatch) < 2 {
		return "", 0, fmt.Errorf("could not parse SSH_AUTH_SOCK")
	}

	sshAgentPIDMatch := sshAgentPIDRe.FindStringSubmatch(output)
	if len(sshAgentPIDMatch) < 2 {
		return "", 0, fmt.Errorf("could not parse SSH_AGENT_PID")
	}

	sshAgentPID, err := strconv.Atoi(sshAgentPIDMatch[1])
	if err != nil {
		return "", 0, fmt.Errorf("error converting SSH_AGENT_PID to int: %w", err)
	}

	return sshAuthSockMatch[1], sshAgentPID, nil
}
