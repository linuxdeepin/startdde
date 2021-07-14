package display

// #cgo pkg-config: glib-2.0
// #cgo LDFLAGS: -lm
// #include "sensor.h"
import "C"

import (
	"fmt"
)

var dev_fd C.int = -1
var data_fd C.int = -1

func initSensorListener() {
	dev_fd = C.open_device()
	if dev_fd < 0 {
		fmt.Printf("Failed to open sensor device")
		return
	}
	C.read_calibration(dev_fd)
}

func startSensorListener() {
	data_fd = C.get_input()
	if data_fd < 0 {
		fmt.Printf("Failed to get sensor input event")
		return
	}

	go func() {
		C.read_events(&data_fd)
	}()
}

func stopSensorListener() {
	if data_fd < 0 {
		return
	}
	C.close_input(data_fd)
	data_fd = -1
}
