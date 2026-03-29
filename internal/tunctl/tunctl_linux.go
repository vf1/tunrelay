package tunctl

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

func CreateTun(name string) (*os.File, error) {
	const path = "/dev/net/tun"
	fd, err := unix.Open(path, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("unix open: %w", err)
	}

	ifreq, err := unix.NewIfreq(name)
	if err != nil {
		return nil, fmt.Errorf("new ifreq: %w", err)
	}
	ifreq.SetUint16(unix.IFF_TUN | unix.IFF_NO_PI)

	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(unix.TUNSETIFF),
		uintptr(unsafe.Pointer(ifreq)),
	)
	if errno != 0 {
		return nil, os.NewSyscallError("ioctl", errno)
	}

	f := os.NewFile(uintptr(fd), path)

	return f, nil
}

func UpIface(name, CIDR string) error {
	_ = exec.Command("ip", "addr", "flush", "dev", name).Run()
	if err := exec.Command("ip", "addr", "add", CIDR, "dev", name).Run(); err != nil {
		return fmt.Errorf("add ip (%v): %w", CIDR, err)
	}
	if err := exec.Command("ip", "link", "set", "dev", name, "up").Run(); err != nil {
		return fmt.Errorf("up iface: %v", err)
	}
	return nil
}

func RouteAllToTun(tun, exceptIP string) error {
	out, err := exec.Command("ip", "route", "show", "default").Output()
	if err != nil {
		return fmt.Errorf("get default route: %w", err)
	}
	strs := strings.Split(string(out), " ")
	if len(strs) < 3 {
		return fmt.Errorf("can't parse route")
	}
	defRoute := strs[2]

	_ = exec.Command("ip", "route", "delete", exceptIP).Run()

	cmds := []string{
		fmt.Sprintf("ip route add %s via %s", exceptIP, defRoute),
		"ip route add 0.0.0.0/1 dev " + tun,
		"ip route add 128.0.0.0/1 dev " + tun,
	}
	for _, cmd := range cmds {
		args := strings.Split(cmd, " ")
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			return fmt.Errorf("cmd (%v): %v", cmd, err)
		}
	}
	return nil
}

func DeleteRouteAll(exceptIP string) error {
	cmds := []string{
		fmt.Sprintf("ip route delete %s", exceptIP),
		"ip route delete 0.0.0.0/1",
		"ip route delete 128.0.0.0/1",
	}
	for _, cmd := range cmds {
		args := strings.Split(cmd, " ")
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			return fmt.Errorf("cmd (%v): %v", cmd, err)
		}
	}
	return nil
}
