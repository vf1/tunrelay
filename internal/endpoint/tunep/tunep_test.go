package tunep

import (
	"fmt"
	"testing"
	"time"

	"tunrelay/internal/config"
)

type testLog struct{}

func (testLog) Info(msg string, args ...any) {
	fmt.Print(msg)
	for i, a := range args {
		if i%2 == 0 {
			fmt.Print(" ", a, "=")
		} else {
			fmt.Print(a)
		}
	}
	fmt.Println()
}

func TestCreateClose(t *testing.T) {
	if !isAdmin() {
		t.Skip("requires elevated privileges")
	}

	name := fmt.Sprintf("tun%d", time.Now().UnixMilli()%10000)
	cfg := config.TunEndpoint{
		Name: name,
		CIDR: "10.99.0.2/24",
		Peer: "10.99.0.1",
	}

	d, err := createTun(cfg, testLog{})
	if err != nil {
		t.Fatalf("createTun: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
