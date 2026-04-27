package tunep

import (
	"fmt"
	"net"
	"os"

	"tunrelay/internal/config"
	"tunrelay/internal/tunctl"

	"golang.org/x/sys/unix"
)

const PrefixSize = 4

func read(f *os.File, p []byte, off int) (int, error) {
	n, err := f.Read(p[off-PrefixSize:])
	if n <= PrefixSize {
		return 0, err
	}
	return n - 4, err
}

func write(f *os.File, p []byte, off int) (int, error) {
	p[0] = 0x00
	p[1] = 0x00
	p[2] = 0x00
	switch p[off] >> 4 {
	case 4:
		p[3] = unix.AF_INET
	case 6:
		p[3] = unix.AF_INET6
	default:
		return 0, fmt.Errorf("unsupported ip version %d", p[off]>>4)
	}
	_, err := f.Write(p[off-PrefixSize:])
	if err != nil {
		return 0, err
	}
	return len(p) - off, nil
}

func createTun(cfg config.TunEndpoint, log Logger) (*os.File, error) {
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
	return f, nil
}
