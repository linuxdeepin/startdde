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
	"io/ioutil"
	"os"
	"path/filepath"

	"pkg.deepin.io/lib/xdg/basedir"
)

const (
	userCfgPathSuffix = "deepin/deepin-wm-switcher/config.json"
)

type userConfig struct {
	LastWM string `json:"last_wm"`
	Wait   bool   `json:"wait"`
}

func (s *Switcher) loadUserConfig() error {
	filename := getUserConfigPath()
	userCfg, err := loadUserConfig(filename)
	if err != nil {
		// ignore not exist
		if !os.IsNotExist(err) {
			s.logger.Warning("failed to load user config:", err)
		}
	} else {
		s.userConfig = userCfg
		s.logger.Debugf("load user config: %#v", userCfg)
	}
	return err
}

func (s *Switcher) setLastWM(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.userConfig.LastWM == v {
		return
	}
	s.userConfig.LastWM = v
}

func getUserConfigPath() string {
	return filepath.Join(basedir.GetUserConfigDir(), userCfgPathSuffix)
}

func (s *Switcher) saveUserConfig() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Debugf("save user config: %#v", s.userConfig)
	filename := getUserConfigPath()
	err := saveUserConfig(filename, s.userConfig)
	if err != nil {
		s.logger.Warning("failed to save user config", err)
	}
}

func loadUserConfig(filename string) (*userConfig, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var v userConfig
	v.Wait = true
	err = json.Unmarshal(content, &v)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func saveUserConfig(filename string, v *userConfig) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(filename), 0755)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename, data, 0644)
}
