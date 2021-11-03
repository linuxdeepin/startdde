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
	"os"
	"path/filepath"
	"strings"

	"pkg.deepin.io/dde/startdde/display/brightness"
)

type InvalidOutputNameError struct {
	Name string
}

func (err InvalidOutputNameError) Error() string {
	return fmt.Sprintf("invalid output name %q", err.Name)
}

func (m *Manager) saveBrightnessInCfg(valueMap map[string]float64) error {
	if len(valueMap) == 0 {
		return nil
	}
	changed := false
	m.modifySuitableSysMonitorConfigs(func(configs SysMonitorConfigs) SysMonitorConfigs {
		for _, config := range configs {
			v, ok := valueMap[config.Name]
			if ok {
				config.Brightness = v
				changed = true
			}
		}
		return configs
	})

	if !changed {
		return nil
	}

	err := m.saveSysConfig()
	return err
}

func (m *Manager) changeBrightness(raised bool) error {
	var step = 0.05
	if m.MaxBacklightBrightness < 100 && m.MaxBacklightBrightness != 0 {
		step = 1 / float64(m.MaxBacklightBrightness)
	}
	if !raised {
		step = -step
	}

	monitors := m.getConnectedMonitors()

	successMap := make(map[string]float64)
	for _, monitor := range monitors {
		v, ok := m.Brightness[monitor.Name]
		if !ok {
			v = 1.0
		}

		var br float64
		br = v + step
		if br > 1.0 {
			br = 1.0
		}
		if br < 0.0 {
			br = 0.0
		}
		logger.Debug("[changeBrightness] will set to:", monitor.Name, br)
		err := m.setBrightnessAndSync(monitor.Name, br)
		if err != nil {
			logger.Warning(err)
			continue
		}
		successMap[monitor.Name] = br
	}
	err := m.saveBrightnessInCfg(successMap)
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

// TODO 废弃
//func (m *Manager) initBrightness() {
//brightnessTable, err := m.getSavedBrightnessTable()
//if err != nil {
//	logger.Warning(err)
//}
//if brightnessTable == nil {
//	brightnessTable = make(map[string]float64)
//}
//
//monitors := m.getConnectedMonitors()
//for _, monitor := range monitors {
//	if _, ok := brightnessTable[monitor.Name]; ok {
//		continue
//	}
//	brightnessTable[monitor.Name] = 1
//}
// 在 applyDisplayConfig 中会设置亮度
//m.Brightness = make(map[string]float64)
//}

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

// see also: gnome-desktop/libgnome-desktop/gnome-rr.c
//           '_gnome_rr_output_name_is_builtin_display'
func (m *Manager) isBuiltinMonitor(name string) bool {
	name = strings.ToLower(name)
	switch {
	case strings.HasPrefix(name, "vga"):
		return false
	case strings.HasPrefix(name, "hdmi"):
		return false

	case strings.HasPrefix(name, "dvi"):
		return true
	case strings.HasPrefix(name, "lvds"):
		// Most drivers use an "LVDS" prefix
		return true
	case strings.HasPrefix(name, "lcd"):
		// fglrx uses "LCD" in some versions
		return true
	case strings.HasPrefix(name, "edp"):
		// eDP is for internal built-in panel connections
		return true
	case strings.HasPrefix(name, "dsi"):
		return true
	case name == "default":
		return true
	}
	return false
}

func (m *Manager) setMonitorBrightness(monitor *Monitor, value float64) error {
	isBuiltin := m.isBuiltinMonitor(monitor.Name)
	err := brightness.Set(value, m.getBrightnessSetter(), isBuiltin,
		monitor.ID, m.xConn)
	return err
}

func (m *Manager) setBrightnessAux(fake bool, name string, value float64) error {
	monitors := m.getConnectedMonitors()
	monitor := monitors.GetByName(name)
	if monitor == nil {
		return InvalidOutputNameError{Name: name}
	}

	monitor.PropsMu.RLock()
	enabled := monitor.Enabled
	monitor.PropsMu.RUnlock()

	value = math.Round(value*1000) / 1000 // 通过该方法，用来对亮度值(亮度值范围为0-1)四舍五入保留小数点后三位有效数字
	if !fake && enabled {
		err := m.setMonitorBrightness(monitor, value)
		if err != nil {
			logger.Warningf("failed to set brightness for %s: %v", name, err)
			return err
		}
	}

	monitor.setPropBrightnessWithLock(value)

	return nil
}

func (m *Manager) setBrightness(name string, value float64) error {
	return m.setBrightnessAux(false, name, value)
}

func (m *Manager) setBrightnessAndSync(name string, value float64) error {
	err := m.setBrightness(name, value)
	if err == nil {
		m.syncPropBrightness()
	}
	return err
}
