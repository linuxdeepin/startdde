/*
 * Copyright (C) 2014 ~ 2018 Deepin Technology Co., Ltd.
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

package memchecker

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"pkg.deepin.io/lib/xdg/basedir"
)

const (
	sysConfigFile = "/usr/share/startdde/memchecker.json"

	defaultMinMemAvail = 300  // 300M
	defaultMaxSwapUsed = 1200 // 1200M
)

type configInfo struct {
	MinMemAvail uint64 `json:"min-mem-available"`
	MaxSwapUsed uint64 `json:"max-swap-used"`
}

func loadConfig(filename string) (*configInfo, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		content, err = ioutil.ReadFile(sysConfigFile)
		if err != nil {
			return nil, err
		}
	}

	var info configInfo
	err = json.Unmarshal(content, &info)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func getConfigPath() string {
	return filepath.Join(basedir.GetUserConfigDir(),
		"deepin", "startdde", "memchecker.json")
}
