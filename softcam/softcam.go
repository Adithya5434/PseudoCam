package softcam

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

type Camera struct {
	handle uintptr
	width  int
	height int
	fps    float32
}

var (
	dll                 *syscall.DLL
	scCreateCamera      *syscall.Proc
	scDeleteCamera      *syscall.Proc
	scSendFrame         *syscall.Proc
	scWaitForConnection *syscall.Proc
	scIsConnected       *syscall.Proc
)

// load loads softcam.dll lazily (after Install() has extracted it).
func load() error {
	if dll != nil {
		return nil
	}

	path := DLLPath()
	if _, err := os.Stat(path); err != nil {
		path = "softcam.dll" // fall back to cwd / PATH
	}

	d, err := syscall.LoadDLL(path)
	if err != nil {
		return fmt.Errorf("softcam.dll not loaded: %w", err)
	}

	procs := []struct {
		dst  **syscall.Proc
		name string
	}{
		{&scCreateCamera, "scCreateCamera"},
		{&scDeleteCamera, "scDeleteCamera"},
		{&scSendFrame, "scSendFrame"},
		{&scWaitForConnection, "scWaitForConnection"},
		{&scIsConnected, "scIsConnected"},
	}
	for _, p := range procs {
		proc, err := d.FindProc(p.name)
		if err != nil {
			d.Release()
			return fmt.Errorf("softcam.dll missing %s: %w", p.name, err)
		}
		*p.dst = proc
	}

	dll = d
	return nil
}

func New(width int, height int, fps float32) (*Camera, error) {
	if err := load(); err != nil {
		return nil, err
	}

	if width <= 0 || height <= 0 || fps <= 0 {
		return nil, fmt.Errorf("invalid camera parameters")
	}

	if width%4 != 0 || height%4 != 0 {
		return nil, fmt.Errorf("width and height must be multiples of 4")
	}

	fpsBits := *(*uint32)(unsafe.Pointer(&fps))

	handle, _, _ := scCreateCamera.Call(uintptr(width), uintptr(height), uintptr(fpsBits))

	if handle == 0 {
		return nil, fmt.Errorf("failed to create camera")
	}

	return &Camera{handle: handle, width: width, height: height, fps: fps}, nil
}

func (c *Camera) Close() {
	if c == nil || c.handle == 0 {
		return
	}

	scDeleteCamera.Call(c.handle)
	c.handle = 0
}

func (c *Camera) SendFrame(data []byte) error {
	if c == nil || c.handle == 0 {
		return fmt.Errorf("camera is not initialized")
	}

	frameSize := c.width * c.height * 3
	if len(data) != frameSize {
		return fmt.Errorf("invalid frame size: expected %d bytes, got %d bytes", frameSize, len(data))
	}

	scSendFrame.Call(c.handle, uintptr(unsafe.Pointer(&data[0])))

	return nil
}

func (c *Camera) WaitForConnection(timeout float32) bool {
	if c == nil || c.handle == 0 {
		return false
	}

	timeoutBits := *(*uint32)(unsafe.Pointer(&timeout))
	result, _, _ := scWaitForConnection.Call(c.handle, uintptr(timeoutBits))

	return result != 0
}

func (c *Camera) IsConnected() bool {
	if c == nil || c.handle == 0 {
		return false
	}

	result, _, _ := scIsConnected.Call(c.handle)

	return result != 0
}