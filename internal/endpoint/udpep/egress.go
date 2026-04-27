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
	mu        sync.Mutex
	conn      *net.UDPConn
	dial      string
	pass      string
	allowSrc  netip.Prefix
	allowIPv6 bool
	log       Logger
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
	log.Info("udp client", "remote", cfg.Dial, "allow_src", logAllowSrc, "allow_ipv6", cfg.AllowIPv6)

	return &Egress{
		dial:      cfg.Dial,
		pass:      cfg.Password,
		allowSrc:  allowSrc,
		allowIPv6: cfg.AllowIPv6,
		log:       log,
	}, nil
}

func (e *Egress) Read(ctx context.Context, b []byte, off int) (context.Context, int, error) {
	conn, err := e.connect()
	if err != nil {
		return ctx, 0, fmt.Errorf("connect: %w", err)
	}

	n, err := conn.Read(b[off:])
	return ctx, n, err
}

func (e *Egress) Write(ctx context.Context, b []byte, off int) (context.Context, int, error) {
	p := b[off:]
	conn, err := e.connect()
	if err != nil {
		return ctx, 0, fmt.Errorf("connect: %w", err)
	}

	if !iptool.CanGetVersion(p) {
		return ctx, 0, fmt.Errorf("can not get ip version")
	}
	version := iptool.Version(p)
	if version != 4 && version != 6 {
		return ctx, 0, fmt.Errorf("wrong ip version %v", version)
	}

	if e.allowIPv6 == false && version == 6 {
		return ctx, 0, fmt.Errorf("not ipv4, ver %v", version)
	}

	if version == 4 && e.allowSrc.IsValid() {
		if len(p) < iptool.IPHeaderMinSize {
			return ctx, 0, fmt.Errorf("can not get src")
		}
		proto := p[iptool.ProtocolOffset]
		src := netip.AddrFrom4(iptool.Src(p))
		dst := netip.AddrFrom4(iptool.Dst(p))
		if !e.allowSrc.Contains(src) {
			return ctx, 0, fmt.Errorf("proto: %v, src %s, dst: %s: %w", proto, src, dst, ErrNotAllowSrc)
		}
	}

	bb, err := pack(p, e.pass)
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
