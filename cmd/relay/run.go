package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"tunrelay/internal/config"
	"tunrelay/internal/sysctl"
	"tunrelay/internal/tunctl"
)

func getConfig() (*config.Config, error) {
	configFile := flag.String("config", "", "yaml confing file pathname")
	flag.Parse()

	if *configFile == "" {
		return nil, fmt.Errorf("param --config reqired")
	}

	yaml, err := os.ReadFile(*configFile)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg, err := config.UnmarshalConfig([]byte(yaml))
	if err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return cfg, nil
}

func waitForSigterm() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	signal.Stop(ch)
}

func setupSystem(cfg config.System) (func(), error) {
	var orig int
	var err error
	if cfg.IPForward {
		orig, err = sysctl.IPForward()
		if err != nil {
			return nil, fmt.Errorf("get ip_forward: %w", err)
		}
		if orig != 1 {
			err := sysctl.SetIPForward(1)
			if err != nil {
				return nil, fmt.Errorf("set ip_forward: %w", err)
			}
			slog.Info("ip_forward enabled")
		}
		defer func() {
			if err != nil {
				_ = sysctl.SetIPForward(orig)
			}
		}()
	}

	if cfg.Masquerade.SAddr != "" {
		err = tunctl.EnableMasquerade(cfg.Masquerade.SAddr, cfg.Masquerade.OIFName)
		if err != nil {
			return nil, fmt.Errorf("masquerade: %w", err)
		}
		slog.Info("system nat masquerade", "saddr", cfg.Masquerade.SAddr)
		defer func() {
			if err != nil {
				_ = tunctl.DisableMasquerade(cfg.Masquerade.SAddr, cfg.Masquerade.OIFName)
			}
		}()
	}

	if cfg.DefaultRoute.Tun != "" {
		if cfg.DefaultRoute.Except == "" {
			return nil, fmt.Errorf("route except not specified")
		}
		err = tunctl.RouteAllToTun(cfg.DefaultRoute.Tun, cfg.DefaultRoute.Except)
		if err != nil {
			return nil, fmt.Errorf("default route: %w", err)
		}
		slog.Info("default route", "tun", cfg.DefaultRoute.Tun)
	}

	var runned bool
	rest := func() {
		if runned {
			return
		}
		runned = true
		if cfg.IPForward && orig != 1 {
			err := sysctl.SetIPForward(orig)
			if err != nil {
				slog.Error(fmt.Sprintf("set ip_forward: %v", err))
			}
			slog.Info("ip_forward disabled")
		}
		if cfg.Masquerade.SAddr != "" {
			err := tunctl.DisableMasquerade(cfg.Masquerade.SAddr, cfg.Masquerade.OIFName)
			if err != nil {
				slog.Error(fmt.Sprintf("disable nat masquerade: %v", err))
			}
			slog.Info("masquerade disabled")
		}
		if cfg.DefaultRoute.Tun != "" {
			_ = tunctl.DeleteRouteAll(cfg.DefaultRoute.Except)
			slog.Info("routes deleted")
		}
	}

	return rest, nil
}
