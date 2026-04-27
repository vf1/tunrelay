package tunep

import (
	"context"
	"os"

	"tunrelay/internal/config"
)

type Egress struct {
	f *os.File
}

func NewEgress(cfg config.TunEgress, log Logger) (*Egress, error) {
	f, err := createTun(cfg.TunEndpoint, log)
	if err != nil {
		return nil, err
	}
	return &Egress{f}, nil
}

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
