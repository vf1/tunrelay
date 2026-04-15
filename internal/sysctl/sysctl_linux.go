package sysctl

import (
	"fmt"
	"os/exec"
)

const IPForwardOption = "net.ipv4.ip_forward"

func DisableIPv6(iface string) error {
	if err := exec.Command("sysctl", "-w", fmt.Sprintf("net.ipv6.conf.%s.disable_ipv6=1", iface)).Run(); err != nil {
		return err
	}
	return nil
}
