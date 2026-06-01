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
	gateway, ifIndex, err := getDefaultRoute()
	if err != nil {
		return fmt.Errorf("get default route: %w", err)
	}

	tunGateway, err := getTunGateway(tun)
	if err != nil {
		return fmt.Errorf("get tun gateway: %w", err)
	}

	type routeCmd struct {
		args []string
		desc string
	}
	cmds := []routeCmd{
		{strings.Fields(fmt.Sprintf("route add %s %s IF %s", exceptIP, gateway, ifIndex)),
			"except route"},
		{strings.Fields(fmt.Sprintf("route add 0.0.0.0 mask 128.0.0.0 %s", tunGateway)),
			"0.0.0.0/1 route"},
		{strings.Fields(fmt.Sprintf("route add 128.0.0.0 mask 128.0.0.0 %s", tunGateway)),
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

func getDefaultRoute() (gateway, ifIndex string, err error) {
	out, err := exec.Command("route", "print", "-4", "0.0.0.0").Output()
	if err != nil {
		return "", "", fmt.Errorf("route print: %w", err)
	}

	re := regexp.MustCompile(`^\s*0\.0\.0\.0\s+0\.0\.0\.0\s+(\S+)\s+(\S+)\s+(\d+)`)
	for _, line := range strings.Split(string(out), "\n") {
		m := re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		return m[2], m[3], nil
	}
	return "", "", fmt.Errorf("default route not found")
}

func getTunGateway(tun string) (string, error) {
	out, err := exec.Command("netsh", "interface", "ip", "show", "address", "name="+tun).Output()
	if err != nil {
		return "", fmt.Errorf("netsh show address: %w", err)
	}

	re := regexp.MustCompile(`(?i)IP\s+address[:\s]+(\d+\.\d+\.\d+\.\d+)`)
	m := re.FindStringSubmatch(string(out))
	if m == nil {
		return "", fmt.Errorf("tun interface %s has no IP address", tun)
	}
	return m[1], nil
}
