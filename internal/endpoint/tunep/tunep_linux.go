package tunep

import (
	"fmt"
	"os"

	"tunrelay/internal/config"
	"tunrelay/internal/sysctl"
	"tunrelay/internal/tunctl"
)

func read(f *os.File, p []byte, off int) (int, error) {
	return f.Read(p[off:])
}

func write(f *os.File, p []byte, off int) (int, error) {
	return f.Write(p[off:])
}

func createTun(cfg config.TunEndpoint, log Logger) (*os.File, error) {
	f, err := tunctl.CreateTun(cfg.Name)
	if err != nil {
		return nil, fmt.Errorf("create %v: %w", cfg.Name, err)
	}

	if cfg.DisableIPv6Linux {
		sysctl.DisableIPv6(cfg.Name)
	}

	err = tunctl.UpIface(cfg.Name, cfg.CIDR)
	if err != nil {
		return nil, fmt.Errorf("up face %v: %w", cfg.Name, err)
	}

	log.Info("interface created", "name", cfg.Name, "cidr", cfg.CIDR)
	return f, nil
}
