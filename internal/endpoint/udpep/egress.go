package udpep

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"tunrelay/internal/config"
)

type Egress struct {
	mu   sync.Mutex
	conn *net.UDPConn
	dial string
	pass string
	log  Logger
}

func NewEgress(cfg config.UDPEgress, log Logger) (*Egress, error) {
	log.Info("udp client", "remote", cfg.Dial)

	return &Egress{
		dial: cfg.Dial,
		pass: cfg.Password,
		log:  log,
	}, nil
}

func (e *Egress) Read(ctx context.Context, b []byte) (context.Context, int, error) {
	conn, err := e.connect()
	if err != nil {
		return ctx, 0, fmt.Errorf("connect: %w", err)
	}

	n, err := conn.Read(b)
	return ctx, n, err
}

func (e *Egress) Write(ctx context.Context, b []byte) (context.Context, int, error) {
	conn, err := e.connect()
	if err != nil {
		return ctx, 0, fmt.Errorf("connect: %w", err)
	}

	bb, err := pack(b, e.pass)
	if err != nil {
		return ctx, 0, fmt.Errorf("pack: %w", err)
	}

	conn.SetDeadline(time.Now().Add(UDPTimeout))
	defer conn.SetDeadline(time.Time{})

	n, err := bb.WriteTo(conn)
	return ctx, int(n), err
}

func (e *Egress) Close() error {
	e.mu.Lock()
	conn := e.conn
	e.mu.Unlock()
	if conn == nil {
		return nil
	}
	return conn.Close()
}

func (e *Egress) connect() (*net.UDPConn, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.conn != nil {
		return e.conn, nil
	}

	addr, err := net.ResolveUDPAddr("udp", e.dial)
	if err != nil {
		return nil, fmt.Errorf("resolve addr %v: %w", e.dial, err)
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}

	e.conn = conn
	return e.conn, nil
}

func (_ *Egress) Name() string {
	return "udp egress"
}
