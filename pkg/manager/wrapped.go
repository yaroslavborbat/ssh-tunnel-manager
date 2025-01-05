package manager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"ssh-tunell-manager/pkg/config"
	"ssh-tunell-manager/pkg/logger"
)

const (
	sshCMD        = "ssh"
	sshAddExp     = "ssh-add.exp"
	sshAgentCmd   = "ssh-agent"
	passPhraseEnv = "PASSPHRASE"
)

var _ SSHTunnelManager = &wrappedSSHTunnelManager{}

type wrappedSSHTunnelManager struct {
	tunnels []config.Tunnel
}

func newWrappedSSHTunnelManager(tunnels []config.Tunnel) *wrappedSSHTunnelManager {
	return &wrappedSSHTunnelManager{
		tunnels: tunnels,
	}
}

func (m *wrappedSSHTunnelManager) Run(ctx context.Context) error {
	slog.Info("Start ssh-agent")
	if err := m.startAgent(ctx); err != nil {
		return err
	}

	type phraseKey struct {
		phrase string
		key    string
	}
	phraseKeyMap := make(map[phraseKey]struct{})
	for _, t := range m.tunnels {
		pk := phraseKey{
			phrase: t.PassPhrasePath,
			key:    t.PrivateKeyPath,
		}
		if _, ok := phraseKeyMap[pk]; ok {
			continue
		}
		phraseKeyMap[pk] = struct{}{}
		slog.Info("exec ssh-add", slog.String("key", pk.key), slog.String("phrase", pk.phrase))
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
					if errors.Is(ctx.Err(), context.Canceled) {
						return
					}
					log.Error("error running tunnel", logger.SlogErr(err))
				}
				log.Info("SSHTunnel finished. Rerun...")
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
	if err = os.Setenv("SSH_AUTH_SOCK", sshAuthSock); err != nil {
		return fmt.Errorf("error setting SSH_AUTH_SOCK: %w", err)
	}
	if err = os.Setenv("SSH_AGENT_PID", strconv.Itoa(sshAgentPID)); err != nil {
		return fmt.Errorf("error setting SSH_AGENT_PID: %w", err)
	}

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
	env = append(env,
		fmt.Sprintf("%s=%s", passPhraseEnv, phrase),
	)
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
	return cmd.Run()
}

// Example ssh -N user@example-host -L 127.0.0.1:2001:192.168.0.10:6443
func (m *wrappedSSHTunnelManager) makeSShTunelCommandArgs(tunnel *config.Tunnel) (string, []string) {
	name := sshCMD
	args := []string{
		"-o",
		"StrictHostKeyChecking=no",
		"-N",
		fmt.Sprintf("%s@%s", tunnel.User, tunnel.Host),
		"-L",
		fmt.Sprintf("%s:%d:%s:%d", tunnel.BindIP, tunnel.BindPort, tunnel.HostIP, tunnel.HostPort),
	}
	if tunnel.PrivateKeyPath != "" {
		args = append(args, "-i", tunnel.PrivateKeyPath)
	}
	return name, args
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
