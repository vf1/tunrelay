package tunep

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"

	"tunrelay/internal/config"
	"tunrelay/internal/tunctl"

	"golang.org/x/sys/unix"
)

const PrefixSize = 4

type tunDevice struct {
	f *os.File
}

func (i *tunDevice) Read(ctx context.Context, p []byte, off int) (context.Context, int, error) {
	n, err := i.f.Read(p[off-PrefixSize:])
	if err != nil {
		if errors.Is(err, os.ErrClosed) {
			return ctx, 0, err
		}
		if errors.Is(err, syscall.EBADF) {
			return ctx, 0, fmt.Errorf("read interrupted by close: %w: %w", os.ErrClosed, err)
		}
		return ctx, 0, err
	}
	if n <= PrefixSize {
		return ctx, 0, fmt.Errorf("darwin read less %v bytes", PrefixSize)
	}
	return ctx, n - PrefixSize, err
}

func (i *tunDevice) Write(ctx context.Context, p []byte, off int) (context.Context, int, error) {
	p = p[off-PrefixSize:]
	p[0] = 0x00
	p[1] = 0x00
	p[2] = 0x00
	ver := p[PrefixSize] >> 4
	switch ver {
	case 4:
		p[3] = unix.AF_INET
	case 6:
		p[3] = unix.AF_INET6
	default:
		return ctx, 0, fmt.Errorf("unsupported ip version %d", ver)
	}
	_, err := i.f.Write(p)
	if err != nil {
		return ctx, 0, err
	}
	return ctx, len(p) - off, nil
}

func (i *tunDevice) Close() error { return i.f.Close() }

func createTun(cfg config.TunEndpoint, log Logger) (*tunDevice, error) {
	name := "u" + cfg.Name

	f, err := tunctl.CreateTun(name)
	if err != nil {
		return nil, fmt.Errorf("create %v: %w", name, err)
	}

	localAddr, localNet, err := net.ParseCIDR(cfg.CIDR)
	if err != nil {
		return nil, fmt.Errorf("parse cidr %v: %w", cfg.CIDR, err)
	}
	mask := net.IP(localNet.Mask).String()

	err = tunctl.UpIface(name, localAddr.String(), cfg.Peer, mask)
	if err != nil {
		return nil, fmt.Errorf("up face %v: %w", name, err)
	}

	err = tunctl.AddIfaceRoute(name, cfg.Peer)
	if err != nil {
		return nil, fmt.Errorf("add route %v: %w", name, err)
	}

	log.Info("interface created", "name", name, "local", localAddr, "peer", cfg.Peer, "mask", mask)
	return &tunDevice{f}, nil
}
