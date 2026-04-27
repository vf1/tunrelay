package tunctl

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

func CreateTun(name string) (*os.File, error) {
	ifIndex := -1
	if name != "utun" {
		_, err := fmt.Sscanf(name, "utun%d", &ifIndex)
		if err != nil || ifIndex < 0 {
			return nil, fmt.Errorf("interface name must be utun[0-9]*")
		}
	}

	syscall.ForkLock.RLock()
	fd, err := unix.Socket(unix.AF_SYSTEM, unix.SOCK_DGRAM, 2)
	if err != nil {
		syscall.ForkLock.RUnlock()
		return nil, fmt.Errorf("socket: %w", err)
	}
	unix.CloseOnExec(fd)
	syscall.ForkLock.RUnlock()

	ctlInfo := &unix.CtlInfo{}
	copy(ctlInfo.Name[:], []byte("com.apple.net.utun_control"))
	if err := unix.IoctlCtlInfo(fd, ctlInfo); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("ioctl ctlinfo: %w", err)
	}

	sc := &unix.SockaddrCtl{
		ID:   ctlInfo.Id,
		Unit: uint32(ifIndex) + 1,
	}
	if err := unix.Connect(fd, sc); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("connect utun: %w", err)
	}

	if err := unix.SetNonblock(fd, true); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("set nonblock: %w", err)
	}

	return os.NewFile(uintptr(fd), name), nil
}

func UpIface(name, localAddr, peerAddr, mask string) error {
	if err := exec.Command("ifconfig", name, localAddr, peerAddr, "netmask", mask, "up").Run(); err != nil {
		return fmt.Errorf("ifconfig (%v): %w", name, err)
	}
	return nil
}

func AddIfaceRoute(name, addr string) error {
	if err := exec.Command("route", "add", "-ifscope", name, "default", addr).Run(); err != nil {
		return fmt.Errorf("route add: %w", err)
	}
	return nil
}

func RouteAllToTun(tun, exceptIP string) error {
	gateway, err := getDefaultRoute("gateway")
	if err != nil {
		return err
	}

	_ = exec.Command("route", "delete", exceptIP).Run()

	cmds := []string{
		fmt.Sprintf("route add -host %s %s", exceptIP, gateway),
		"route add -net 0.0.0.0/1 -interface u" + tun,
		"route add -net 128.0.0.0/1 -interface u" + tun,
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
		fmt.Sprintf("route delete %s", exceptIP),
		"route delete 0.0.0.0/1",
		"route delete 128.0.0.0/1",
	}
	for _, cmd := range cmds {
		args := strings.Split(cmd, " ")
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			return fmt.Errorf("cmd (%v): %v", cmd, err)
		}
	}
	return nil
}

const ruleName = "tunrelay"

func EnableMasquerade(saddr, oifname string) error {
	iface, err := getDefaultRoute("interface")
	if err != nil {
		return err
	}

	rule := fmt.Sprintf("nat on %s from %s to any -> (%s)\n", iface, saddr, iface)
	cmd := exec.Command("pfctl", "-Ef", "-")
	cmd.Stdin = strings.NewReader(rule)

	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(rule)
		fmt.Println(string(out))
		return fmt.Errorf("add pftcl rule: %w", err)
	}

	return nil
}

func DisableMasquerade(_, _ string) error {
	if err := exec.Command("pfctl", "-a", ruleName, "-F", "rules").Run(); err != nil {
		return fmt.Errorf("pfctl del rule: %w", err)
	}
	return nil
}

func getDefaultRoute(prop string) (value string, err error) {
	out, err := exec.Command("route", "-n", "get", "default").Output()
	if err != nil {
		return "", fmt.Errorf("get default route: %w", err)
	}
	re := regexp.MustCompile(`(?m)^\s*` + prop + `:\s*(\S+)\s*$`)
	matches := re.FindSubmatch(out)
	if len(matches) < 2 {
		return "", fmt.Errorf("can't parse '%s' in route", prop)
	}
	return string(matches[1]), nil
}
