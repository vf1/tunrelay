package tunep

import "tunrelay/internal/config"

type Logger interface {
	Info(msg string, args ...any)
}

type Ingress struct {
	tunDevice
}

type Egress struct {
	tunDevice
}

func NewIngress(cfg config.TunIngress, log Logger) (*Ingress, error) {
	d, err := createTun(cfg.TunEndpoint, log)
	if err != nil {
		return nil, err
	}
	return &Ingress{*d}, nil
}

func NewEgress(cfg config.TunEgress, log Logger) (*Egress, error) {
	d, err := createTun(cfg.TunEndpoint, log)
	if err != nil {
		return nil, err
	}
	return &Egress{*d}, nil
}

func (_ *Egress) Name() string  { return "tun egress" }
func (_ *Ingress) Name() string { return "tun ingress" }
