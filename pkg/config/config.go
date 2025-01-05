package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

var IsNotValidErr = errors.New("config is not valid")

const DefaultPath = "~/.ssh-tunnel-manager.yaml"

type Type string

const (
	Native  Type = "native"
	Wrapped Type = "wrapped"
)

type Config struct {
	Type                  Type     `yaml:"type"`
	DefaultUser           string   `yaml:"defaultUser"`
	DefaultBindIP         string   `yaml:"defaultBindIP"`
	DefaultPrivateKeyPath string   `yaml:"defaultPrivateKeyPath"`
	DefaultPassPhrasePath string   `yaml:"defaultPassPhrasePath"`
	Tunnels               []Tunnel `yaml:"tunnels"`
}

type Tunnel struct {
	Name           string `yaml:"name"`
	User           string `yaml:"user"`
	Host           string `yaml:"host"`
	HostIP         string `yaml:"hostIP"`
	HostPort       int    `yaml:"hostPort"`
	BindIP         string `yaml:"bindIP"`
	BindPort       int    `yaml:"bindPort"`
	PrivateKeyPath string `yaml:"privateKeyPath"`
	PassPhrasePath string `yaml:"passPhrasePath"`
}

func (c *Config) Validate() error {
	switch c.Type {
	case Native, Wrapped:
	default:
		return fmt.Errorf("unknown type %q: %w", c.Type, IsNotValidErr)
	}

	var errs error
	names := make(map[string]struct{})
	for _, t := range c.Tunnels {
		if err := t.Validate(); err != nil {
			errs = errors.Join(errs, err)
		}
		names[t.Name] = struct{}{}
	}
	if errs != nil {
		return fmt.Errorf("%w: %s", IsNotValidErr, errs.Error())
	}
	if len(names) != len(c.Tunnels) {
		return fmt.Errorf("%w: overlapping tunnel names", IsNotValidErr)
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
	if t.HostIP == "" {
		return fmt.Errorf("hostIP is required")
	}
	if t.HostPort <= 0 {
		return fmt.Errorf("hostPort is required and must be greater than 0")
	}
	if t.BindIP == "" {
		return fmt.Errorf("bindIP is required")
	}
	if t.BindPort <= 0 {
		return fmt.Errorf("bindPort is required and must be greater than 0")
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
	for i := range config.Tunnels {
		t := &config.Tunnels[i]
		if t.User == "" {
			t.User = config.DefaultUser
		}
		if t.BindIP == "" {
			t.BindIP = config.DefaultBindIP
		}
		if t.PassPhrasePath == "" {
			t.PassPhrasePath = config.DefaultPassPhrasePath
		}
		if t.PrivateKeyPath == "" {
			t.PrivateKeyPath = config.DefaultPrivateKeyPath
		}
	}
	return config, nil
}

func checkFile(path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("file %s does not exist", path)
	}
	return nil
}
