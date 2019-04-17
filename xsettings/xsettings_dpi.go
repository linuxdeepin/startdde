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

package xsettings

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"pkg.deepin.io/lib/utils"
)

const (
	DPI_FALLBACK = 96
	HIDPI_LIMIT  = DPI_FALLBACK * 2

	ffKeyPixels = `user_pref("layout.css.devPixelsPerPx",`
)

// TODO: update 'antialias, hinting, hintstyle, rgba, cursor-theme, cursor-size'
func (m *XSManager) updateDPI() {
	scale := m.gs.GetDouble(gsKeyScaleFactor)
	if scale <= 0 {
		scale = 1
	}

	var infos []xsSetting
	scaledDPI := int32(float64(DPI_FALLBACK*1024) * scale)
	if scaledDPI != m.gs.GetInt("xft-dpi") {
		m.gs.SetInt("xft-dpi", scaledDPI)
		infos = append(infos, xsSetting{
			sType: settingTypeInteger,
			prop:  "Xft/DPI",
			value: scaledDPI,
		})
	}

	// update window scale and cursor size
	windowScale := m.gs.GetInt(gsKeyWindowScale)
	if windowScale > 1 {
		scaledDPI = int32(DPI_FALLBACK * 1024)
	}
	cursorSize := m.gs.GetInt(gsKeyGtkCursorThemeSize)
	v, _ := m.GetInteger("Gdk/WindowScalingFactor")
	if v != windowScale {
		infos = append(infos, xsSetting{
			sType: settingTypeInteger,
			prop:  "Gdk/WindowScalingFactor",
			value: windowScale,
		}, xsSetting{
			sType: settingTypeInteger,
			prop:  "Gdk/UnscaledDPI",
			value: scaledDPI,
		}, xsSetting{
			sType: settingTypeInteger,
			prop:  "Gtk/CursorThemeSize",
			value: cursorSize,
		})
	}

	if len(infos) != 0 {
		err := m.setSettings(infos)
		if err != nil {
			logger.Warning("Failed to update dpi:", err)
		}
		m.updateXResources()
	}
}

func (m *XSManager) updateXResources() {
	scaleFactor := m.gs.GetDouble(gsKeyScaleFactor)
	xftDpi := int(DPI_FALLBACK * scaleFactor)
	updateXResources(xresourceInfos{
		&xresourceInfo{
			key:   "Xcursor.theme",
			value: m.gs.GetString("gtk-cursor-theme-name"),
		},
		&xresourceInfo{
			key:   "Xcursor.size",
			value: fmt.Sprintf("%d", m.gs.GetInt(gsKeyGtkCursorThemeSize)),
		},
		&xresourceInfo{
			key:   "Xft.dpi",
			value: strconv.Itoa(xftDpi),
		},
	})
}

var ffDir = path.Join(os.Getenv("HOME"), ".mozilla/firefox")

func (m *XSManager) updateFirefoxDPI() {
	scale := m.gs.GetDouble(gsKeyScaleFactor)
	if scale <= 0 {
		// firefox default value: -1
		scale = -1
	}

	configs, err := getFirefoxConfigs(ffDir)
	if err != nil {
		logger.Debug("Failed to get firefox configs:", err)
		return
	}

	for _, config := range configs {
		err = setFirefoxDPI(scale, config, config)
		if err != nil {
			logger.Warning("Failed to set firefox dpi:", config, err)
		}
	}
}

func getFirefoxConfigs(dir string) ([]string, error) {
	finfos, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var configs []string
	for _, finfo := range finfos {
		config := path.Join(dir, finfo.Name(), "prefs.js")
		if !utils.IsFileExist(config) {
			continue
		}
		configs = append(configs, config)
	}
	return configs, nil
}

func setFirefoxDPI(value float64, src, dest string) error {
	contents, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	lines := strings.Split(string(contents), "\n")
	target := fmt.Sprintf("%s \"%.2f\");", ffKeyPixels, value)
	found := false
	for i, line := range lines {
		if line == "" || line[0] == '#' {
			continue
		}
		if !strings.Contains(line, ffKeyPixels) {
			continue
		}

		if line == target {
			return nil
		}

		tmp := strings.Split(ffKeyPixels, ",")[0] + ", " +
			fmt.Sprintf("\"%.2f\");", value)
		lines[i] = tmp
		found = true
		break
	}
	if !found {
		if value == -1 {
			return nil
		}
		tmp := lines[len(lines)-1]
		lines[len(lines)-1] = target
		lines = append(lines, tmp)
	}
	return ioutil.WriteFile(dest, []byte(strings.Join(lines, "\n")), 0644)
}
