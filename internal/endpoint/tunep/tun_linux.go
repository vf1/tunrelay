package tunep

import (
	"fmt"
	"os"

	"tunrelay/internal/config"
	"tunrelay/internal/tunctl"
)

func createTun(cfg config.TunEndpoint, log Logger) (*os.File, error) {
	tun, err := tunctl.CreateTun(cfg.Name)
	if err != nil {
		return nil, fmt.Errorf("create %v: %w", cfg.Name, err)
	}

	err = tunctl.UpIface(cfg.Name, cfg.CIDR)
	if err != nil {
		return nil, fmt.Errorf("up face %v: %w", cfg.Name, err)
	}

	log.Info("interface created", "name", cfg.Name, "cidr", cfg.CIDR)
	return tun, nil
}
