package tunctl

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/songgao/water"
)

func CreateTun(name string) (*water.Interface, error) {
	tun, err := water.New(water.Config{
		DeviceType:             water.TUN,
		PlatformSpecificParams: water.PlatformSpecificParams{Name: name},
	})
	if err != nil {
		return nil, fmt.Errorf("create tunnel: %w", err)
	}
	return tun, nil
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
