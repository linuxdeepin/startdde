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

package wm

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"pkg.deepin.io/lib/utils"
	"pkg.deepin.io/lib/xdg/basedir"
)

type configInfo struct {
	AllowSwitch bool   `json:"allow_switch"`
	LastWM      string `json:"last_wm"`
}

const (
	swSystemPath = "/etc/deepin-wm-switcher/config.json"
	swUserPath   = "deepin/deepin-wm-switcher/config.json"
)

func (s *Switcher) loadConfig() (*configInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	file := filepath.Join(basedir.GetUserConfigDir(), swUserPath)
	if !utils.IsFileExist(file) {
		file = swSystemPath
	}

	if !utils.IsFileExist(file) {
		return nil, fmt.Errorf("Failed to found config: %s", file)
	}
	return doLoadSwConfig(file)
}

func (s *Switcher) setAllowSwitch(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.info.AllowSwitch == v {
		return
	}
	s.info.AllowSwitch = v
}

func (s *Switcher) setLastWM(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.info.LastWM == v {
		return
	}
	s.info.LastWM = v
}

func (s *Switcher) saveConfig() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	file := filepath.Join(basedir.GetUserConfigDir(), swUserPath)
	data, err := json.Marshal(s.info)
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(file), 0755)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(file, data, 0644)
}

func doLoadSwConfig(file string) (*configInfo, error) {
	contents, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var info configInfo
	// fix no 'allow_switch' in config
	info.AllowSwitch = true
	err = json.Unmarshal(contents, &info)
	if err != nil {
		return nil, err
	}
	return &info, nil
}
