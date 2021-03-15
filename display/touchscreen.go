package display

import (
	"sync"

	"github.com/godbus/dbus"
	"pkg.deepin.io/dde/api/dxinput"
	"pkg.deepin.io/dde/api/dxinput/common"
	dxutils "pkg.deepin.io/dde/api/dxinput/utils"
)

const (
	BusTypeUnknown uint8 = iota
	BusTypeUSB
)

type Touchscreen struct {
	Id         int32
	Name       string
	DeviceNode string
	Serial     string
	uuid       string
	outputName string
	busType    uint8
	width      float64
	height     float64
	path       dbus.ObjectPath
}

type dxTouchscreens []*Touchscreen

var (
	devInfos           common.DeviceInfos
	touchscreenInfos   dxTouchscreens
	touchscreenInfosMu sync.Mutex
)

func getDeviceInfos(force bool) common.DeviceInfos {
	if force || len(devInfos) == 0 {
		devInfos = dxutils.ListDevice()
	}

	return devInfos
}

func getXTouchscreenInfo(t *Touchscreen) {
	for _, v := range getDeviceInfos(false) {
		if v.Type != common.DevTypeTouchscreen {
			continue
		}

		tmp, _ := dxinput.NewTouchscreenFromDevInfo(v)
		data, num := dxutils.GetProperty(tmp.Id, "Device Node")
		if len(data) == 0 {
			logger.Warningf("could not get DeviceNode for %s (%d)", tmp.Name, tmp.Id)
			continue
		}

		deviceNode := string(data[:num])
		logger.Warningf("deviceNode: %s", deviceNode)

		logger.Warningf("devNode: %s, deviceNode: %s", t.DeviceNode, deviceNode)
		if t.DeviceNode != deviceNode {
			continue
		}

		t.Id = tmp.Id
	}
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
