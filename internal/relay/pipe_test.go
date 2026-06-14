package relay

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// --- mockEndpoint ---

type mockReadResult struct {
	data []byte
	err  error
}

type mockEndpoint struct {
	name    string
	packets chan mockReadResult
	written chan []byte
	closing chan struct{}
	closeOnce sync.Once
	closed    atomic.Bool

	mu       sync.Mutex
	writeErr error
}

func newMockEndpoint(name string) *mockEndpoint {
	return &mockEndpoint{
		name:    name,
		packets: make(chan mockReadResult, 10),
		written: make(chan []byte, 10),
		closing: make(chan struct{}),
	}
}

func (m *mockEndpoint) Read(_ context.Context, p []byte, off int) (context.Context, int, error) {
	select {
	case <-m.closing:
		return context.Background(), 0, os.ErrClosed
	case res := <-m.packets:
		if res.err != nil {
			return context.Background(), 0, res.err
		}
		copy(p[off:], res.data)
		return context.Background(), len(res.data), nil
	}
}

func (m *mockEndpoint) Write(_ context.Context, p []byte, off int) (context.Context, int, error) {
	m.mu.Lock()
	err := m.writeErr
	m.writeErr = nil
	m.mu.Unlock()
	if err != nil {
		return context.Background(), 0, err
	}
	data := make([]byte, len(p)-off)
	copy(data, p[off:])
	m.written <- data
	return context.Background(), len(p)-off, nil
}

func (m *mockEndpoint) Close() error {
	m.closed.Store(true)
	m.closeOnce.Do(func() { close(m.closing) })
	return nil
}

func (m *mockEndpoint) Name() string { return m.name }

func (m *mockEndpoint) sendPacket(data []byte) {
	m.packets <- mockReadResult{data: data}
}

func (m *mockEndpoint) sendError(err error) {
	m.packets <- mockReadResult{err: err}
}

func (m *mockEndpoint) setWriteErr(err error) {
	m.mu.Lock()
	m.writeErr = err
	m.mu.Unlock()
}

// --- mockMiddleware ---

type mockMiddleware struct {
	name     string
	forward  func(ctx context.Context, packet []byte) (context.Context, error)
	backward func(ctx context.Context, packet []byte) (context.Context, error)
}

func (m *mockMiddleware) Forward(ctx context.Context, packet []byte) (context.Context, error) {
	return m.forward(ctx, packet)
}

func (m *mockMiddleware) Backward(ctx context.Context, packet []byte) (context.Context, error) {
	return m.backward(ctx, packet)
}

func (m *mockMiddleware) Name() string { return m.name }

// --- testLogger ---

type testLogger struct {
	mu     sync.Mutex
	errors []string
}

func (l *testLogger) Info(string, ...any)  {}
func (l *testLogger) Warn(string, ...any)  {}
func (l *testLogger) Error(msg string, args ...any) {
	l.mu.Lock()
	l.errors = append(l.errors, fmt.Sprintf(msg, args...))
	l.mu.Unlock()
}

// --- helpers ---

func runPipe(a, b Endpoint, ms []pipeMiddleware, log Logger) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		pipe(a, b, ms, log)
		close(done)
	}()
	return done
}

func waitDone(t *testing.T, done <-chan struct{}, msg string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal(msg)
	}
}

func writtenOrTimeout(t *testing.T, ch <-chan []byte, timeout time.Duration) []byte {
	t.Helper()
	select {
	case data := <-ch:
		return data
	case <-time.After(timeout):
		t.Fatal("timeout waiting for written data")
		return nil
	}
}

// --- tests ---

func TestPipeReadOsErrClosed(t *testing.T) {
	a := newMockEndpoint("a")
	b := newMockEndpoint("b")
	log := &testLogger{}

	done := runPipe(a, b, nil, log)
	a.Close()
	waitDone(t, done, "pipe did not exit after Read returned os.ErrClosed")
}

func TestPipeReadNetErrClosed(t *testing.T) {
	a := newMockEndpoint("a")
	b := newMockEndpoint("b")
	log := &testLogger{}

	done := runPipe(a, b, nil, log)
	a.sendError(net.ErrClosed)
	waitDone(t, done, "pipe did not exit after Read returned net.ErrClosed")
}

func TestPipeReadErrorContinues(t *testing.T) {
	a := newMockEndpoint("a")
	b := newMockEndpoint("b")
	log := &testLogger{}

	done := runPipe(a, b, nil, log)

	a.sendError(io.ErrUnexpectedEOF)
	time.Sleep(50 * time.Millisecond)

	if len(log.errors) == 0 {
		t.Fatal("expected log error after Read error")
	}

	a.sendPacket([]byte("hello"))
	data := writtenOrTimeout(t, b.written, time.Second)
	if string(data) != "hello" {
		t.Fatalf("expected 'hello', got %q", data)
	}

	a.Close()
	waitDone(t, done, "pipe did not exit")
}

func TestPipeWriteOsErrClosed(t *testing.T) {
	a := newMockEndpoint("a")
	b := newMockEndpoint("b")
	log := &testLogger{}

	b.setWriteErr(os.ErrClosed)
	done := runPipe(a, b, nil, log)

	a.sendPacket([]byte("hello"))
	waitDone(t, done, "pipe did not exit after Write returned os.ErrClosed")
}

