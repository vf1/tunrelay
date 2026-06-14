package relay

import (
	"context"
	"errors"
	"testing"

	"tunrelay/internal/config"
)

var errTest = errors.New("test error")

type mockFactory struct {
	ingress    func(config.IngressEndpoint, Logger) (Endpoint, error)
	egress     func(config.EgressEndpoint, Logger) (Endpoint, error)
	middleware func(config.Middleware, Logger) (Middleware, error)
}

func (f mockFactory) NewIngress(cfg config.IngressEndpoint, log Logger) (Endpoint, error) {
	return f.ingress(cfg, log)
}

func (f mockFactory) NewEgress(cfg config.EgressEndpoint, log Logger) (Endpoint, error) {
	return f.egress(cfg, log)
}

func (f mockFactory) NewMiddleware(cfg config.Middleware, log Logger) (Middleware, error) {
	return f.middleware(cfg, log)
}

func defaultMockFactory() mockFactory {
	return mockFactory{
		ingress: func(_ config.IngressEndpoint, _ Logger) (Endpoint, error) {
			return newMockEndpoint("mock-ingress"), nil
		},
		egress: func(_ config.EgressEndpoint, _ Logger) (Endpoint, error) {
			return newMockEndpoint("mock-egress"), nil
		},
		middleware: func(_ config.Middleware, _ Logger) (Middleware, error) {
			return &mockMiddleware{
				name:     "mock-mw",
				forward:  func(ctx context.Context, _ []byte) (context.Context, error) { return ctx, nil },
				backward: func(ctx context.Context, _ []byte) (context.Context, error) { return ctx, nil },
			}, nil
		},
	}
}

func nullRelay() config.Relay {
	return config.Relay{
		Ingress: config.IngressEndpoint{EndpointValue: config.EndpointValue{Value: config.NullEndpoint{}}},
		Egress:  config.EgressEndpoint{EndpointValue: config.EndpointValue{Value: config.NullEndpoint{}}},
	}
}

func nullRelayWithMiddleware() config.Relay {
	r := nullRelay()
	r.Middlewares = []config.Middleware{{Value: config.NAT{}}}
	return r
}

func TestNewRelaysCreatesRelay(t *testing.T) {
	r, err := NewRelays([]config.Relay{nullRelay()}, &testLogger{}, defaultMockFactory())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r == nil {
		t.Fatal("expected non-nil relay")
	}
	if err := r.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
}

func TestNewRelaysMultipleRelays(t *testing.T) {
	r, err := NewRelays([]config.Relay{nullRelay(), nullRelay()}, &testLogger{}, defaultMockFactory())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(r.items))
	}
	if err := r.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
}

func TestNewRelaysWithMiddlewares(t *testing.T) {
	r, err := NewRelays([]config.Relay{nullRelayWithMiddleware()}, &testLogger{}, defaultMockFactory())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.items[0].middlewares) != 1 {
		t.Fatalf("expected 1 middleware, got %d", len(r.items[0].middlewares))
	}
	if err := r.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
}

func TestNewRelaysCreateError(t *testing.T) {
	tests := []struct {
		name    string
		factory mockFactory
		cfg     []config.Relay
	}{
		{
			name: "ingress error",
			factory: func() mockFactory {
				f := defaultMockFactory()
				f.ingress = func(_ config.IngressEndpoint, _ Logger) (Endpoint, error) {
					return nil, errTest
				}
				return f
			}(),
			cfg: []config.Relay{nullRelay()},
		},
		{
			name: "egress error",
			factory: func() mockFactory {
				f := defaultMockFactory()
				f.egress = func(_ config.EgressEndpoint, _ Logger) (Endpoint, error) {
					return nil, errTest
				}
				return f
			}(),
			cfg: []config.Relay{nullRelay()},
		},
		{
			name: "middleware error",
			factory: func() mockFactory {
				f := defaultMockFactory()
				f.middleware = func(_ config.Middleware, _ Logger) (Middleware, error) {
					return nil, errTest
				}
				return f
			}(),
			cfg: []config.Relay{nullRelayWithMiddleware()},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewRelays(tt.cfg, &testLogger{}, tt.factory)
			if !errors.Is(err, errTest) {
				t.Fatalf("expected error wrapping %v, got %v", errTest, err)
			}
		})
	}
}

func TestNewRelaysCleanupOnError(t *testing.T) {
	ep := newMockEndpoint("mock-ingress")
	f := defaultMockFactory()
	f.ingress = func(_ config.IngressEndpoint, _ Logger) (Endpoint, error) {
		return ep, nil
	}
	f.egress = func(_ config.EgressEndpoint, _ Logger) (Endpoint, error) {
		return nil, errTest
	}

	_, err := NewRelays([]config.Relay{nullRelay()}, &testLogger{}, f)
	if !errors.Is(err, errTest) {
		t.Fatalf("expected error wrapping %v, got %v", errTest, err)
	}
	if !ep.closed.Load() {
		t.Fatal("expected ingress to be closed on egress error")
	}
}