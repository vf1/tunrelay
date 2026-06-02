package tunep

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"sync"
	"unsafe"

	"tunrelay/internal/config"
	"tunrelay/internal/tunctl"

	"golang.org/x/sys/windows"
)

const ringCapacity = 0x400000

type tunDevice struct {
	w         *Wintun
	adapter   Adapter
	session   Session
	closeOnce sync.Once
}

func (d *tunDevice) Read(ctx context.Context, p []byte, off int) (context.Context, int, error) {
	for {
		ptr, size, err := d.w.ReceivePacket(d.session)
		if err != nil {
			if errors.Is(err, os.ErrClosed) {
				return nil, 0, err
			}
			if errors.Is(err, windows.ERROR_NO_MORE_ITEMS) {
				event, err := d.w.GetReadWaitEvent(d.session)
				if err != nil {
					return nil, 0, fmt.Errorf("get read wait event: %w", err)
				}
				windows.WaitForSingleObject(event, windows.INFINITE)
				continue
			}
			return nil, 0, err
		}
		defer d.w.ReleaseReceivePacket(d.session, ptr)
		n := copy(p[off:], unsafe.Slice((*byte)(ptr), size))
		return ctx, n, nil
	}
}

func (d *tunDevice) Write(ctx context.Context, p []byte, off int) (context.Context, int, error) {
	n := len(p[off:])
	ptr, err := d.w.AllocateSendPacket(d.session, n)
	if err != nil {
		return nil, 0, err
	}
	copy(unsafe.Slice((*byte)(ptr), n), p[off:])
	d.w.SendPacket(d.session, ptr)
	return ctx, n, nil
}

func (d *tunDevice) Close() error {
	d.closeOnce.Do(func() {
		d.w.EndSession(d.session)
		d.w.CloseAdapter(d.adapter)
		d.w.Release()
	})
	return nil
}

func createTun(cfg config.TunEndpoint, log Logger) (*tunDevice, error) {
	w, err := LoadWintun()
	if err != nil {
		return nil, err
	}

	guid := nameToGUID(cfg.Name)
	adapter, err := w.CreateAdapter(cfg.Name, &guid)
	if err != nil {
		w.Release()
		return nil, fmt.Errorf("create adapter: %w", err)
	}

	session, err := w.StartSession(adapter, ringCapacity)
	if err != nil {
		w.CloseAdapter(adapter)
		w.Release()
		return nil, fmt.Errorf("start session: %w", err)
	}

	err = tunctl.UpIface(cfg.Name, cfg.CIDR)
	if err != nil {
		w.EndSession(session)
		w.CloseAdapter(adapter)
		w.Release()
		return nil, fmt.Errorf("up iface: %w", err)
	}

	log.Info("interface created", "name", cfg.Name, "cidr", cfg.CIDR)
	return &tunDevice{w: w, adapter: adapter, session: session}, nil
}

func nameToGUID(name string) windows.GUID {
	h := sha256.Sum256([]byte(name))
	return windows.GUID{
		Data1: uint32(h[0])<<24 | uint32(h[1])<<16 | uint32(h[2])<<8 | uint32(h[3]),
		Data2: uint16(h[4])<<8 | uint16(h[5]),
		Data3: uint16(h[6])<<8 | uint16(h[7]),
	}
}
