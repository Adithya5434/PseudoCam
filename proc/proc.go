// Package proc starts child processes without flashing a console window.
package proc

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// Command is exec.Command, but the child never opens a console window.
// Needed because the -H=windowsgui build has no console of its own, so
// Windows creates a new one for every adb / scrcpy / ffmpeg call.
func Command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	return cmd
}