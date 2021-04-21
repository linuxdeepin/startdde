package display

// #cgo pkg-config: x11 xi
// #cgo LDFLAGS: -lpthread
// #include "listen.h"
import "C"

import (
	"os"
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
			// 在虚拟机中v.Type判断触摸屏不准确
			// 为了防止虚拟机再次添加非ID_INPUT_TOUCHSCREEN设备到TouchscreenMap中
			// 通过gudevClient再次判断保证设备为触控屏
			deviceType := device.GetProperty("ID_INPUT_TOUCHSCREEN")

			// rockchip平板设备信息中没有ID_SERIAL, 无法准确识别触摸屏，使用DEVPATH作为唯一标识进行标记
			if os.Getenv("XDG_CURRENT_DESKTOP") == "Deepin-tablet" && serial == "" {
				serial = device.GetProperty("DEVPATH")
			}

			if serial == "" || deviceType == "" {
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

func (m *Manager) touchScreenSetRotation(direction uint16, output string) {
	touchSerial := ""
	for ts, op := range m.TouchMap {
		if op == output {
			touchSerial = ts
			break
		}
	}
	if touchSerial == "" {
		logger.Errorf("get touchSerial failed")
		return
	}

	for _, ts := range m.Touchscreens {
		if ts.Serial != touchSerial {
			continue
		}
		touchScreen, err := dxinput.NewTouchscreen(ts.Id)
		if err != nil {
			logger.Warningf("NewTouchScreen %d failed", ts.Id)
		}
		err = touchScreen.SetRotation(uint8(direction))
		if err != nil {
			logger.Warningf("touchScreen %d SetRotation failed", ts.Id)
		}
	}
}
