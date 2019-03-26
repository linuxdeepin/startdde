/*
 * Copyright (C) 2017 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package display

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"

	"pkg.deepin.io/dde/api/dxinput"
	"pkg.deepin.io/dde/api/dxinput/utils"
)

type uint16Splice []uint16

func (s uint16Splice) Len() int {
	return len(s)
}

func (s uint16Splice) Less(i, j int) bool {
	return s[i] < s[j]
}

func (s uint16Splice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s uint16Splice) equal(list uint16Splice) bool {
	sort.Sort(list)
	i, j := len(s), len(list)
	if i != j {
		return false
	}
	for v := 0; v < i; v++ {
		if s[v] != list[v] {
			return false
		}
	}
	return true
}

func rotateInputDevices(rotation uint16) {
	devs := utils.ListDevice()
	for _, dev := range devs {
		if dev.Type == utils.DevTypeMouse {
			m, _ := dxinput.NewMouseFromDeviceInfo(dev)
			err := m.SetRotation(uint8(rotation))
			if err != nil {
				logger.Warning(err)
			}
		}
		if dev.Type == utils.DevTypeTouchpad {
			tpad, _ := dxinput.NewTouchpadFromDevInfo(dev)
			err := tpad.SetRotation(uint8(rotation))
			if err != nil {
				logger.Warning(err)
			}
		}
		if dev.Type == utils.DevTypeTouchscreen {
			touch, _ := dxinput.NewTouchscreenFromDevInfo(dev)
			err := touch.SetRotation(uint8(rotation))
			if err != nil {
				logger.Warning(err)
			}
		}
	}
}

func doAction(cmd string) error {
	logger.Debug("Command:", cmd)
	c := exec.Command("/bin/sh", "-c", "exec "+cmd)
	var errBuf bytes.Buffer
	c.Stderr = &errBuf
	err := c.Run()
	if err != nil {
		return fmt.Errorf("%s, stdErr: %s", err.Error(), errBuf.Bytes())
	}
	return nil
}

func jsonMarshal(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func jsonUnmarshal(data string, ret interface{}) error {
	return json.Unmarshal([]byte(data), ret)
}
