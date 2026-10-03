package scrcpy

import (
	"fmt"
	"pseudocam/proc"
	"strconv"
	"strings"
)

type Camera struct {
	ID          int
	Name        string
	MaxWidth    int
	MaxHeight   int
	Sizes       []CameraSize
}

type CameraSize struct {
	Width  int
	Height int
}

func (s *Scrcpy) ListCameraSizes(deviceSerial string) ([]Camera, error) {
	cmd := proc.Command(s.Path,"-s",deviceSerial,"--list-camera-sizes")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run scrcpy: %w", err)
	}

	return parseCameraSizes(string(output))
}

func parseCameraSizes(output string) ([]Camera, error) {
	var cameras []Camera

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)

		// Camera line
		if strings.HasPrefix(line, "--camera-id=") {
			rest := strings.TrimPrefix(line, "--camera-id=")

			parts := strings.SplitN(rest, " ", 2)
			if len(parts) != 2 {
				continue
			}

			cameraID, err := strconv.Atoi(parts[0])
			if err != nil {
				continue
			}

			info := strings.TrimSpace(parts[1])

			info = strings.TrimPrefix(info, "(")
			info = strings.TrimSuffix(info, ")")

			fields := strings.Split(info, ",")
			if len(fields) < 2 {
				continue
			}

			name := strings.TrimSpace(fields[0])
			maxResolution := strings.TrimSpace(fields[1])

			resolutionParts := strings.Split(maxResolution, "x")
			if len(resolutionParts) != 2 {
				continue
			}

			maxWidth, err := strconv.Atoi(resolutionParts[0])
			if err != nil {
				continue
			}

			maxHeight, err := strconv.Atoi(resolutionParts[1])
			if err != nil {
				continue
			}

			cameras = append(cameras, Camera{
				ID:       cameraID,
				Name:     name,
				MaxWidth: maxWidth,
				MaxHeight: maxHeight,
				Sizes:    []CameraSize{},
			})

			continue
		}

		// Resolution line
		if strings.HasPrefix(line, "- ") {
			if len(cameras) == 0 {
				continue
			}

			resolution := strings.TrimSpace(
				strings.TrimPrefix(line, "- "),
			)

			resolutionParts := strings.Split(resolution, "x")
			if len(resolutionParts) != 2 {
				continue
			}

			width, err := strconv.Atoi(resolutionParts[0])
			if err != nil {
				continue
			}

			height, err := strconv.Atoi(resolutionParts[1])
			if err != nil {
				continue
			}

			cameras[len(cameras)-1].Sizes = append(
				cameras[len(cameras)-1].Sizes,
				CameraSize{
					Width:  width,
					Height: height,
				},
			)
		}
	}

	if len(cameras) == 0 {
		return nil, fmt.Errorf("no cameras found")
	}

	return cameras, nil
}