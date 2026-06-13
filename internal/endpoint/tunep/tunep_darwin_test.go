//go:build darwin

package tunep

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

func TestDarwinPrefixWrite(t *testing.T) {
	f := newTestFile(t)
	d := &tunDevice{f: f}

	packet := []byte{
		0x45, 0x00, 0x00, 0x14,
		0x00, 0x00, 0x40, 0x00,
		0x40, 0x01, 0x00, 0x00,
		192, 168, 1, 1,
		192, 168, 1, 2,
	}

	buf := make([]byte, bufferSize)
	copy(buf[bufferOffset:], packet)

	ctx, n, err := d.Write(context.Background(), buf[:bufferOffset+len(packet)], bufferOffset)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(packet) {
		t.Fatalf("Write returned n=%d, want %d", n, len(packet))
	}
	if ctx == nil {
		t.Fatal("Write returned nil context")
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Seek: %v", err)
	}

	raw := make([]byte, PrefixSize+len(packet))
	if _, err := io.ReadFull(f, raw); err != nil {
		t.Fatalf("Read raw file: %v", err)
	}

	if raw[0] != 0x00 || raw[1] != 0x00 || raw[2] != 0x00 {
		t.Fatalf("unexpected prefix bytes: %x", raw[:3])
	}
	if raw[3] != unix.AF_INET {
		t.Fatalf("expected AF_INET prefix, got %x", raw[3])
	}
	if !bytes.Equal(raw[PrefixSize:], packet) {
		t.Fatalf("packet mismatch: got %x, want %x", raw[PrefixSize:], packet)
	}
}

func TestDarwinPrefixRead(t *testing.T) {
	packet := []byte{
		0x45, 0x00, 0x00, 0x14,
		0x00, 0x00, 0x40, 0x00,
		0x40, 0x01, 0x00, 0x00,
		192, 168, 1, 1,
		192, 168, 1, 2,
	}
	raw := append([]byte{0x00, 0x00, 0x00, unix.AF_INET}, packet...)

	f := newTestFile(t)
	if _, err := f.Write(raw); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Seek: %v", err)
	}

	d := &tunDevice{f: f}
	buf := make([]byte, bufferSize)

	ctx, n, err := d.Read(context.Background(), buf, bufferOffset)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if n != len(packet) {
		t.Fatalf("Read returned n=%d, want %d", n, len(packet))
	}
	if ctx == nil {
		t.Fatal("Read returned nil context")
	}
	if !bytes.Equal(buf[bufferOffset:bufferOffset+n], packet) {
		t.Fatalf("packet mismatch: got %x, want %x", buf[bufferOffset:bufferOffset+n], packet)
	}
}

func TestDarwinPrefixIPv6Write(t *testing.T) {
	f := newTestFile(t)
	d := &tunDevice{f: f}

	packet := make([]byte, 40)
	packet[0] = 0x60 // IPv6 version/traffic class
	for i := 1; i < 40; i++ {
		packet[i] = byte(i)
	}

	buf := make([]byte, bufferSize)
	copy(buf[bufferOffset:], packet)

	_, n, err := d.Write(context.Background(), buf[:bufferOffset+len(packet)], bufferOffset)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(packet) {
		t.Fatalf("Write returned n=%d, want %d", n, len(packet))
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	raw := make([]byte, PrefixSize+len(packet))
	if _, err := io.ReadFull(f, raw); err != nil {
		t.Fatalf("Read raw file: %v", err)
	}
	if raw[3] != unix.AF_INET6 {
		t.Fatalf("expected AF_INET6 prefix, got %x", raw[3])
	}
	if !bytes.Equal(raw[PrefixSize:], packet) {
		t.Fatalf("packet mismatch: got %x, want %x", raw[PrefixSize:], packet)
	}
}

func TestDarwinWriteInvalidVersion(t *testing.T) {
	f := newTestFile(t)
	d := &tunDevice{f: f}

	buf := make([]byte, bufferSize)
	buf[bufferOffset] = 0x00

	_, _, err := d.Write(context.Background(), buf[:bufferOffset+1], bufferOffset)
	if err == nil {
		t.Fatal("expected error for invalid IP version, got nil")
	}

	info, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("expected no bytes written on error, got %d", info.Size())
	}
}

func newTestFile(t *testing.T) *os.File {
	t.Helper()
	f, err := os.CreateTemp("", "tunep-*")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	t.Cleanup(func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	})
	return f
}
