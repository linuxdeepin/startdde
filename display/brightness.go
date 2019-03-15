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

func (dpy *Manager) saveBrightness() {
	dpy.brightnessMutex.RLock()
	jsonStr := jsonMarshal(dpy.Brightness)
	dpy.brightnessMutex.RUnlock()
	dpy.setting.SetString(gsKeyBrightness, jsonStr)
}

func (dpy *Manager) ChangeBrightness(raised bool) {
	var step float64 = 0.05
	if !raised {
		step = -step
	}

	for _, info := range dpy.outputInfos {
		dpy.brightnessMutex.RLock()
		v, ok := dpy.Brightness[info.Name]
		dpy.brightnessMutex.RUnlock()
		if !ok {
			v = 1.0
		}

		var br float64
		setBr := true

		if blCtrl, err := brightness.GetBacklightController(info.Id, dpy.conn); err != nil {
			logger.Debugf("get output %q backlight controller failed: %v", info.Name, err)
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
			logger.Debug("[changeBrightness] will set to:", info.Name, br)
			dpy.brightnessMutex.Lock()
			dpy.doSetBrightness(br, info.Name)
			dpy.brightnessMutex.Unlock()
		} else {
			logger.Debug("[changeBrightness] will update to:", info.Name, br)
			dpy.brightnessMutex.Lock()
			dpy.doSetBrightnessFake(br, info.Name)
			dpy.brightnessMutex.Unlock()
		}
	}

	dpy.saveBrightness()
}

func (dpy *Manager) getSavedBrightnessTable() (map[string]float64, error) {
	value := dpy.setting.GetString(gsKeyBrightness)
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

func (dpy *Manager) initBrightness() {
	brightnessTable, err := dpy.getSavedBrightnessTable()
	if err != nil {
		logger.Warning(err)
	}

	for _, info := range dpy.outputInfos {
		if _, ok := brightnessTable[info.Name]; ok {
			continue
		}

		if brightnessTable == nil {
			brightnessTable = make(map[string]float64)
		}
		brightnessTable[info.Name] = 1
	}

	for name, value := range brightnessTable {
		err = dpy.doSetBrightness(value, name)
		if err != nil {
			logger.Warning(err)
		}
	}
}

func (dpy *Manager) doSetBrightnessAux(fake bool, value float64, name string) error {
	info := dpy.outputInfos.QueryByName(name)
	if info.Name == "" {
		return InvalidOutputNameError{name}
	}

	if !fake {
		isBuiltin := isBuiltinOutput(name)
		err := brightness.Set(value, dpy.setting.GetString(gsKeySetter), isBuiltin,
			info.Id, dpy.conn)
		if err != nil {
			logger.Warningf("failed to set brightness to %v for %s: %v", value, name, err)
			return err
		}
	}

	oldValue := dpy.Brightness[name]
	if oldValue == value {
		return nil
	}

	// update brightness of the output
	dpy.Brightness[name] = value
	dpy.notifyBrightnessChange()
	return nil
}

func (dpy *Manager) doSetBrightness(value float64, name string) error {
	return dpy.doSetBrightnessAux(false, value, name)
}

func (dpy *Manager) doSetBrightnessFake(value float64, name string) error {
	return dpy.doSetBrightnessAux(true, value, name)
}
