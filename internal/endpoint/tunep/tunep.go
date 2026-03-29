package tunep

import (
	"fmt"
	"io"
	"log/slog"
	"net"

	"tunrelay/internal/config"
	"tunrelay/internal/iptool"
)

type Logger interface {
	Info(msg string, args ...any)
}

type Ingress struct {
	io.ReadWriteCloser
}

type Egress struct {
	*iptool.NAT
}

func NewIngress(cfg config.TunIngress, log Logger) (*Ingress, error) {
	f, err := createTun(cfg.TunEndpoint, log)
	if err != nil {
		return nil, err
	}
	return &Ingress{f}, nil
}

func NewEgress(cfg config.TunEgress, log Logger) (*Egress, error) {
	tun, err := createTun(cfg.TunEndpoint, log)
	if err != nil {
		return nil, err
	}

	forwardSrc, forwardDst, backwardSrc, backwardDst, err := parseNATAddrs(cfg)
	if err != nil {
		return nil, err
	}

	nat := iptool.NewNAT(tun, forwardSrc, forwardDst, backwardSrc, backwardDst)
	return &Egress{nat}, nil
}

func (_ *Ingress) Name() string {
	return "tun ingress"
}

func (_ *Egress) Name() string {
	return "tun egress"
}

func parseNATAddrs(cfg config.TunEgress) (net.IP, net.IP, net.IP, net.IP, error) {
	forwardSrc := net.ParseIP(cfg.NAT.Forward.Src)
	forwardDst := net.ParseIP(cfg.NAT.Forward.Dst)
	backwardSrc := net.ParseIP(cfg.NAT.Backward.Src)
	backwardDst := net.ParseIP(cfg.NAT.Backward.Dst)

	if forwardSrc == nil && cfg.NAT.Forward.Src != "" {
		return nil, nil, nil, nil, fmt.Errorf("parse nat forward src %v", cfg.NAT.Forward.Src)
	}
	if forwardDst == nil && cfg.NAT.Forward.Dst != "" {
		return nil, nil, nil, nil, fmt.Errorf("parse nat backward dst %v", cfg.NAT.Forward.Dst)
	}
	if backwardSrc == nil && cfg.NAT.Backward.Src != "" {
		return nil, nil, nil, nil, fmt.Errorf("parse nat forward src %v", cfg.NAT.Backward.Src)
	}
	if backwardDst == nil && cfg.NAT.Backward.Dst != "" {
		return nil, nil, nil, nil, fmt.Errorf("parse nat backward dst %v", cfg.NAT.Backward.Dst)
	}

	slog.Info(
		"nat",
		"forward_src", forwardSrc,
		"forward_dst", forwardDst,
		"backward_src", backwardSrc,
		"backward_dst", backwardDst,
	)

	return forwardSrc, forwardDst, backwardSrc, backwardDst, nil
}
