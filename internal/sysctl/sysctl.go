package sysctl

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
)

func IPForward() (int, error) {
	return readInt(IPForwardOption)
}

func SetIPForward(value int) error {
	if err := exec.Command("sysctl", "-w", fmt.Sprintf("%s=%d", IPForwardOption, value)).Run(); err != nil {
		return err
	}
	return nil
}

func RPFilter(iface string) (int, error) {
	return readInt(fmt.Sprintf("net.ipv4.conf.%s.rp_filter", iface))
}

func SetRPFilter(iface string, value int) error {
	option := fmt.Sprintf("net.ipv4.conf.%s.rp_filter=%d", iface, value)
	out, err := exec.Command("sysctl", "-w", option).CombinedOutput()
	if err != nil {
		return fmt.Errorf("sysctl failed: %s, err: %w", string(out), err)
	}
	return nil
}

func readInt(name string) (int, error) {
	cmd := exec.Command("sysctl", name)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("call command: %w", err)
	}

	re := regexp.MustCompile(fmt.Sprintf(`^%s\s*[=:]\s*(\d+)[\n]$`, name))
	matches := re.FindStringSubmatch(string(output))
	if len(matches) != 2 {
		return 0, fmt.Errorf("unexpected output")
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("atoi: %w", err)
	}

	return value, nil
}
