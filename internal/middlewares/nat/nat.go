package nat

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"tunrelay/internal/config"
	"tunrelay/internal/endpoint/udpep"
	"tunrelay/internal/iptool"
)

type Logger interface {
	Info(msg string, args ...any)
}

type NAT struct {
	mu            sync.Mutex
	addrsByNewSrc map[[4]byte]item
	addrsByRemote map[string]item
	srcCurrent    uint32
	srcRangeStart uint32
	srcRangeEnd   uint32
}

type item struct {
	remoteAddr net.Addr
	newSrc     net.IP
	oldSrc     net.IP
	updatedAt  time.Time
}

func NewNAT(cfg config.NAT, log Logger) (*NAT, error) {
	startIP := net.ParseIP(cfg.SrcRangeStart)
	if startIP == nil {
		return nil, fmt.Errorf("parse src_range_start: %v", cfg.SrcRangeStart)
	}
	endIP := net.ParseIP(cfg.SrcRangeEnd)
	if endIP == nil {
		return nil, fmt.Errorf("parse src_range_end: %v", cfg.SrcRangeEnd)
	}

	log.Info(
		"nat",
		"range_start", startIP.String(),
		"range_end", endIP.String(),
	)

	return &NAT{
		addrsByNewSrc: map[[4]byte]item{},
		addrsByRemote: map[string]item{},
		srcCurrent:    binary.BigEndian.Uint32(startIP.To4()),
		srcRangeStart: binary.BigEndian.Uint32(startIP.To4()),
		srcRangeEnd:   binary.BigEndian.Uint32(endIP.To4()),
	}, nil
}

func (n *NAT) Forward(ctx context.Context, packet []byte) (context.Context, error) {
	remoteAddr := udpep.RemoteAddr(ctx)
	if remoteAddr == nil {
		return ctx, fmt.Errorf("no real src addr")
	}

	oldSrc := iptool.SrcIPv4(packet)

	n.mu.Lock()
	var newSrc net.IP
	rec, found := n.addrsByRemote[remoteAddr.String()]
	if found {
		newSrc = rec.newSrc
	} else {
		var err error
		newSrc, err = n.nextSrc()
		if err != nil {
			return ctx, err
		}
	}
	rec = item{remoteAddr, newSrc, oldSrc, time.Now()}
	n.addrsByNewSrc[[4]byte(newSrc.To4())] = rec
	n.addrsByRemote[remoteAddr.String()] = rec
	n.mu.Unlock()

	return ctx, iptool.ReplaceIPs(packet, newSrc, nil)
}

func (n *NAT) Backward(ctx context.Context, packet []byte) (context.Context, error) {
	dst := iptool.DstIPv4(packet)

	n.mu.Lock()
	item, found := n.addrsByNewSrc[[4]byte(dst.To4())]
	n.mu.Unlock()
	if !found {
		return ctx, fmt.Errorf("no addr in table: %v", dst)
	}

	ctx = udpep.WithRemoteAddr(ctx, item.remoteAddr)

	return ctx, iptool.ReplaceIPs(packet, nil, item.oldSrc)
}

func (_ *NAT) Name() string {
	return "nat"
}

func (n *NAT) nextSrc() (net.IP, error) {
	start := n.srcCurrent
	for {
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, n.srcCurrent)

		n.srcCurrent++
		if n.srcCurrent == n.srcRangeEnd {
			n.srcCurrent = n.srcRangeStart
		}

		if item, found := n.addrsByNewSrc[[4]byte(ip.To4())]; !found || time.Since(item.updatedAt) > time.Hour {
			return ip, nil
		}

		if n.srcCurrent == start {
			return net.IP{}, fmt.Errorf("no free src found")
		}
	}
}
