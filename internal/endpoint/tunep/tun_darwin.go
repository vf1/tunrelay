package tunep

import (
	"fmt"
	"net"

	"tunrelay/internal/config"
	"tunrelay/internal/tunctl"

	"github.com/songgao/water"
)

func createTun(cfg config.TunEndpoint, log Logger) (*water.Interface, error) {
	name := "u" + cfg.Name

	tun, err := tunctl.CreateTun(name)
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
	return tun, nil
}
