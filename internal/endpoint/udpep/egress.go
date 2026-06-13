package udpep

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"syscall"
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
	reconnect bool
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
	for {
		conn, err := e.connect()
		if err != nil {
			return ctx, 0, fmt.Errorf("connect: %w", err)
		}

		n, err := conn.Read(b[off:])
		if err != nil {
			e.mu.Lock()
			reconnect := e.reconnect
			if reconnect {
				e.reconnect = false
			}
			e.mu.Unlock()
			if reconnect {
				continue
			}
		}
		return ctx, n, err
	}
}

func (e *Egress) Write(ctx context.Context, b []byte, off int) (context.Context, int, error) {
	p := b[off:]

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

	var n int64
	var r retry
	for {
		conn, err := e.connect()
		if err != nil {
			return ctx, 0, fmt.Errorf("connect: %w", err)
		}

		conn.SetDeadline(time.Now().Add(UDPTimeout))
		n, err = bb.WriteTo(conn)
		if err == nil {
			conn.SetDeadline(time.Time{})
			break
		}
		if errno, ok := errors.AsType[syscall.Errno](err); ok {
			switch errno {
			case syscall.ENOBUFS:
				e.log.Warn("ENOBUFS", "retry", r.count+1, "sleep", r.delay)
				sleep(&r, errno, WriteRetryDelay)
				if r.count >= WriteRetries {
					return ctx, 0, fmt.Errorf("resend on ENOBUFS attempts exceed: %w", err)
				}
			case syscall.EADDRNOTAVAIL:
				e.log.Warn("close broken connection", "errno", errno, "retry", r.count+1, "sleep", r.delay)
				sleep(&r, errno, ReconnectRetryDelay)
				e.close(true)
				if r.count >= ReconnectRetries {
					return ctx, 0, fmt.Errorf("reconnect on EADDRNOTAVAIL attempts exceed: %w", err)
				}
			default:
				return ctx, 0, err
			}
		} else {
			return ctx, 0, err
		}
	}
	return ctx, int(n), nil
}

func (e *Egress) Close() error {
	return e.close(false)
}

func (e *Egress) close(reconnect bool) error {
	e.mu.Lock()
	conn := e.conn
	e.conn = nil
	e.reconnect = reconnect
	e.mu.Unlock()
	if conn == nil {
		return nil
	}
	return conn.Close()
}

func (_ *Egress) Name() string {
	return "udp egress"
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

type retry struct {
	errno syscall.Errno
	count int
	delay time.Duration
}

func sleep(r *retry, errno syscall.Errno, initDelay time.Duration) {
	if r.errno != errno {
		*r = retry{errno: errno}
	}
	if r.delay == 0 {
		r.delay = initDelay
	}
	time.Sleep(r.delay)
	r.count++
	r.delay *= 2
}
