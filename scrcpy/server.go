package scrcpy

import (
	"bufio"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"pseudocam/adb"
	"pseudocam/proc"
)

type Scrcpy struct {
	Path       string
	ServerPath string
	Version    string
}

type ServerProcess struct {
	Cmd  *exec.Cmd
	Port int
}

func FindScrcpy() (*Scrcpy, error) {
	path, err := exec.LookPath("scrcpy")
	if err != nil {
		return nil, fmt.Errorf("scrcpy executable not found in PATH")
	}

	version, err := getVersion(path)
	if err != nil {
		return nil, err
	}

	serverPath := filepath.Join(filepath.Dir(path), "scrcpy-server")
	return &Scrcpy{
		Path:      path,
		ServerPath: serverPath,
		Version:   version,
	}, nil
}

func getVersion(executable string) (string, error) {
	cmd := proc.Command(executable, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute scrcpy: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) == 0 {
		return "", fmt.Errorf("unexpected output from scrcpy --version")
	}

	versionLine := strings.TrimSpace(lines[0])
	version := strings.Fields(versionLine)[1]

	return version, nil
}


func (s *Scrcpy) StartServer(device adb.Device , cameraID int, size *CameraSize, port int) (*ServerProcess, error) {
	// push scrcpy server
	push := proc.Command("adb", "-s", device.Serial, "push", s.ServerPath, "/data/local/tmp/scrcpy-server.jar")
	output, err := push.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to push scrcpy server: %w\n%s", err, strings.TrimSpace(string(output)))
	}

	// ADB port forword
	forword := proc.Command("adb", "-s", device.Serial, "forward", fmt.Sprintf("tcp:%d", port), "localabstract:scrcpy")
	output, err = forword.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to create ADB port forward: %w\n%s", err, strings.TrimSpace(string(output)))
	}

	// start scrcpy server
	args := []string{
		"adb",
		"-s",
		device.Serial,
		"shell",
		"CLASSPATH=/data/local/tmp/scrcpy-server.jar",
		"app_process",
		"/",
		"com.genymobile.scrcpy.Server",
		s.Version,
		"tunnel_forward=true",
		"video_source=camera",
		fmt.Sprintf("camera_id=%d", cameraID),
		"audio=false",
		"control=false",
		"cleanup=false",
		"raw_stream=true",
	}

	if size != nil {
		args = append(args, fmt.Sprintf("camera_size=%dx%d", size.Width, size.Height))
	}

	cmd := proc.Command(args[0], args[1:]...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create scrcpy stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start scrcpy server: %w", err)
	}

	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		line := scanner.Text()

		fmt.Println("[scrcpy]", line)

		if strings.Contains(line, "[server] INFO: Device:") {
			return &ServerProcess{
				Cmd:  cmd,
				Port: port,
			}, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read scrcpy server output: %w", err)
	}

	return nil, fmt.Errorf("scrcpy server stopped before initialization completed")
}


func (p *ServerProcess) Close() error {
	if p == nil || p.Cmd == nil || p.Cmd.Process == nil {
		return nil
	}

	if err := p.Cmd.Process.Kill(); err != nil {
		return err
	}

	return p.Cmd.Wait()
}