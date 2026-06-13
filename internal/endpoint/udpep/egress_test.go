package udpep

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"tunrelay/internal/config"
)

type testLog struct{}

func (testLog) Info(string, ...any) {}
func (testLog) Warn(string, ...any) {}

func newTestEgress(t *testing.T) (*Egress, *net.UDPConn) {
	t.Helper()
	server, err := net.ListenUDP("udp", nil)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { server.Close() })

	e, err := NewEgress(config.UDPEgress{Dial: server.LocalAddr().String()}, testLog{})
	if err != nil {
		t.Fatalf("new egress: %v", err)
	}
	t.Cleanup(func() { e.Close() })
	return e, server
}

func makeIPv4Packet(src, dst [4]byte) []byte {
	p := make([]byte, 20)
	p[0] = 0x45
	copy(p[12:16], src[:])
	copy(p[16:20], dst[:])
	return p
}

func TestWriteReadRoundTrip(t *testing.T) {
	server, err := net.ListenUDP("udp", nil)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { server.Close() })

	e, err := NewEgress(config.UDPEgress{Dial: server.LocalAddr().String()}, testLog{})
	if err != nil {
		t.Fatalf("new egress: %v", err)
	}
	t.Cleanup(func() { e.Close() })

	packet := makeIPv4Packet([4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2})
	buf := make([]byte, 1500)

	sendBack := []byte("hello from server")

	serverReadDone := make(chan struct{})
	go func() {
		defer close(serverReadDone)
		n, addr, err := server.ReadFrom(buf)
		if err != nil {
			t.Errorf("server read: %v", err)
			return
		}
		if n <= HeaderSize {
			t.Errorf("server got too small packet: %d", n)
			return
		}
		server.WriteToUDP(sendBack, addr.(*net.UDPAddr))
	}()

	_, nw, err := e.Write(context.Background(), packet, 0)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if nw != HeaderSize+len(packet) {
		t.Fatalf("write: expected %d bytes, got %d", HeaderSize+len(packet), nw)
	}

	<-serverReadDone

	_, nr, err := e.Read(context.Background(), buf, 0)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if nr != len(sendBack) {
		t.Fatalf("read: expected %d bytes, got %d", len(sendBack), nr)
	}
	if string(buf[:nr]) != string(sendBack) {
		t.Fatalf("read: expected %q, got %q", sendBack, buf[:nr])
	}
}

func TestReadReturnsErrorOnClose(t *testing.T) {
	e, _ := newTestEgress(t)

	_, err := e.connect()
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	readDone := make(chan error, 1)
	buf := make([]byte, 1500)
	go func() {
		_, _, err := e.Read(context.Background(), buf, 0)
		readDone <- err
	}()

	time.Sleep(100 * time.Millisecond)
	e.Close()

	select {
	case err := <-readDone:
		if !errors.Is(err, net.ErrClosed) {
			t.Fatalf("expected net.ErrClosed, got: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Read should return after Close, but it didn't")
	}
}

func TestReadDoesNotReturnOnReconnect(t *testing.T) {
	e, _ := newTestEgress(t)

	_, err := e.connect()
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	readDone := make(chan error, 1)
	buf := make([]byte, 1500)
	go func() {
		_, _, err := e.Read(context.Background(), buf, 0)
		readDone <- err
	}()

	time.Sleep(100 * time.Millisecond)
	e.close(true)

	select {
	case err := <-readDone:
		t.Fatalf("Read should not return on reconnect, got: %v", err)
	case <-time.After(time.Second):
	}
}
