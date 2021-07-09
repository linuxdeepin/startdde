package display

// #cgo pkg-config: glib-2.0
// #cgo LDFLAGS: -lm
// #include "sensor.h"
import "C"

import (
	"fmt"
)

var dev_fd C.int
var data_fd C.int

func initSensorListener() {
	dev_fd = C.open_device()
	if dev_fd < 0 {
		fmt.Printf("Failed to open sensor device")
		return
	}
	C.read_calibration(dev_fd)

	data_fd = C.get_input()
	if data_fd < 0 {
		fmt.Printf("Failed to get sensor input event")
		return
	}
}

func eventLoop() {
	if data_fd < 0 {
		return
	}
	C.read_events(data_fd)
}

func startSensorListener() {
	C.start_device(dev_fd)
}

func stopSensorListener() {
	C.stop_device(dev_fd)
}
