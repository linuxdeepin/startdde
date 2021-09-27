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
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"pkg.deepin.io/dde/startdde/wl_display/brightness"
)

type InvalidOutputNameError struct {
	Name string
}

func (err InvalidOutputNameError) Error() string {
	return fmt.Sprintf("invalid output name %q", err.Name)
}

func (m *Manager) saveBrightness() {
	if m.settings == nil || m.settings.C == nil || m.settings.Object.C == nil {
		logger.Info("[saveBrightness] object invalid, not save...")
		return
	}

	jsonStr := jsonMarshal(m.Brightness)
	logger.Info("[saveBrightness] gsettings set:", m.settings, gsKeyBrightness, jsonStr)
	m.settings.SetString(gsKeyBrightness, jsonStr)
}

func (m *Manager) changeBrightness(raised bool) error {
	// TODO
	// return errors.New("TODO")
	if m.xConn == nil {
		logger.Warning("No xorg connection")
		return nil
	}

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
		setBr := true

		if blCtrl, err := brightness.GetBacklightController(monitor.ID, m.xConn); err != nil {
			logger.Debugf("get output %q backlight controller failed: %v", monitor.Name, err)
		} else {
			max := blCtrl.MaxBrightness
			cur, err := blCtrl.GetBrightness()
			if err == nil {
				// TODO: Some drivers will also set the brightness when the brightness up/down key is pressed
				hv := float64(cur) / float64(max)
				avg := (v + hv) / 2
				delta := (v - hv) / avg
				logger.Debugf("v: %g, hv: %g, avg: %g delta: %g", v, hv, avg, delta)

				if math.Abs(delta) > 0.05 {
					logger.Debug("backlight actual brightness is not set")
					setBr = false
					br = hv
				}
			}
		}

		if setBr {
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
		} else {
			logger.Debug("[changeBrightness] will update to:", monitor.Name, br)
			err := m.doSetBrightnessFake(br, monitor.Name)
			if err != nil {
				return err
			}
		}
	}

	m.saveBrightness()
	return nil
}

// doSetBrightnessAuxForBacklight单独处理使用F1,F2调节亮度逻辑,避免发送多次dbus信号
func (m *Manager) doSetBrightnessAuxForBacklight(fake bool, value float64, name string, isRaised bool) error {
	monitors := m.getConnectedMonitors()
	monitor0 := monitors.GetByName(name)
	if monitor0 == nil {
		return InvalidOutputNameError{Name: name}
	}

	monitor0.PropsMu.RLock()
	enabled := monitor0.Enabled
	monitor0.PropsMu.RUnlock()

	var br float64
	if !fake && enabled {
		var step float64
		var times int
		if value < 0.4 {
			step = 0.001
			times = 50
		} else if value < 0.7 {
			step = 0.002
			times = 25
		} else if value < 0.9 {
			step = 0.005
			times = 10
		} else {
			step = 0.05
			times = 1
		}

		if !isRaised {
			step = -step
		}

		for i := 1; i <= times; i++ {
			br = value + step*float64(i)
			if br > 1.0 {
				br = 1.0
			}
			if br < 0.1 {
				br = 0.1
			}
			err := m.setMonitorBrightness(monitor0, br)
			if err != nil {
				logger.Warningf("brightness: failed to set brightness for %s: %v", name, err)
				return err
			}

			// 防止出现多次调节亮度值不变的情况
			if math.Abs(br-0.1) < 1e-5 || math.Abs(br-1.0) < 1e-5 {
				break
			}
		}
	}

	value = br
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

func (m *Manager) initBrightness() error {
	brightnessTable, err := m.getSavedBrightnessTable()
	if err != nil {
		logger.Warning(err)
	}
	if brightnessTable == nil {
		brightnessTable = make(map[string]float64)
	}

	var saved = false
	monitors := m.getConnectedMonitors()
	for _, monitor := range monitors {
		_, ok := brightnessTable[monitor.Name]
		if !ok {
			// add new monitor
			brightnessTable[monitor.Name] = 1
			saved = true
		}
	}

	var lightSet = false
	m.brightnessMapMu.Lock()
	m.Brightness = brightnessTable
	m.brightnessMapMu.Unlock()

	for name, v := range brightnessTable {
		// set the saved brightness
		if m.isConnected(name) {
			err = m.doSetBrightness(v, name)
			if err != nil {
				logger.Warning("Failed to set default brightness:", name, err)
				continue
			}
			if !lightSet {
				lightSet = true
			}
		}
	}

	if saved {
		logger.Info("Init default output brightness")
		// In huawei KelvinU sometimes crash because of GObject assert failure in GSettings
		m.saveBrightness()
	}

	if !lightSet {
		return errors.New("Init default output brightness failed!")
	}
	return nil
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

func (m *Manager) setMonitorBrightness(monitor *Monitor, value float64) error {
	isBuiltin := isBuiltinOutput(monitor.Name)
	logger.Debugf("brightness: setMonitorBrightness for %s, setter=%s, value=%.2f, edidBase64=%s", monitor.Name, m.getBrightnessSetter(), value, monitor.edid)
	err := brightness.Set(monitor.uuid, value, m.getBrightnessSetter(), isBuiltin, monitor.edid)
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

	if !fake && enabled {
		err := m.setMonitorBrightness(monitor0, value)
		if err != nil {
			logger.Warningf("failed to set brightness for %s: %v", name, err)
			return err
		}
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

func (m *Manager) isConnected(monitorName string) bool {
	monitors := m.getConnectedMonitors()
	for _, monitor := range monitors {
		if monitor.Name == monitorName {
			return true
		}
	}
	return false
}
