package relay

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"

	"tunrelay/internal/config"
	"tunrelay/internal/endpoint/nullep"
	"tunrelay/internal/endpoint/tunep"
	"tunrelay/internal/endpoint/udpep"
	"tunrelay/internal/middlewares/staticnat"
)

const (
	MaxPacketSize = 2048
)

type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

type Relay struct {
	items []item
	log   Logger
}

type item struct {
	ingress     Endpoint
	egress      Endpoint
	middlewares []Middleware
}

func NewRelays(cfg []config.Relay, log Logger) (*Relay, error) {
	relay := Relay{items: make([]item, len(cfg)), log: log}
	closers := relay.items
	defer func() {
		for _, closer := range closers {
			err := closer.Close()
			if err != nil {
				log.Error("cleanup failed new relay: %w", err)
			}
		}
	}()

	for i := len(cfg) - 1; i >= 0; i-- {
		relayCfg := cfg[i]
		var err error
		ingress, err := createIngress(relayCfg.Ingress, log)
		if err != nil {
			return nil, fmt.Errorf("create ingress: %w", err)
		}
		relay.items[i].ingress = ingress

		egress, err := createEgress(relayCfg.Egress, log)
		if err != nil {
			return nil, fmt.Errorf("create egress: %w", err)
		}
		relay.items[i].egress = egress

		for _, middlewareCfg := range relayCfg.Middlewares {
			mw, err := createMiddleware(middlewareCfg, log)
			if err != nil {
				return nil, fmt.Errorf("create middleware: %w", err)
			}
			relay.items[i].middlewares = append(relay.items[i].middlewares, mw)
		}
	}

	for _, item := range relay.items {
		item.relay(log)
	}
	closers = nil

	return &relay, nil
}

func (r *Relay) Close() error {
	var errs []error
	for _, item := range r.items {
		err := item.Close()
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (i *item) Close() error {
	var err1, err2 error
	if i.ingress != nil {
		err2 = i.ingress.Close()
	}
	if i.egress != nil {
		err1 = i.egress.Close()
	}
	return errors.Join(err1, err2)
}

func (i *item) relay(log Logger) {
	var forward, backward []pipeMiddleware
	for _, mw := range i.middlewares {
		forward = append(forward, pipeMiddleware{mw.Forward, mw.Name})
		backward = append(backward, pipeMiddleware{mw.Backward, mw.Name})
	}
	go pipe(i.ingress, i.egress, forward, log)
	go pipe(i.egress, i.ingress, backward, log)
}

type pipeMiddleware struct {
	Process func(_ []byte) error
	Name    func() string
}

func pipe(a, b Endpoint, middlewares []pipeMiddleware, log Logger) {
	buf := make([]byte, MaxPacketSize)

	for {
		n, err := a.Read(buf)
		if err != nil {
			if errors.Is(err, os.ErrClosed) || errors.Is(err, net.ErrClosed) {
				return
			}
			log.Error(fmt.Sprintf("%v read: %v", a.Name(), err))
			continue
		}

		packet := buf[:n]

		for _, mw := range middlewares {
			err = mw.Process(packet)
			if err != nil {
				log.Error(fmt.Sprintf("%v process: %v", mw.Name(), err))
				continue
			}
		}

		_, err = b.Write(packet)
		if err != nil {
			if errors.Is(err, os.ErrClosed) || errors.Is(err, net.ErrClosed) {
				return
			}
			log.Error(fmt.Sprintf("%v write: %v", b.Name(), err))
			continue
		}
	}
}

type Endpoint interface {
	io.ReadWriteCloser
	Name() string
}

func createIngress(cfg config.IngressEndpoint, log Logger) (Endpoint, error) {
	switch ep := cfg.Value.(type) {
	case config.TunIngress:
		return tunep.NewIngress(ep, log)
	case config.UDPIngress:
		return udpep.NewIngress(ep, log)
	case config.NullEndpoint:
		return nullep.NewEndpoint("null ingress", log), nil
	default:
		return nil, fmt.Errorf("unknown ingress type")
	}
}

func createEgress(cfg config.EgressEndpoint, log Logger) (Endpoint, error) {
	switch ep := cfg.Value.(type) {
	case config.TunEgress:
		return tunep.NewEgress(ep, log)
	case config.UDPEgress:
		return udpep.NewEgress(ep, log)
	case config.NullEndpoint:
		return nullep.NewEndpoint("null egress", log), nil
	default:
		return nil, fmt.Errorf("unknown egress type")
	}
}

type Middleware interface {
	Forward(packet []byte) error
	Backward(packet []byte) error
	Name() string
}

func createMiddleware(cfg config.Middleware, log Logger) (Middleware, error) {
	switch m := cfg.Value.(type) {
	case config.StaticNAT:
		return staticnat.NewStaticNAT(m, log)
	default:
		return nil, fmt.Errorf("unknown middleware type")
	}
}
