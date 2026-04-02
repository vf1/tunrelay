package tunep

import (
	"io"

	"tunrelay/internal/config"
)

type Logger interface {
	Info(msg string, args ...any)
}

type Ingress struct {
	io.ReadWriteCloser
}

type Egress struct {
	io.ReadWriteCloser
}

func NewIngress(cfg config.TunIngress, log Logger) (*Ingress, error) {
	tun, err := createTun(cfg.TunEndpoint, log)
	if err != nil {
		return nil, err
	}
	return &Ingress{tun}, nil
}

func NewEgress(cfg config.TunEgress, log Logger) (*Egress, error) {
	tun, err := createTun(cfg.TunEndpoint, log)
	if err != nil {
		return nil, err
	}

	return &Egress{tun}, nil
}

func (_ *Ingress) Name() string {
	return "tun ingress"
}

func (_ *Egress) Name() string {
	return "tun egress"
}
