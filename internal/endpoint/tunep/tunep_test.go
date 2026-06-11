package tunep

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"tunrelay/internal/config"
)

const (
	bufferOffset = 16
	bufferSize   = 2048
)

type testLog struct{}

func (testLog) Info(msg string, args ...any) {
	fmt.Print(msg)
	for i, a := range args {
		if i%2 == 0 {
			fmt.Print(" ", a, "=")
		} else {
			fmt.Print(a)
		}
	}
	fmt.Println()
}

func createTestTun(t *testing.T) *tunDevice {
	t.Helper()
	if !isAdmin() {
		t.Skip("requires elevated privileges")
	}

	name := fmt.Sprintf("tun%d", time.Now().UnixMilli()%10000)
	cfg := config.TunEndpoint{
		Name: name,
		CIDR: "10.99.0.2/24",
		Peer: "10.99.0.1",
	}

	d, err := createTun(cfg, testLog{})
	if err != nil {
		t.Fatalf("createTun: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestCreateClose(t *testing.T) {
	d := createTestTun(t)
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestReadAfterClose(t *testing.T) {
	d := createTestTun(t)
	d.Close()

	_, _, err := d.Read(context.Background(), make([]byte, bufferSize), bufferOffset)
	if !errors.Is(err, os.ErrClosed) {
		t.Fatalf("expected os.ErrClosed, got: %v", err)
	}
}

func TestCloseUnblocksRead(t *testing.T) {
	d := createTestTun(t)

	readReady := make(chan struct{})
	errCh := make(chan error, 1)
	go func() {
		close(readReady)
		for {
			_, _, err := d.Read(context.Background(), make([]byte, bufferSize), bufferOffset)
			if err != nil {
				errCh <- err
				return
			}
		}
	}()

	<-readReady
	runtime.Gosched()
	d.Close()

	select {
	case err := <-errCh:
		if !errors.Is(err, os.ErrClosed) {
			t.Fatalf("expected os.ErrClosed, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Read did not unblock after Close")
	}
}

func TestWriteIPv4(t *testing.T) {
	d := createTestTun(t)
	buf := make([]byte, bufferSize)

	ipHeader := buf[bufferOffset:]
	ipHeader[0] = 0x45
	ipHeader[1] = 0x00
	ipHeader[2] = 0x00
	ipHeader[3] = 21
	ipHeader[4] = 0x00
	ipHeader[5] = 0x00
	ipHeader[6] = 0x40
	ipHeader[7] = 0x00
	ipHeader[8] = 64
	ipHeader[9] = 6
	copy(ipHeader[12:16], []byte{10, 99, 0, 1})
	copy(ipHeader[16:20], []byte{10, 99, 0, 2})
	ipHeader[20] = 0x00

	_, n, err := d.Write(context.Background(), buf[:bufferOffset+21], bufferOffset)
	if err != nil {
		t.Fatalf("Write IPv4: %v", err)
	}
	if n != 21 {
		t.Fatalf("Write IPv4: expected n=21, got n=%d", n)
	}
}

func TestWriteIPv6(t *testing.T) {
	d := createTestTun(t)
	buf := make([]byte, bufferSize)

	ipHeader := buf[bufferOffset:]
	ipHeader[0] = 0x60
	ipHeader[1] = 0x00
	ipHeader[2] = 0x00
	ipHeader[3] = 0x00
	ipHeader[4] = 0x00
	ipHeader[5] = 1
	ipHeader[6] = 0x3a
	ipHeader[7] = 0xff
	copy(ipHeader[8:24], []byte{0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1})
	copy(ipHeader[24:40], []byte{0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2})
	ipHeader[40] = 0x80

	_, n, err := d.Write(context.Background(), buf[:bufferOffset+41], bufferOffset)
	if err != nil {
		t.Fatalf("Write IPv6: %v", err)
	}
	if n != 41 {
		t.Fatalf("Write IPv6: expected n=41, got n=%d", n)
	}
}

func TestWriteInvalidVersion(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("version check is darwin-only")
	}
	d := createTestTun(t)
	buf := make([]byte, bufferSize)

	buf[bufferOffset] = 0x00

	_, _, err := d.Write(context.Background(), buf[:bufferOffset+1], bufferOffset)
	if err == nil {
		t.Fatal("expected error for invalid IP version, got nil")
	}
}
