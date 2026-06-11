package tunep

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"

	"tunrelay/internal/config"
	"tunrelay/internal/sysctl"
	"tunrelay/internal/tunctl"
)

type tunDevice struct {
	f *os.File
}

func (i *tunDevice) Read(ctx context.Context, p []byte, off int) (context.Context, int, error) {
	n, err := i.f.Read(p[off:])
	if err != nil {
		if errors.Is(err, os.ErrClosed) {
			return ctx, 0, err
		}
		if errors.Is(err, syscall.EBADF) {
			return ctx, 0, fmt.Errorf("read interrupted by close: %w: %w", os.ErrClosed, err)
		}
		return ctx, 0, err
	}
	return ctx, n, err
}

func (i *tunDevice) Write(ctx context.Context, p []byte, off int) (context.Context, int, error) {
	n, err := i.f.Write(p[off:])
	return ctx, n, err
}

func (i *tunDevice) Close() error { return i.f.Close() }

func createTun(cfg config.TunEndpoint, log Logger) (*tunDevice, error) {
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
	return &tunDevice{f}, nil
}
