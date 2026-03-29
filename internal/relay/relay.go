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
)

const (
	MaxPacketSize = 2048
)

type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

type Relay struct {
	closers []Endpoint
	log     Logger
}

func NewRelays(cfg []config.Relay, log Logger) (*Relay, error) {
	var ie []Endpoint
	defer func() {
		for _, closer := range ie {
			err := closer.Close()
			if err != nil {
				log.Error("cleanup failed new relay: %w", err)
			}
		}
	}()

	for i := len(cfg) - 1; i >= 0; i-- {
		relayCfg := cfg[i]
		ingress, err := createIngress(relayCfg.Ingress, log)
		if err != nil {
			return nil, fmt.Errorf("create ingress: %w", err)
		}
		ie = append(ie, ingress)
		egress, err := createEgress(relayCfg.Egress, log)
		if err != nil {
			return nil, fmt.Errorf("create egress: %w", err)
		}
		ie = append(ie, egress)
	}

	r := &Relay{ie, log}
	for i := 0; i < len(ie); i += 2 {
		r.relay(ie[i], ie[i+1])
	}
	ie = nil

	return r, nil
}

func (r *Relay) Close() error {
	var err error
	for _, closer := range r.closers {
		err1 := closer.Close()
		if err1 != nil {
			err = errors.Join(err, err1)
		}
	}
	return err
}

func (r *Relay) relay(ingress, egress Endpoint) {
	go r.pipe(ingress, egress)
	go r.pipe(egress, ingress)
}

func (r *Relay) pipe(a Endpoint, b Endpoint) {
	buf := make([]byte, MaxPacketSize)

	for {
		n, err := a.Read(buf)
		if err != nil {
			if errors.Is(err, os.ErrClosed) || errors.Is(err, net.ErrClosed) {
				return
			}
			r.log.Error(fmt.Sprintf("%v read: %v", a.Name(), err))
			continue
		}

		packet := buf[:n]

		_, err = b.Write(packet)
		if err != nil {
			if errors.Is(err, os.ErrClosed) || errors.Is(err, net.ErrClosed) {
				return
			}
			r.log.Error(fmt.Sprintf("%v write: %v", b.Name(), err))
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
