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

	_, _, err := d.Read(context.Background(), make([]byte, 1500), 0)
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
		_, _, err := d.Read(context.Background(), make([]byte, 1500), 0)
		errCh <- err
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
