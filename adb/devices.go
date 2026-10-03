package adb

import (
	"strings"

	"pseudocam/proc"
)


type Device struct {
	Serial string
	State  string
}

func ListDevices() ([]Device, error) {
	cmd := proc.Command("adb", "devices")

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	var devices []Device

	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of devices") || strings.HasPrefix(line, "* daemon") {
			continue
		}

		parts := strings.Fields(line)

		if len(parts) >= 2 && parts[1] == "device" {
			devices = append(devices, Device{
				Serial: parts[0],
				State: parts[1],
			})
		}
	}

	return devices, nil
}