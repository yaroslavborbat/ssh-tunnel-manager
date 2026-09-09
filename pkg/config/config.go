package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// ErrInvalidConfig indicates that configuration validation failed.
var ErrInvalidConfig = errors.New("config is not valid")

const DefaultPath = "~/.ssh-tunnel-manager.yaml"

type Type string

const (
	Native  Type = "native"
	Wrapped Type = "wrapped"
)

// ForwardType identifies the SSH port-forwarding mode.
type ForwardType string

const (
	// Local forwards a local TCP port to a host and port reachable from the SSH server.
	Local ForwardType = "local"
	// Dynamic exposes a local SOCKS proxy through the SSH server.
	Dynamic ForwardType = "dynamic"
)

type Config struct {
	Type                         Type          `yaml:"type"`
	DefaultUser                  string        `yaml:"defaultUser"`
	DefaultBindIP                string        `yaml:"defaultBindIP"`
	DefaultPrivateKeyPath        string        `yaml:"defaultPrivateKeyPath"`
	DefaultPassPhrasePath        string        `yaml:"defaultPassPhrasePath"`
	DefaultUseSSHAgent           bool          `yaml:"defaultUseSSHAgent"`
	DefaultConnectionTimeout     time.Duration `yaml:"-"`
	DefaultConnectionTimeoutText string        `yaml:"defaultConnectionTimeout"`
	Tunnels                      []Tunnel      `yaml:"tunnels"`
}

type Tunnel struct {
	Name                  string        `yaml:"name"`
	ForwardType           ForwardType   `yaml:"forwardType"`
	User                  string        `yaml:"user"`
	Host                  string        `yaml:"host"`
	HostIP                string        `yaml:"hostIP"`
	HostPort              int           `yaml:"hostPort"`
	BindIP                string        `yaml:"bindIP"`
	BindPort              int           `yaml:"bindPort"`
	PrivateKeyPath        string        `yaml:"privateKeyPath"`
	PassPhrasePath        string        `yaml:"passPhrasePath"`
	UseSSHAgent           *bool         `yaml:"useSSHAgent,omitempty"`
	SSHAgent              bool          `yaml:"-"`
	ConnectionTimeout     time.Duration `yaml:"-"`
	ConnectionTimeoutText string        `yaml:"connectionTimeout"`
}

func (c *Config) Validate() error {
	if c.DefaultConnectionTimeout < 0 {
		return fmt.Errorf("defaultConnectionTimeout must not be negative: %w", ErrInvalidConfig)
	}

	switch c.Type {
	case Native, Wrapped:
	default:
		return fmt.Errorf("unknown type %q: %w", c.Type, ErrInvalidConfig)
	}

	var errs error
	names := make(map[string]struct{})
	for _, t := range c.Tunnels {
		if err := t.Validate(); err != nil {
			errs = errors.Join(errs, fmt.Errorf("tunnel %q: %w", t.Name, err))
		}
		if c.Type == Native && t.ForwardType == Dynamic {
			errs = errors.Join(errs, fmt.Errorf("tunnel %q: dynamic forwarding requires wrapped type", t.Name))
		}
		names[t.Name] = struct{}{}
	}
	if errs != nil {
		return fmt.Errorf("%w: %w", ErrInvalidConfig, errs)
	}
	if len(names) != len(c.Tunnels) {
		return fmt.Errorf("%w: overlapping tunnel names", ErrInvalidConfig)
	}
	return nil
}

func (t *Tunnel) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("name is required")
	}
	if t.User == "" {
		return fmt.Errorf("user is required")
	}
	if t.Host == "" {
		return fmt.Errorf("host is required")
	}
	switch t.ForwardType {
	case Local:
		if t.HostIP == "" {
			return fmt.Errorf("hostIP is required")
		}
		if t.HostPort <= 0 {
			return fmt.Errorf("hostPort is required and must be greater than 0")
		}
	case Dynamic:
	default:
		return fmt.Errorf("unknown forwardType %q", t.ForwardType)
	}
	if t.BindIP == "" {
		return fmt.Errorf("bindIP is required")
	}
	if t.BindPort <= 0 {
		return fmt.Errorf("bindPort is required and must be greater than 0")
	}
	if t.ConnectionTimeout < 0 {
		return fmt.Errorf("connectionTimeout must not be negative")
	}
	if t.SSHAgent {
		return nil
	}
	if t.PrivateKeyPath != "" {
		if err := checkFile(t.PrivateKeyPath); err != nil {
			return err
		}
	}
	if t.PassPhrasePath != "" {
		if err := checkFile(t.PassPhrasePath); err != nil {
			return err
		}
		if t.PrivateKeyPath == "" {
			return fmt.Errorf("privateKeyPath is required if passPhraseFile defined")
		}
	}
	return nil
}

func (c *Config) Marshal() (out []byte, err error) {
	return yaml.Marshal(c)
}

func Load(path string) (*Config, error) {
	configPath := DefaultPath
	if path != "" {
		configPath = path
	}
	bytes, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	config := &Config{
		Type: Native,
	}
	err = yaml.Unmarshal(bytes, config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file: %w", err)
	}
	config.DefaultConnectionTimeout, err = parseDuration(
		"defaultConnectionTimeout",
		config.DefaultConnectionTimeoutText,
	)
	if err != nil {
		return nil, err
	}
	for i := range config.Tunnels {
		t := &config.Tunnels[i]
		if t.ForwardType == "" {
			t.ForwardType = Local
		}
		if t.User == "" {
			t.User = config.DefaultUser
		}
		if t.BindIP == "" {
			t.BindIP = config.DefaultBindIP
		}
		t.SSHAgent = config.DefaultUseSSHAgent
		if t.UseSSHAgent != nil {
			t.SSHAgent = *t.UseSSHAgent
		}
		if t.ConnectionTimeoutText == "" {
			t.ConnectionTimeout = config.DefaultConnectionTimeout
		} else {
			t.ConnectionTimeout, err = parseDuration("tunnel "+t.Name+" connectionTimeout", t.ConnectionTimeoutText)
			if err != nil {
				return nil, err
			}
		}
		if !t.SSHAgent {
			if t.PassPhrasePath == "" {
				t.PassPhrasePath = config.DefaultPassPhrasePath
			}
			if t.PrivateKeyPath == "" {
				t.PrivateKeyPath = config.DefaultPrivateKeyPath
			}
		}
	}
	return config, nil
}

func parseDuration(field, value string) (time.Duration, error) {
	if value == "" {
		return 0, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", field, value, err)
	}
	return duration, nil
}

func checkFile(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("cannot access file %q: %w", path, err)
	}
	return nil
}