func TestPipeWriteNetErrClosed(t *testing.T) {
	a := newMockEndpoint("a")
	b := newMockEndpoint("b")
	log := &testLogger{}

	b.setWriteErr(net.ErrClosed)
	done := runPipe(a, b, nil, log)

	a.sendPacket([]byte("hello"))
	waitDone(t, done, "pipe did not exit after Write returned net.ErrClosed")
}

func TestPipeWriteErrorContinues(t *testing.T) {
	a := newMockEndpoint("a")
	b := newMockEndpoint("b")
	log := &testLogger{}

	b.setWriteErr(io.ErrUnexpectedEOF)
	done := runPipe(a, b, nil, log)

	a.sendPacket([]byte("first"))
	time.Sleep(50 * time.Millisecond)

	if len(log.errors) == 0 {
		t.Fatal("expected log error after Write error")
	}

	a.sendPacket([]byte("second"))
	data := writtenOrTimeout(t, b.written, time.Second)
	if string(data) != "second" {
		t.Fatalf("expected 'second', got %q", data)
	}

	a.Close()
	waitDone(t, done, "pipe did not exit")
}

func TestPipeHappyPath(t *testing.T) {
	a := newMockEndpoint("a")
	b := newMockEndpoint("b")
	log := &testLogger{}

	done := runPipe(a, b, nil, log)

	a.sendPacket([]byte("hello"))
	data := writtenOrTimeout(t, b.written, time.Second)
	if string(data) != "hello" {
		t.Fatalf("expected 'hello', got %q", data)
	}

	a.Close()
	waitDone(t, done, "pipe did not exit")
}

func TestPipeMiddlewareTransforms(t *testing.T) {
	a := newMockEndpoint("a")
	b := newMockEndpoint("b")
	log := &testLogger{}

	mw := pipeMiddleware{
		Process: func(_ context.Context, p []byte) (context.Context, error) {
			p[0] = 'X'
			return context.Background(), nil
		},
		Name: func() string { return "transform" },
	}
	mws := []pipeMiddleware{mw}

	done := runPipe(a, b, mws, log)

	a.sendPacket([]byte("hello"))
	data := writtenOrTimeout(t, b.written, time.Second)
	if string(data) != "Xello" {
		t.Fatalf("expected 'Xello', got %q", data)
	}

	a.Close()
	waitDone(t, done, "pipe did not exit")
}

func TestPipeMultipleMiddlewares(t *testing.T) {
	a := newMockEndpoint("a")
	b := newMockEndpoint("b")
	log := &testLogger{}

	mw1 := pipeMiddleware{
		Process: func(_ context.Context, p []byte) (context.Context, error) {
			p[0] = 'X'
			return context.Background(), nil
		},
		Name: func() string { return "mw1" },
	}
	mw2 := pipeMiddleware{
		Process: func(_ context.Context, p []byte) (context.Context, error) {
			p[1] = 'Y'
			return context.Background(), nil
		},
		Name: func() string { return "mw2" },
	}
	mws := []pipeMiddleware{mw1, mw2}

	done := runPipe(a, b, mws, log)

	a.sendPacket([]byte("hello"))
	data := writtenOrTimeout(t, b.written, time.Second)
	if string(data) != "XYllo" {
		t.Fatalf("expected 'XYllo', got %q", data)
	}

	a.Close()
	waitDone(t, done, "pipe did not exit")
}

func TestPipeMiddlewareErrorSkipsWrite(t *testing.T) {
	a := newMockEndpoint("a")
	b := newMockEndpoint("b")
	log := &testLogger{}

	var calls atomic.Int32
	mw := pipeMiddleware{
		Process: func(_ context.Context, p []byte) (context.Context, error) {
			if calls.Add(1) == 1 {
				return context.Background(), errors.New("mw error")
			}
			return context.Background(), nil
		},
		Name: func() string { return "toggle" },
	}
	mws := []pipeMiddleware{mw}

	done := runPipe(a, b, mws, log)

	a.sendPacket([]byte("first"))
	time.Sleep(50 * time.Millisecond)

	if len(log.errors) == 0 {
		t.Fatal("expected log error after middleware error")
	}

	a.sendPacket([]byte("second"))
	data := writtenOrTimeout(t, b.written, time.Second)
	if string(data) != "second" {
		t.Fatalf("expected 'second', got %q", data)
	}

	a.Close()
	waitDone(t, done, "pipe did not exit")
}

func TestPipeMultiplePackets(t *testing.T) {
	a := newMockEndpoint("a")
	b := newMockEndpoint("b")
	log := &testLogger{}

	done := runPipe(a, b, nil, log)

	packets := [][]byte{[]byte("one"), []byte("two"), []byte("three")}
	for _, p := range packets {
		a.sendPacket(p)
		data := writtenOrTimeout(t, b.written, time.Second)
		if string(data) != string(p) {
			t.Fatalf("expected %q, got %q", p, data)
		}
	}

	a.Close()
	waitDone(t, done, "pipe did not exit")
}
