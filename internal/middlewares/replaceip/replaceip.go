package replaceip

import (
	"context"
	"fmt"
	"net/netip"

	"tunrelay/internal/config"
	"tunrelay/internal/iptool"
)

type Logger interface {
	Info(msg string, args ...any)
}

type StaticNAT struct {
	forwardSrc  netip.Prefix
	forwardDst  netip.Prefix
	backwardSrc netip.Prefix
	backwardDst netip.Prefix
}

func NewReplaceIP(cfg config.ReplaceIP, log Logger) (*StaticNAT, error) {
	forwardSrc, forwardDst, backwardSrc, backwardDst, err := parseNATAddrs(cfg)
	if err != nil {
		return nil, err
	}

	log.Info(
		"replace ip",
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
	return ctx, iptool.ReplaceAddrs(packet, n.forwardSrc, n.forwardDst)
}

func (n *StaticNAT) Backward(ctx context.Context, packet []byte) (context.Context, error) {
	return ctx, iptool.ReplaceAddrs(packet, n.backwardSrc, n.backwardDst)
}

func (_ *StaticNAT) Name() string {
	return "static nat"
}

func parseNATAddrs(cfg config.ReplaceIP) (
	forwardSrc netip.Prefix,
	forwardDst netip.Prefix,
	backwardSrc netip.Prefix,
	backwardDst netip.Prefix,
	err error,
) {
	if cfg.ForwardSrc != "" {
		forwardSrc, err = netip.ParsePrefix(cfg.ForwardSrc)
		if err != nil {
			err = fmt.Errorf("parse nat forward src %v: %w", cfg.ForwardSrc, err)
			return
		}
		if !forwardSrc.Addr().Is4() {
			err = fmt.Errorf("forward src not ip v4")
			return
		}
	}

	if cfg.ForwardDst != "" {
		forwardDst, err = netip.ParsePrefix(cfg.ForwardDst)
		if err != nil {
			err = fmt.Errorf("parse nat forward dst %v: %w", cfg.ForwardDst, err)
			return
		}
		if !forwardDst.Addr().Is4() {
			err = fmt.Errorf("forward dst not ip v4")
			return
		}
	}

	if cfg.BackwardSrc != "" {
		backwardSrc, err = netip.ParsePrefix(cfg.BackwardSrc)
		if err != nil {
			err = fmt.Errorf("parse nat backward src %v: %w", cfg.BackwardSrc, err)
			return
		}
		if !backwardSrc.Addr().Is4() {
			err = fmt.Errorf("backward src not ip v4")
			return
		}
	}

	if cfg.BackwardDst != "" {
		backwardDst, err = netip.ParsePrefix(cfg.BackwardDst)
		if err != nil {
			err = fmt.Errorf("parse nat backward dst %v: %w", cfg.BackwardDst, err)
			return
		}
		if !backwardDst.Addr().Is4() {
			err = fmt.Errorf("backward dst not ip v4")
			return
		}
	}

	return
}
