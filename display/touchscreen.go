// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display

import (
	"strings"

	"github.com/godbus/dbus"
	inputdevices "github.com/linuxdeepin/go-dbus-factory/com.deepin.system.inputdevices"
)

type Touchscreen struct {
	Id         int32
	Name       string
	DeviceNode string
	Serial     string
	UUID       string
	outputName string
	busType    uint8
	width      float64
	height     float64
	path       dbus.ObjectPath
}

type dxTouchscreens []*Touchscreen

func (ts *dxTouchscreens) removeByPath(path dbus.ObjectPath) {
	//touchScreenUUID := ""
	i := -1
	for index, v := range *ts {
		if v.path == path {
			i = index
			//touchScreenUUID = v.UUID
		}
	}

	if i == -1 {
		return
	}

	ts.removeByIdx(i)
}

func (ts *dxTouchscreens) removeByDeviceNode(deviceNode string) {
	i := -1
	for idx, v := range *ts {
		if v.DeviceNode == deviceNode {
			i = idx
			break
		}
	}

	if i == -1 {
		return
	}

	ts.removeByIdx(i)
}

func (ts *dxTouchscreens) removeByIdx(i int) {
	tsr := *ts
	if len(tsr) > i {
		// see https://github.com/golang/go/wiki/SliceTricks
		tsr[i] = tsr[len(tsr)-1]
		tsr[len(tsr)-1] = nil
		*ts = tsr[:len(tsr)-1]
	}
}

type touchscreenManager interface {
	refreshDevicesFromDisplayServer()
	addTouchscreen(path dbus.ObjectPath) error
	removeTouchscreen(path dbus.ObjectPath)
	touchscreenList() dxTouchscreens
	associateTouchscreen(monitor *Monitor, touchUUID string) error
}

type touchscreenManagerOuter interface {
	completeTouchscreenID(*Touchscreen)
}

type baseTouchscreenManager struct {
	outer touchscreenManagerOuter

	sysBus *dbus.Conn
	list   dxTouchscreens
}

func (tm *baseTouchscreenManager) addTouchscreen(path dbus.ObjectPath) error {
	t, err := inputdevices.NewTouchscreen(tm.sysBus, path)
	if err != nil {
		return err
	}

	touchscreen := &Touchscreen{
		path: path,
	}
	touchscreen.Name, _ = t.Name().Get(0)
	touchscreen.DeviceNode, _ = t.DevNode().Get(0)
	touchscreen.Serial, _ = t.Serial().Get(0)
	touchscreen.UUID, _ = t.UUID().Get(0)
	touchscreen.outputName, _ = t.OutputName().Get(0)
	touchscreen.width, _ = t.Width().Get(0)
	touchscreen.height, _ = t.Height().Get(0)

	touchscreen.busType = busTypeUnknown
	busType, _ := t.BusType().Get(0)
	if strings.ToLower(busType) == "usb" {
		touchscreen.busType = busTypeUSB
	}

	tm.outer.completeTouchscreenID(touchscreen)

	tm.list = append(tm.list, touchscreen)

	return nil
}

func (tm *baseTouchscreenManager) removeTouchscreen(path dbus.ObjectPath) {
	tm.list.removeByPath(path)
}

func (tm *baseTouchscreenManager) touchscreenList() dxTouchscreens {
	return tm.list
}
