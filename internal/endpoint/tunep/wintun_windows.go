package tunep

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Wintun struct {
	dll *windows.DLL

	createAdapter    *windows.Proc
	closeAdapter     *windows.Proc
	startSession     *windows.Proc
	endSession       *windows.Proc
	receivePacket    *windows.Proc
	releaseReceive   *windows.Proc
	allocateSend     *windows.Proc
	sendPacket       *windows.Proc
	getReadWaitEvent *windows.Proc
}

type Adapter windows.Handle

type Session windows.Handle

func LoadWintun() (*Wintun, error) {
	dll, err := windows.LoadDLL("wintun.dll")
	if err != nil {
		return nil, fmt.Errorf("load wintun.dll: %w", err)
	}

	w := &Wintun{dll: dll}
	procs := []struct {
		name string
		p    **windows.Proc
	}{
		{"WintunCreateAdapter", &w.createAdapter},
		{"WintunCloseAdapter", &w.closeAdapter},
		{"WintunStartSession", &w.startSession},
		{"WintunEndSession", &w.endSession},
		{"WintunReceivePacket", &w.receivePacket},
		{"WintunReleaseReceivePacket", &w.releaseReceive},
		{"WintunAllocateSendPacket", &w.allocateSend},
		{"WintunSendPacket", &w.sendPacket},
		{"WintunGetReadWaitEvent", &w.getReadWaitEvent},
	}
	for _, pr := range procs {
		*pr.p, err = dll.FindProc(pr.name)
		if err != nil {
			dll.Release()
			return nil, fmt.Errorf("wintun.dll: find %s: %w", pr.name, err)
		}
	}
	return w, nil
}

func (w *Wintun) Release() {
	w.dll.Release()
}

func (w *Wintun) callR1(proc *windows.Proc, op string, args ...uintptr) (uintptr, error) {
	r1, _, e1 := proc.Call(args...)
	if r1 == 0 {
		if e1 != nil {
			return 0, fmt.Errorf("%s: %w", op, e1)
		}
		return 0, fmt.Errorf("%s: failed", op)
	}
	return r1, nil
}

func (w *Wintun) CreateAdapter(name string, guid *windows.GUID) (Adapter, error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	typePtr, err := windows.UTF16PtrFromString("Wintun")
	if err != nil {
		return 0, err
	}

	r1, err := w.callR1(w.createAdapter, "create adapter",
		uintptr(unsafe.Pointer(namePtr)), uintptr(unsafe.Pointer(typePtr)), uintptr(unsafe.Pointer(guid)))
	return Adapter(r1), err
}

func (w *Wintun) CloseAdapter(a Adapter) {
	w.closeAdapter.Call(uintptr(a))
}

func (w *Wintun) StartSession(a Adapter, capacity uint32) (Session, error) {
	r1, err := w.callR1(w.startSession, "start session", uintptr(a), uintptr(capacity))
	return Session(r1), err
}

func (w *Wintun) EndSession(s Session) {
	w.endSession.Call(uintptr(s))
}

func (w *Wintun) GetReadWaitEvent(s Session) (windows.Handle, error) {
	r1, err := w.callR1(w.getReadWaitEvent, "get read wait event", uintptr(s))
	return windows.Handle(r1), err
}

func (w *Wintun) ReceivePacket(s Session) (unsafe.Pointer, uint32, error) {
	var size uint32
	r1, err := w.callR1(w.receivePacket, "receive packet", uintptr(s), uintptr(unsafe.Pointer(&size)))
	return unsafe.Pointer(r1), size, err
}

func (w *Wintun) ReleaseReceivePacket(s Session, packet unsafe.Pointer) {
	w.releaseReceive.Call(uintptr(s), uintptr(packet))
}

func (w *Wintun) AllocateSendPacket(s Session, size int) (unsafe.Pointer, error) {
	r1, err := w.callR1(w.allocateSend, "allocate send packet", uintptr(s), uintptr(size))
	return unsafe.Pointer(r1), err
}

func (w *Wintun) SendPacket(s Session, packet unsafe.Pointer) {
	w.sendPacket.Call(uintptr(s), uintptr(packet))
}
