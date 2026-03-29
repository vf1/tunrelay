package udpep

import (
	"fmt"
	"net"
	"time"

	"tunrelay/internal/config"
)

type Ingress struct {
	conn *net.UDPConn
	pass string
	log  Logger

	raddr net.Addr
}

func NewIngress(cfg config.UDPIngress, log Logger) (*Ingress, error) {
	addr, err := net.ResolveUDPAddr("udp", cfg.Listen)
	if err != nil {
		return nil, fmt.Errorf("resolve addr %v: %w", cfg.Listen, err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	log.Info("udp listener", "local", cfg.Listen)
	return &Ingress{conn: conn, pass: cfg.Password, log: log}, nil
}

func (i *Ingress) Read(b []byte) (int, error) {
	n, raddr, err := i.conn.ReadFrom(b)
	if err != nil {
		return 0, err
	}

	data, err := unpack(b[:n:n], i.pass)
	if err != nil {
		return 0, err
	}

	i.raddr = raddr
	copy(b, data)
	return len(data), nil
}

func (i *Ingress) Write(b []byte) (int, error) {
	if i.raddr == nil {
		return 0, ErrNoPeer
	}

	i.conn.SetDeadline(time.Now().Add(UDPTimeout))
	defer i.conn.SetDeadline(time.Time{})

	return i.conn.WriteTo(b, i.raddr)
}

func (i *Ingress) Close() error {
	return i.conn.Close()
}

func (_ *Ingress) Name() string {
	return "udp ingress"
}
