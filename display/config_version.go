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
	"io/ioutil"
	"os"
	"path"
)

const _confVersion = "3.3"

var (
	confVersionFile = os.Getenv("HOME") + "/.config/deepin/startdde/config.version"
)

func (dpy *Manager) checkConfigVersion() {
	if isVersionRight(_confVersion, confVersionFile) {
		return
	}

	logger.Debug("Config version not same, will delete config && write version file")
	dpy.config = &configManager{
		BaseGroup: make(map[string]*configMonitor),
		filename:  configFile,
	}

	err := os.Remove(configFile)
	if err != nil {
		logger.Warning("Failed to delete config:", err)
	}

	err = os.MkdirAll(path.Dir(confVersionFile), 0755)
	if err != nil {
		logger.Warning("Failed to mkdir:", err)
		return
	}
	err = ioutil.WriteFile(confVersionFile, []byte(_confVersion), 0644)
	if err != nil {
		logger.Warning("Failed to write version file:", err)
	}
	dpy.setPropDisplayMode(DisplayModeExtend)
}

func isVersionRight(version, file string) bool {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return false
	}

	return string(data) == version
}
