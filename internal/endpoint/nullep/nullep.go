package nullep

import "context"

type Endpoing struct {
	EPName string
}

type Logger interface {
	Info(msg string, args ...any)
}

func NewEndpoint(name string, log Logger) *Endpoing {
	log.Info("null endpoint", "name", name)
	return &Endpoing{EPName: name}
}

func (_ *Endpoing) Read(ctx context.Context, b []byte) (context.Context, int, error) {
	select {}
}

func (_ *Endpoing) Write(ctx context.Context, b []byte) (context.Context, int, error) {
	select {}
}

func (_ *Endpoing) Close() error {
	return nil
}

func (e *Endpoing) Name() string {
	return e.EPName
}
