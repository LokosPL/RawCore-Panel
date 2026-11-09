//go:build !windows

package main

import (
	"errors"
	"os/exec"
	"syscall"
	"time"
)

func hideWindow(cmd *exec.Cmd) {}
func cryptToken(data []byte, decrypt bool) ([]byte, error) {
	return nil, errors.New("Zapisywanie tokenu jest dostępne tylko w Windows (DPAPI)")
}
func waitParent(pid int) error {
	for i := 0; i < 120; i++ {
		if syscall.Kill(pid, 0) != nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
 return errors.New("Poprzedni panel nie zakończył pracy")
}

func updateNativeTheme(theme string) {}

func syncInstalledVersion(executable,version string) {}

