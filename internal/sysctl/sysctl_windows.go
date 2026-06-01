package sysctl

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

func IPForward() (int, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\Tcpip\Parameters`, registry.QUERY_VALUE)
	if err != nil {
		return 0, fmt.Errorf("open registry: %w", err)
	}
	defer k.Close()

	val, _, err := k.GetIntegerValue("IPEnableRouter")
	if err != nil {
		return 0, fmt.Errorf("read IPEnableRouter: %w", err)
	}
	return int(val), nil
}

func SetIPForward(value int) error {
	return fmt.Errorf("enable IP forwarding manually:\n  reg add HKLM\\SYSTEM\\CurrentControlSet\\Services\\Tcpip\\Parameters /v IPEnableRouter /t REG_DWORD /d 1 /f\n  then reboot or run: netsh interface ipv4 set global forwarding=enabled")
}

func RPFilter(_ string) (int, error) {
	return 0, nil
}

func SetRPFilter(_ string, _ int) error {
	return nil
}