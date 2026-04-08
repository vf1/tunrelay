package udpep

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	"tunrelay/internal/config"
	"tunrelay/internal/iptool"
)

type Egress struct {
	mu       sync.Mutex
	conn     *net.UDPConn
	dial     string
	pass     string
	allowSrc netip.Prefix
	log      Logger
}

func NewEgress(cfg config.UDPEgress, log Logger) (*Egress, error) {
	var allowSrc netip.Prefix
	if cfg.AllowSrc != "" {
		var err error
		allowSrc, err = netip.ParsePrefix(cfg.AllowSrc)
		if err != nil {
			return nil, fmt.Errorf("parse allow src: %w", err)
		}
	}

	logAllowSrc := allowSrc.String()
	if !allowSrc.IsValid() {
		logAllowSrc = "any"
	}
	log.Info("udp client", "remote", cfg.Dial, "allow_src", logAllowSrc)

	return &Egress{
		dial:     cfg.Dial,
		pass:     cfg.Password,
		allowSrc: allowSrc,
		log:      log,
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

	if e.allowSrc.IsValid() {
		if len(b) < iptool.IPHeaderMinSize {
			return ctx, 0, fmt.Errorf("can not get src")
		}
		src := netip.AddrFrom4(iptool.Src(b))
		dst := netip.AddrFrom4(iptool.Dst(b))
		if !e.allowSrc.Contains(src) {
			return ctx, 0, fmt.Errorf("src %s, dst: %s: %w", src, dst, ErrNotAllowSrc)
		}
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
