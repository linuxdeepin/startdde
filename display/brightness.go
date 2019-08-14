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
	"math"

	"pkg.deepin.io/dde/startdde/display/brightness"
)

type InvalidOutputNameError struct {
	Name string
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
			if br < 0.0 {
				br = 0.0
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
	m.Brightness = make(map[string]float64)
	brightnessTable, err := m.getSavedBrightnessTable()
	if err != nil {
		logger.Warning(err)
	}

	monitors := m.getConnectedMonitors()
	for _, monitor := range monitors {
		if _, ok := brightnessTable[monitor.Name]; ok {
			continue
		}

		if brightnessTable == nil {
			brightnessTable = make(map[string]float64)
		}
		brightnessTable[monitor.Name] = 1
	}

	brightness.InitBacklightHelper()

	for name, value := range brightnessTable {
		err = m.doSetBrightness(value, name)
		if err != nil {
			logger.Warning(err)
		}
	}
}

func (m *Manager) doSetBrightnessAux(fake bool, value float64, name string) error {
	monitors := m.getConnectedMonitors()
	// TODO
	var monitor0 *Monitor
	for _, monitor := range monitors {
		if monitor.Name == name {
			monitor0 = monitor
			break
		}
	}
	if monitor0 == nil {
		return InvalidOutputNameError{name}
	}

	if !fake {
		isBuiltin := isBuiltinOutput(name)
		err := brightness.Set(value, m.settings.GetString(gsKeySetter), isBuiltin,
			monitor0.ID, m.xConn)
		if err != nil {
			logger.Warningf("failed to set brightness to %v for %s: %v", value, name, err)
			return err
		}
	}

	oldValue := m.Brightness[name]
	if oldValue == value {
		return nil
	}

	// update brightness of the output
	m.Brightness[name] = value
	m.emitPropChangedBrightness(m.Brightness)
	return nil
}

func (m *Manager) doSetBrightness(value float64, name string) error {
	return m.doSetBrightnessAux(false, value, name)
}

func (m *Manager) doSetBrightnessFake(value float64, name string) error {
	return m.doSetBrightnessAux(true, value, name)
}
