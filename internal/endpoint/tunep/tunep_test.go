package tunep

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"tunrelay/internal/config"
)

const (
	bufferOffset = 16
	bufferSize   = 2048

	testLocal = "10.99.0.2"
	testPeer  = "10.99.0.1"
	testCIDR  = testLocal + "/24"
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
		CIDR: testCIDR,
		Peer: testPeer,
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
	copy(ipHeader[12:16], net.ParseIP(testPeer).To4())
	copy(ipHeader[16:20], net.ParseIP(testLocal).To4())
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

func pingCmd(host string) *exec.Cmd {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("ping", "-n", "1", "-w", "1000", host)
	case "darwin":
		return exec.Command("ping", "-c", "1", "-W", "1000", host)
	default:
		return exec.Command("ping", "-c", "1", "-W", "1", host)
	}
}

func TestPingRead(t *testing.T) {
	d := createTestTun(t)

	cmd := pingCmd(testPeer)
	var pingOut bytes.Buffer
	cmd.Stdout = &pingOut
	cmd.Stderr = &pingOut
	if err := cmd.Start(); err != nil {
		t.Fatalf("ping start: %v", err)
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})

	buf := make([]byte, bufferSize)
	deadline := time.After(5 * time.Second)

	for {
		type readResult struct {
			n   int
			err error
		}
		ch := make(chan readResult, 1)
		go func() {
			_, n, err := d.Read(context.Background(), buf, bufferOffset)
			ch <- readResult{n, err}
		}()

		var n int
		select {
		case r := <-ch:
			if r.err != nil {
				t.Fatalf("Read: %v", r.err)
			}
			n = r.n
		case <-deadline:
			d.Close()
			<-ch
			t.Fatalf("timed out waiting for ICMP packet\nping output: %s", pingOut.String())
		}

		pkt := buf[bufferOffset : bufferOffset+n]

		ver := pkt[0] >> 4
		if ver != 4 {
			continue
		}

		proto := pkt[9]
		if proto != 1 {
			continue
		}

		dst := net.IP(pkt[16:20])
		if !dst.Equal(net.ParseIP(testPeer)) {
			continue
		}

		totalLen := int(pkt[2])<<8 | int(pkt[3])
		if totalLen != n {
			t.Fatalf("IP totalLen %d != n %d", totalLen, n)
		}
		return
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
