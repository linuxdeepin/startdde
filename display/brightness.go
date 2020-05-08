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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"pkg.deepin.io/dde/startdde/display/brightness"
)

type InvalidOutputNameError struct {
	Name string
}

func round(value float64) float64 {
	value, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", value), 64)
	return value
}

func (err InvalidOutputNameError) Error() string {
	return fmt.Sprintf("invalid output name %q", err.Name)
}

func (m *Manager) saveBrightness() {
	jsonStr := jsonMarshal(m.Brightness)
	m.settings.SetString(gsKeyBrightness, jsonStr)
}

func (m *Manager) changeBrightness(raised bool) error {
	var step float64 = 0.05
	if !raised {
		step = -step
	}

	monitors := m.getConnectedMonitors()

	for _, monitor := range monitors {
		v, ok := m.Brightness[monitor.Name]
		if !ok {
			v = 1.0
		}

		var br float64
		//setBr := true

		//if blCtrl, err := brightness.GetBacklightController(monitor.ID, m.xConn); err != nil {
		//	logger.Debugf("get output %q backlight controller failed: %v", monitor.Name, err)
		//} else {
		//	max := blCtrl.MaxBrightness
		//	cur, err := blCtrl.GetBrightness()
		//	if err == nil {
		//		// TODO: Some drivers will also set the brightness when the brightness up/down key is pressed
		//		hv := float64(cur) / float64(max)
		//		avg := (v + hv) / 2
		//		delta := (v - hv) / avg
		//		logger.Debugf("v: %g, hv: %g, avg: %g delta: %g", v, hv, avg, delta)
		//
		//		if math.Abs(delta) > 0.05 {
		//			logger.Debug("backlight actual brightness is not set")
		//			setBr = false
		//			br = hv
		//		}
		//	}
		//}

		//if setBr {
		br = v + step
		if br > 1.0 {
			br = 1.0
		}
		if br < 0.1 {
			br = 0.1
		}
		logger.Debug("[changeBrightness] will set to:", monitor.Name, br)
		err := m.doSetBrightness(br, monitor.Name)
		if err != nil {
			return err
		}
		//} else {
		//	logger.Debug("[changeBrightness] will update to:", monitor.Name, br)
		//	err := m.doSetBrightnessFake(br, monitor.Name)
		//	if err != nil {
		//		return err
		//	}
		//}
	}

	m.saveBrightness()
	return nil
}

func (m *Manager) getSavedBrightnessTable() (map[string]float64, error) {
	value := m.settings.GetString(gsKeyBrightness)
	if value == "" {
		return nil, nil
	}
	var result map[string]float64
	err := json.Unmarshal([]byte(value), &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (m *Manager) initBrightness() {
	brightnessTable, err := m.getSavedBrightnessTable()
	if err != nil {
		logger.Warning(err)
	}
	if brightnessTable == nil {
		brightnessTable = make(map[string]float64)
	}

	monitors := m.getConnectedMonitors()
	for _, monitor := range monitors {
		if _, ok := brightnessTable[monitor.Name]; ok {
			continue
		}
		brightnessTable[monitor.Name] = 1
	}
	m.Brightness = brightnessTable
}

func (m *Manager) getBrightnessSetter() string {
	// NOTE: 特殊处理龙芯笔记本亮度设置问题
	blDir := "/sys/class/backlight/loongson"
	_, err := os.Stat(blDir)
	if err == nil {
		_, err := os.Stat(filepath.Join(blDir, "device/edid"))
		if err != nil {
			return "backlight"
		}
	}

	return m.settings.GetString(gsKeySetter)
}

func (m *Manager) isBuiltinMonitor(monitor *Monitor) bool {
	m.builtinMonitorMu.Lock()
	defer m.builtinMonitorMu.Unlock()
	return m.builtinMonitor == monitor
}

func (m *Manager) setMonitorBrightness(monitor *Monitor, value float64) error {
	isBuiltin := m.isBuiltinMonitor(monitor)
	err := brightness.Set(value, m.getBrightnessSetter(), isBuiltin,
		monitor.ID, m.xConn)
	return err
}

func (m *Manager) doSetBrightnessAux(fake bool, value float64, name string) error {
	monitors := m.getConnectedMonitors()
	monitor0 := monitors.GetByName(name)
	if monitor0 == nil {
		return InvalidOutputNameError{Name: name}
	}

	monitor0.PropsMu.RLock()
	enabled := monitor0.Enabled
	monitor0.PropsMu.RUnlock()

	value = round(value)
	if !fake && enabled {
		err := m.setMonitorBrightness(monitor0, value)
		if err != nil {
			logger.Warningf("failed to set brightness for %s: %v", name, err)
			return err
		}
	}

	oldValue := m.Brightness[name]
	if oldValue == value {
		return nil
	}

	// update brightness of the output
	m.Brightness[name] = value
	err := m.emitPropChangedBrightness(m.Brightness)
	if err != nil {
		logger.Warning(err)
	}

	return nil
}

func (m *Manager) doSetBrightness(value float64, name string) error {
	return m.doSetBrightnessAux(false, value, name)
}

func (m *Manager) doSetBrightnessFake(value float64, name string) error {
	return m.doSetBrightnessAux(true, value, name)
}
