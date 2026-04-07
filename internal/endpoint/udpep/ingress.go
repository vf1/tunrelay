package udpep

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"time"

	"tunrelay/internal/config"
)

type Ingress struct {
	conn  *net.UDPConn
	peers map[[4]byte]string
	log   Logger

	raddr net.Addr
}

func NewIngress(cfg config.UDPIngress, log Logger) (*Ingress, error) {
	peers := make(map[[4]byte]string, len(cfg.Peers))
	for _, peer := range cfg.Peers {
		addr, err := netip.ParseAddr(peer.SAddr)
		if err != nil {
			return nil, fmt.Errorf("parse peer addr (%v): %w", peer.SAddr, err)
		}
		if !addr.Is4() {
			return nil, fmt.Errorf("peer addr is not IPv4: %v", addr)
		}
		peers[addr.As4()] = peer.Password
	}

	addr, err := net.ResolveUDPAddr("udp", cfg.Listen)
	if err != nil {
		return nil, fmt.Errorf("resolve addr %v: %w", cfg.Listen, err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	log.Info("udp listener", "local", cfg.Listen)
	return &Ingress{conn: conn, peers: peers, log: log}, nil
}

func (i *Ingress) Read(ctx context.Context, b []byte) (context.Context, int, error) {
	n, raddr, err := i.conn.ReadFrom(b)
	if err != nil {
		return ctx, 0, err
	}

	data, err := unpack(b[:n:n], i.peers)
	if err != nil {
		return ctx, 0, err
	}

	i.raddr = raddr
	ctx = WithRemoteAddr(ctx, raddr)

	copy(b, data)

	return ctx, len(data), nil
}

func (i *Ingress) Write(ctx context.Context, b []byte) (context.Context, int, error) {
	raddr := RemoteAddr(ctx)
	if raddr == nil {
		raddr = i.raddr
	}
	if raddr == nil {
		return ctx, 0, ErrNoPeer
	}

	i.conn.SetDeadline(time.Now().Add(UDPTimeout))
	defer i.conn.SetDeadline(time.Time{})

	n, err := i.conn.WriteTo(b, raddr)
	return ctx, n, err
}

func (i *Ingress) Close() error {
	return i.conn.Close()
}

func (_ *Ingress) Name() string {
	return "udp ingress"
}

type remoteAddr struct{}

func RemoteAddr(ctx context.Context) net.Addr {
	val := ctx.Value(remoteAddr{})
	if val == nil {
		return nil
	}
	return val.(net.Addr)
}

func WithRemoteAddr(ctx context.Context, addr net.Addr) context.Context {
	return context.WithValue(ctx, remoteAddr{}, addr)
}
