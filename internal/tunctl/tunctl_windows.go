package tunctl

import (
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strings"
)

func UpIface(name, cidr string) error {
	localAddr, localNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("parse cidr %v: %w", cidr, err)
	}
	mask := net.IP(localNet.Mask).String()

	out, err := exec.Command("netsh", "interface", "ip", "set", "address",
		"name="+name, "source=static", "addr="+localAddr.String(), "mask="+mask,
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("netsh set address: %s: %w", string(out), err)
	}
	return nil
}

func RouteAllToTun(tun, exceptIP string) error {
	gateway, err := getDefaultRoute()
	if err != nil {
		return fmt.Errorf("get default route: %w", err)
	}

	tunLocalIP, tunIfIndex, err := getTunInterface(tun)
	if err != nil {
		return fmt.Errorf("get tun interface: %w", err)
	}

	type routeCmd struct {
		args []string
		desc string
	}
	cmds := []routeCmd{
		{strings.Fields(fmt.Sprintf("route add %s %s", exceptIP, gateway)),
			"except route"},
		{strings.Fields(fmt.Sprintf("route add 0.0.0.0 mask 128.0.0.0 %s IF %d", tunLocalIP, tunIfIndex)),
			"0.0.0.0/1 route"},
		{strings.Fields(fmt.Sprintf("route add 128.0.0.0 mask 128.0.0.0 %s IF %d", tunLocalIP, tunIfIndex)),
			"128.0.0.0/1 route"},
	}

	for _, cmd := range cmds {
		out, err := exec.Command(cmd.args[0], cmd.args[1:]...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s (%s): %s: %w", cmd.desc, strings.Join(cmd.args, " "), strings.TrimSpace(string(out)), err)
		}
	}
	return nil
}

func DeleteRouteAll(exceptIP string) error {
	type routeCmd struct {
		args []string
		desc string
	}
	cmds := []routeCmd{
		{strings.Fields(fmt.Sprintf("route delete %s", exceptIP)),
			"except route"},
		{strings.Fields("route delete 0.0.0.0 mask 128.0.0.0"),
			"0.0.0.0/1 route"},
		{strings.Fields("route delete 128.0.0.0 mask 128.0.0.0"),
			"128.0.0.0/1 route"},
	}

	for _, cmd := range cmds {
		out, err := exec.Command(cmd.args[0], cmd.args[1:]...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s (%s): %s: %w", cmd.desc, strings.Join(cmd.args, " "), strings.TrimSpace(string(out)), err)
		}
	}
	return nil
}

func EnableMasquerade(_, _ string) error {
	return fmt.Errorf("masquerade is not supported on windows")
}

func DisableMasquerade(_, _ string) error {
	return nil
}

func getDefaultRoute() (gateway string, err error) {
	out, err := exec.Command("route", "print", "-4", "0.0.0.0").Output()
	if err != nil {
		return "", fmt.Errorf("route print: %w", err)
	}

	re := regexp.MustCompile(`^\s*0\.0\.0\.0\s+0\.0\.0\.0\s+(\S+)`)
	for _, line := range strings.Split(string(out), "\n") {
		m := re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		return m[1], nil
	}
	return "", fmt.Errorf("default route not found")
}

func getTunInterface(tun string) (ip string, ifIndex int, err error) {
	iface, err := net.InterfaceByName(tun)
	if err != nil {
		return "", 0, err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "", 0, err
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
			return ipnet.IP.String(), iface.Index, nil
		}
	}
	return "", 0, fmt.Errorf("tun interface %s has no IPv4 address", tun)
}
