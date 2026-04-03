package staticnat

import (
	"context"
	"fmt"
	"net"

	"tunrelay/internal/config"
	"tunrelay/internal/iptool"
)

type Logger interface {
	Info(msg string, args ...any)
}

type StaticNAT struct {
	forwardSrc  net.IP
	forwardDst  net.IP
	backwardSrc net.IP
	backwardDst net.IP
}

func NewStaticNAT(cfg config.StaticNAT, log Logger) (*StaticNAT, error) {
	forwardSrc, forwardDst, backwardSrc, backwardDst, err := parseNATAddrs(cfg)
	if err != nil {
		return nil, err
	}

	log.Info(
		"static nat",
		"forward_src", forwardSrc.String(),
		"forward_dst", forwardDst.String(),
		"backward_src", backwardSrc.String(),
		"backward_dst", backwardDst.String(),
	)

	return &StaticNAT{
		forwardSrc,
		forwardDst,
		backwardSrc,
		backwardDst,
	}, nil
}

func (n *StaticNAT) Forward(ctx context.Context, packet []byte) (context.Context, error) {
	return ctx, iptool.ReplaceIPs(packet, n.forwardSrc, n.forwardDst)
}

func (n *StaticNAT) Backward(ctx context.Context, packet []byte) (context.Context, error) {
	return ctx, iptool.ReplaceIPs(packet, n.backwardSrc, n.backwardDst)
}

func (_ *StaticNAT) Name() string {
	return "static nat"
}

func parseNATAddrs(cfg config.StaticNAT) (net.IP, net.IP, net.IP, net.IP, error) {
	forwardSrc := net.ParseIP(cfg.ForwardSrc)
	forwardDst := net.ParseIP(cfg.ForwardDst)
	backwardSrc := net.ParseIP(cfg.BackwardSrc)
	backwardDst := net.ParseIP(cfg.BackwardDst)

	if forwardSrc == nil && cfg.ForwardSrc != "" {
		return nil, nil, nil, nil, fmt.Errorf("parse nat forward src %v", cfg.ForwardSrc)
	}
	if forwardDst == nil && cfg.ForwardDst != "" {
		return nil, nil, nil, nil, fmt.Errorf("parse nat backward dst %v", cfg.ForwardDst)
	}
	if backwardSrc == nil && cfg.BackwardSrc != "" {
		return nil, nil, nil, nil, fmt.Errorf("parse nat forward src %v", cfg.BackwardSrc)
	}
	if backwardDst == nil && cfg.BackwardDst != "" {
		return nil, nil, nil, nil, fmt.Errorf("parse nat backward dst %v", cfg.BackwardDst)
	}

	return forwardSrc, forwardDst, backwardSrc, backwardDst, nil
}
