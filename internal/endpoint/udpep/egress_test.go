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
