package udpep

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"

	"tunrelay/internal/config"
)

func newTestIngress(t *testing.T, peers []config.Peer) *Ingress {
	t.Helper()
	i, err := NewIngress(config.UDPIngress{
		Listen: "127.0.0.1:0",
		Peers:  peers,
	}, testLog{})
	if err != nil {
		t.Fatalf("new ingress: %v", err)
	}
	t.Cleanup(func() { i.Close() })
	return i
}

func dialIngress(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.Dial("udp", addr)
	if err != nil {
		t.Fatalf("dial ingress: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func sendPacked(t *testing.T, conn net.Conn, payload []byte, pass string) {
	t.Helper()
	bb, err := pack(payload, pass)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	_, err = bb.WriteTo(conn)
	if err != nil {
		t.Fatalf("write to conn: %v", err)
	}
}

func packWithTimestamp(payload []byte, pass string, timestamp uint32) (net.Buffers, error) {
	header := make([]byte, HeaderSize)
	hash, err := calcHash(pass, payload, timestamp)
	if err != nil {
		return nil, err
	}
	binary.BigEndian.PutUint32(header, timestamp)
	copy(header[4:], hash[:])
	return net.Buffers{header, payload}, nil
}

func sendPackedWithTimestamp(t *testing.T, conn net.Conn, payload []byte, pass string, timestamp uint32) {
	t.Helper()
	bb, err := packWithTimestamp(payload, pass, timestamp)
	if err != nil {
		t.Fatalf("packWithTimestamp: %v", err)
	}
	_, err = bb.WriteTo(conn)
	if err != nil {
		t.Fatalf("write to conn: %v", err)
	}
}

func TestIngressReadHappyPath(t *testing.T) {
	i := newTestIngress(t, []config.Peer{
		{SAddr: "10.0.0.1", Password: "secret"},
	})

	payload := makeIPv4Packet([4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2})
	conn := dialIngress(t, i.conn.LocalAddr().String())
	sendPacked(t, conn, payload, "secret")
	conn.Close()

	buf := make([]byte, 1500)
	ctx, n, err := i.Read(context.Background(), buf, 0)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if n != len(payload) {
		t.Fatalf("expected %d bytes, got %d", len(payload), n)
	}
	if !bytes.Equal(buf[:n], payload) {
		t.Fatalf("payload mismatch")
	}
	if RemoteAddr(ctx) == nil {
		t.Fatal("expected remote addr in context")
	}
}

func TestIngressReadWrongPassword(t *testing.T) {
	i := newTestIngress(t, []config.Peer{
		{SAddr: "10.0.0.1", Password: "secret"},
	})

	payload := makeIPv4Packet([4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2})
	conn := dialIngress(t, i.conn.LocalAddr().String())
	sendPacked(t, conn, payload, "wrong")
	conn.Close()

	buf := make([]byte, 1500)
	_, _, err := i.Read(context.Background(), buf, 0)
	if !errors.Is(err, ErrWrongPass) {
		t.Fatalf("expected ErrWrongPass, got: %v", err)
	}
}

func TestIngressReadStaleTimestamp(t *testing.T) {
	i := newTestIngress(t, []config.Peer{
		{SAddr: "10.0.0.1", Password: "secret"},
	})

	payload := makeIPv4Packet([4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2})
	staleTime := uint32(time.Now().Unix()) - 5
	conn := dialIngress(t, i.conn.LocalAddr().String())
	sendPackedWithTimestamp(t, conn, payload, "secret", staleTime)
	conn.Close()

	buf := make([]byte, 1500)
	_, _, err := i.Read(context.Background(), buf, 0)
	if !errors.Is(err, ErrStalePacket) {
		t.Fatalf("expected ErrStalePacket, got: %v", err)
	}
}

func TestIngressReadUnknownSrc(t *testing.T) {
	i := newTestIngress(t, []config.Peer{
		{SAddr: "10.0.0.1", Password: "secret"},
	})

	payload := makeIPv4Packet([4]byte{10, 0, 0, 99}, [4]byte{10, 0, 0, 2})
	conn := dialIngress(t, i.conn.LocalAddr().String())
	sendPacked(t, conn, payload, "secret")
	conn.Close()

	buf := make([]byte, 1500)
	_, _, err := i.Read(context.Background(), buf, 0)
	if !errors.Is(err, ErrNotAllowSrc) {
		t.Fatalf("expected ErrNotAllowSrc, got: %v", err)
	}
}

func TestIngressReadTooSmall(t *testing.T) {
	i := newTestIngress(t, []config.Peer{
		{SAddr: "10.0.0.1", Password: "secret"},
	})

	conn := dialIngress(t, i.conn.LocalAddr().String())
	_, err := conn.Write([]byte{1, 2, 3})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	conn.Close()

	buf := make([]byte, 1500)
	_, _, err = i.Read(context.Background(), buf, 0)
	if !errors.Is(err, ErrSmallPacket) {
		t.Fatalf("expected ErrSmallPacket, got: %v", err)
	}
}

func TestIngressWriteHappyPath(t *testing.T) {
	i := newTestIngress(t, []config.Peer{
		{SAddr: "10.0.0.1", Password: "secret"},
	})

	payload := makeIPv4Packet([4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2})
	conn := dialIngress(t, i.conn.LocalAddr().String())
	sendPacked(t, conn, payload, "secret")

	buf := make([]byte, 1500)
	_, _, err := i.Read(context.Background(), buf, 0)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	outPacket := makeIPv4Packet([4]byte{10, 0, 0, 2}, [4]byte{10, 0, 0, 1})
	_, nw, err := i.Write(context.Background(), outPacket, 0)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if nw != len(outPacket) {
		t.Fatalf("write: expected %d, got %d", len(outPacket), nw)
	}

	conn.SetReadDeadline(time.Now().Add(time.Second))
	nr, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("client read: %v", err)
	}
	if nr != len(outPacket) {
		t.Fatalf("client: expected %d, got %d", len(outPacket), nr)
	}
}

func TestIngressWriteUnknownDst(t *testing.T) {
	i := newTestIngress(t, []config.Peer{
		{SAddr: "10.0.0.1", Password: "secret"},
	})

	packet := makeIPv4Packet([4]byte{10, 0, 0, 2}, [4]byte{10, 0, 0, 99})
	_, _, err := i.Write(context.Background(), packet, 0)
	if !errors.Is(err, ErrNoPeer) {
		t.Fatalf("expected ErrNoPeer, got: %v", err)
	}
}

func TestIngressWriteNoRaddr(t *testing.T) {
	i := newTestIngress(t, []config.Peer{
		{SAddr: "10.0.0.1", Password: "secret"},
	})

	packet := makeIPv4Packet([4]byte{10, 0, 0, 2}, [4]byte{10, 0, 0, 1})
	_, _, err := i.Write(context.Background(), packet, 0)
	if !errors.Is(err, ErrNoPeer) {
		t.Fatalf("expected ErrNoPeer, got: %v", err)
	}
}

func TestIngressWriteTooSmall(t *testing.T) {
	i := newTestIngress(t, []config.Peer{
		{SAddr: "10.0.0.1", Password: "secret"},
	})

	buf := make([]byte, 19)
	_, _, err := i.Write(context.Background(), buf, 0)
	if !errors.Is(err, ErrSmallPacket) {
		t.Fatalf("expected ErrSmallPacket, got: %v", err)
	}
}

func TestIngressCloseStopsRead(t *testing.T) {
	i := newTestIngress(t, []config.Peer{
		{SAddr: "10.0.0.1", Password: "secret"},
	})

	errCh := make(chan error, 1)
	go func() {
		buf := make([]byte, 1500)
		_, _, err := i.Read(context.Background(), buf, 0)
		errCh <- err
	}()

	time.Sleep(10 * time.Millisecond)
	i.Close()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error after close, got nil")
		}
	case <-time.After(time.Second):
		t.Fatal("Read did not return after Close")
	}
}
