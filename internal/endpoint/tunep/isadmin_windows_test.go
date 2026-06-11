//go:build windows

package tunep

func isAdmin() bool {
	_, err := LoadWintun()
	return err == nil
}
