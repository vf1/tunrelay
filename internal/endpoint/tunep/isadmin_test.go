//go:build !windows

package tunep

import (
	"os"
)

func isAdmin() bool {
	return os.Getuid() == 0
}
