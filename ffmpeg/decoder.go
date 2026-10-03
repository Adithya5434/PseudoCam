package ffmpeg

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"

	"pseudocam/frame"
	"pseudocam/proc"
)


type Decoder struct {
	Cmd    *exec.Cmd
	Stdout io.ReadCloser
	Width  int
	Height int
}

func Start(port int, width int, height int) (*Decoder, error) {
	cmd := proc.Command(
		"ffmpeg",
		"-hide_banner",
		"-loglevel", "info",
		"-flags", "low_delay",
		"-f", "h264",
		"-analyzeduration", "1M",
		"-probesize", "10M",
		"-i", fmt.Sprintf("tcp://127.0.0.1:%d", port),
		"-f", "rawvideo",
		"-pix_fmt", "yuv420p",
		"pipe:1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create FFmpeg stdout pipe: %w",err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create FFmpeg stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			fmt.Println("[FFmpeg]", scanner.Text())
		}
	}()

	return &Decoder{
		Cmd:    cmd,
		Stdout: stdout,
		Width:  width,
		Height: height,
	}, nil
}


func (d *Decoder) ReadFrame(f *frame.Frame) error {
	if f.Width != d.Width || f.Height != d.Height {
		return fmt.Errorf("frame size mismatch: decoder=%dx%d frame=%dx%d", d.Width, d.Height, f.Width, f.Height)
	}

	_, err := io.ReadFull(d.Stdout, f.Data)
	if err != nil {
		return fmt.Errorf("failed to read frame: %w", err)
	}

	return nil
}

func (d *Decoder) Close() error {
	if d.Stdout != nil {
		d.Stdout.Close()
	}

	if d.Cmd != nil && d.Cmd.Process != nil {
		_ = d.Cmd.Process.Kill()
		_ = d.Cmd.Wait()
	}

	return nil
}