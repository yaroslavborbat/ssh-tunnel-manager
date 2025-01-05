package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"ssh-tunell-manager/pkg/config"
	"ssh-tunell-manager/pkg/logger"
	"ssh-tunell-manager/pkg/manager"
	"ssh-tunell-manager/pkg/signals"
)

type Options struct {
	Config string
}

func (o *Options) Parse(set *flag.FlagSet) {
	set.StringVar(&o.Config, "config", "", "path to ssh-tunnel-manager config file")
	flag.Parse()
}

func NewOptions() *Options {
	return &Options{}
}

func main() {
	opts := NewOptions()
	opts.Parse(flag.CommandLine)

	slog.Info(fmt.Sprintf("Options: %v", opts))

	conf, err := config.Load(opts.Config)
	if err != nil {
		slog.Error("", logger.SlogErr(err))
		os.Exit(1)
	}

	err = conf.Validate()
	if err != nil {
		slog.Error("", logger.SlogErr(err))
		os.Exit(1)
	}

	slog.Info(fmt.Sprintf("Configuration: %v", conf))

	ctx := signals.SetupSignalHandler()

	mgr := manager.NewSSHTunnelManager(conf.Tunnels, conf.StartSSHAgent)
	err = mgr.Run(ctx)
	if err != nil {
		slog.Error("", logger.SlogErr(err))
		os.Exit(1)
	}
}
