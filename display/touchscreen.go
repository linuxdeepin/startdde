package display

// #cgo pkg-config: x11 xi
// #cgo LDFLAGS: -lpthread
// #include "listen.h"
import "C"

import (
	"sync"

	"pkg.deepin.io/dde/api/dxinput"
	"pkg.deepin.io/dde/api/dxinput/common"
	dxutils "pkg.deepin.io/dde/api/dxinput/utils"
	gudev "pkg.deepin.io/gir/gudev-1.0"
)

type Touchscreen struct {
	Id         int32
	Name       string
	DeviceNode string
	Serial     string
}

type dxTouchscreens []*Touchscreen

var (
	devInfos           common.DeviceInfos
	touchscreenInfos   dxTouchscreens
	touchscreenInfosMu sync.Mutex
	gudevClient        = gudev.NewClient([]string{"input"})
)

func startDeviceListener() {
	C.start_device_listener()
}

func endDeviceListener() {
	C.end_device_listener()
}

//export handleDeviceChanged
func handleDeviceChanged() {
	logger.Debug("Device changed")

	_dpy.handleTouchscreenChanged()
}

func getDeviceInfos(force bool) common.DeviceInfos {
	if force || len(devInfos) == 0 {
		devInfos = dxutils.ListDevice()
	}

	return devInfos
}

func getTouchscreenInfos(force bool) dxTouchscreens {
	touchscreenInfosMu.Lock()
	defer touchscreenInfosMu.Unlock()

	if !force && len(touchscreenInfos) != 0 {
		return touchscreenInfos
	}

	touchscreenInfos = nil
	for _, v := range getDeviceInfos(force) {
		if v.Type == common.DevTypeTouchscreen {
			tmp, _ := dxinput.NewTouchscreenFromDevInfo(v)
			data, num := dxutils.GetProperty(tmp.Id, "Device Node")
			if len(data) == 0 {
				logger.Warningf("could not get DeviceNode for %s (%d)", tmp.Name, tmp.Id)
				continue
			}

			deviceFile := string(data[:num])
			device := gudevClient.QueryByDeviceFile(deviceFile)
			serial := device.GetProperty("ID_SERIAL")

			if serial == "" {
				continue
			}

			touchscreenInfos = append(touchscreenInfos, &Touchscreen{
				Id:         tmp.Id,
				Name:       tmp.Name,
				DeviceNode: deviceFile,
				Serial:     serial,
			})
		}
	}

	return touchscreenInfos
}
