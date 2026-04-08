package udpep

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sync/atomic"
	"time"

	"tunrelay/internal/config"
	"tunrelay/internal/iptool"
)

type Ingress struct {
	conn  *net.UDPConn
	peers map[[4]byte]*peer
	log   Logger
}

type peer struct {
	pass  string
	raddr atomic.Pointer[net.Addr]
}

func NewIngress(cfg config.UDPIngress, log Logger) (*Ingress, error) {
	peers := make(map[[4]byte]*peer, len(cfg.Peers))
	for _, peerCfg := range cfg.Peers {
		addr, err := netip.ParseAddr(peerCfg.SAddr)
		if err != nil {
			return nil, fmt.Errorf("parse peer addr (%v): %w", peerCfg.SAddr, err)
		}
		if !addr.Is4() {
			return nil, fmt.Errorf("peer addr is not IPv4: %v", addr)
		}
		peers[addr.As4()] = &peer{pass: peerCfg.Password}
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

	data, src, err := unpack(b[:n:n], i.lookupPassword)
	if err != nil {
		return ctx, 0, err
	}

	i.updateRaddr(src, raddr)
	ctx = withRemoteAddr(ctx, raddr)

	copy(b, data)

	return ctx, len(data), nil
}

func (i *Ingress) Write(ctx context.Context, b []byte) (context.Context, int, error) {
	if len(b) < HeaderSize {
		return nil, 0, ErrSmallPacket
	}
	dst := iptool.Dst(b)
	raddr := i.lookupRaddr(dst)
	if raddr == nil {
		return ctx, 0, fmt.Errorf("dst %s: %w", netip.AddrFrom4(dst), ErrNoPeer)
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

func (i *Ingress) lookupPassword(src [4]byte) (string, bool) {
	peer, found := i.peers[src]
	if !found {
		return "", false
	}
	return peer.pass, true
}

func (i *Ingress) lookupRaddr(src [4]byte) net.Addr {
	peer, found := i.peers[src]
	if !found {
		return nil
	}
	raddr := peer.raddr.Load()
	if raddr == nil {
		return nil
	}
	return *raddr
}

func (i *Ingress) updateRaddr(src [4]byte, raddr net.Addr) {
	peer, found := i.peers[src]
	if !found {
		panic("unexpected src")
	}
	peer.raddr.Store(&raddr)
}

type remoteAddr struct{}

func RemoteAddr(ctx context.Context) net.Addr {
	val := ctx.Value(remoteAddr{})
	if val == nil {
		return nil
	}
	return val.(net.Addr)
}

func withRemoteAddr(ctx context.Context, addr net.Addr) context.Context {
	return context.WithValue(ctx, remoteAddr{}, addr)
}
