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
	out, err := exec.Command("route", "-n", "get", "default").Output()
	if err != nil {
		return fmt.Errorf("get default route: %w", err)
	}
	re := regexp.MustCompile(`(?m)^\s*gateway:\s*([0-9a-fA-F:.]+)\s*$`)
	matches := re.FindSubmatch(out)
	if len(matches) < 2 {
		return fmt.Errorf("can't parse route")
	}
	defRoute := string(matches[1])

	_ = exec.Command("route", "delete", exceptIP).Run()

	cmds := []string{
		fmt.Sprintf("route add -host %s %s", exceptIP, defRoute),
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
