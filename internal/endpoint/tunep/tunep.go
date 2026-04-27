package tunep

import (
	"context"
	"os"

	"tunrelay/internal/config"
)

type Logger interface {
	Info(msg string, args ...any)
}

type Ingress struct {
	f *os.File
}

type Egress struct {
	f *os.File
}

func NewIngress(cfg config.TunIngress, log Logger) (*Ingress, error) {
	f, err := createTun(cfg.TunEndpoint, log)
	if err != nil {
		return nil, err
	}
	return &Ingress{f}, nil
}

func NewEgress(cfg config.TunEgress, log Logger) (*Egress, error) {
	f, err := createTun(cfg.TunEndpoint, log)
	if err != nil {
		return nil, err
	}
	return &Egress{f}, nil
}

func (i *Ingress) Read(ctx context.Context, p []byte, off int) (context.Context, int, error) {
	n, err := read(i.f, p, off)
	return ctx, n, err
}

func (i *Ingress) Write(ctx context.Context, p []byte, off int) (context.Context, int, error) {
	n, err := write(i.f, p, off)
	return ctx, n, err
}

func (i *Ingress) Close() error { return i.f.Close() }
func (_ *Ingress) Name() string { return "tun ingress" }

func (e *Egress) Read(ctx context.Context, p []byte, off int) (context.Context, int, error) {
	n, err := read(e.f, p, off)
	return ctx, n, err
}

func (e *Egress) Write(ctx context.Context, p []byte, off int) (context.Context, int, error) {
	n, err := write(e.f, p, off)
	return ctx, n, err
}

func (e *Egress) Close() error { return e.f.Close() }
func (_ *Egress) Name() string { return "tun egress" }
