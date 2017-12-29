/*
 * Copyright (C) 2017 ~ 2017 Deepin Technology Co., Ltd.
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
	"fmt"
	"os/exec"
	"pkg.deepin.io/lib/dbus"
	"pkg.deepin.io/lib/log"
	"sync"
)

const (
	swDBusDest = "com.deepin.WMSwitcher"
	swDBusPath = "/com/deepin/WMSwitcher"
	swDBusIFC  = swDBusDest
)

var wmList = map[string]string{
	"deepin-wm":       "deepin wm",
	"deepin-metacity": "deepin metacity",
}

//Switcher wm switch manager
type Switcher struct {
	goodWM     bool
	logger     *log.Logger
	info       *configInfo
	infoLocker sync.Mutex

	WMChanged func(string)
}

//CurrentWM show the current window manager
func (s *Switcher) CurrentWM() string {
	if s.info.LastWM == "" {
		return "deepin wm"
	}
	return wmList[s.info.LastWM]
}

//RequestSwitchWM try to switch window manager
func (s *Switcher) RequestSwitchWM() error {
	if !s.goodWM {
		showOSD("SwitchWMError")
		return fmt.Errorf("Refused to switch wm")
	}
	nextWM := "deepin-wm"
	osd := "SwitchWM3D"
	if s.info.LastWM == "deepin-wm" {
		osd = "SwitchWM2D"
		nextWM = "deepin-metacity"
	} else if s.info.LastWM == "deepin-metacity" {
		nextWM = "deepin-wm"
	}

	curWM := s.info.LastWM
	err := s.doSwitchWM(nextWM)
	if err == nil {
		s.setLastWM(nextWM)
		s.saveConfig()
		dbus.Emit(s, "WMChanged", nextWM)
		showOSD(osd)
		s.initSogou()
		return nil
	}
	s.setLastWM(curWM)
	s.saveConfig()
	s.doSwitchWM(curWM)

	return nil
}

//GetDBusInfo return dbus object info
func (*Switcher) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		Dest:       swDBusDest,
		ObjectPath: swDBusPath,
		Interface:  swDBusIFC,
	}
}

func (s *Switcher) init() {
	// check card whether changed
	infos, err := loadCardConfig(getCardConfigPath())
	s.logger.Debugf("--------Card config: %#v, %v", infos, err)
	if err == nil {
		tmp, _ := getCardInfos()
		if tmp != nil && tmp.String() != infos.String() {
			// card changed, ignore wm config
			s.initConfig()
			s.initCard()
			return
		}
	} else {
		s.initCard()
	}

	s.info, err = s.loadConfig()
	s.logger.Debugf("--------wm config: %#v, %v", s.info, err)
	if err != nil {
		s.logger.Warning("Failed to load config:", err)
		s.initConfig()
		return
	}

	s.goodWM = s.info.AllowSwitch
}

func (s *Switcher) initSogou() {
	filename := getSogouConfigPath()
	v, _ := getSogouSkin(filename)
	if v != "" && s.goodWM && s.info.LastWM == "deepin-wm" {
		return
	}

	if v == sgDefaultSkin {
		return
	}

	err := setSogouSkin(sgDefaultSkin, filename)
	if err != nil {
		s.logger.Warning("Failed to set sogou skin:", err)
		return
	}

	outs, err := exec.Command("killall", "sogou-qimpanel").CombinedOutput()
	if err != nil {
		s.logger.Warning("Failed to terminate sogou:", string(outs), err)
	}
}

func (s *Switcher) initConfig() {
	s.goodWM = s.isGoodWM()
	if s.goodWM {
		s.info = &configInfo{
			AllowSwitch: s.goodWM,
			LastWM:      "deepin-wm",
		}
	} else {
		s.info = &configInfo{
			AllowSwitch: s.goodWM,
			LastWM:      "deepin-metacity",
		}
	}

	err := s.saveConfig()
	if err != nil {
		s.logger.Warning("Failed to save config:", err)
	}
}

func (s *Switcher) initCard() {
	infos, err := getCardInfos()
	if err != nil {
		s.logger.Warning("Failed to get card infos:", err)
		return
	}
	err = doSaveCardConfig(getCardConfigPath(), infos.genCardConfig())
	if err != nil {
		s.logger.Warning("Failed to save card info:", err)
	}
}

func (s *Switcher) doSwitchWM(wm string) error {
	err := exec.Command("env", "GDK_SCALE=1", wm, "--replace").Start()
	if err != nil {
		s.logger.Warning("Failed to switch wm:", err)
	}
	return err
}

func (s *Switcher) isGoodWM() bool {
	goodWM := true

	platform, err := getPlatform()
	if err == nil && platform == platformSW {
		if !isRadeonExists() {
			goodWM = false
			setupSWPlatform()
		}
	}

	if !isDriverLoadedCorrectly() {
		goodWM = false
		return goodWM
	}

	env, err := getVideoEnv()
	if err == nil {
		correctWMByEnv(env, &goodWM)
	}

	return goodWM
}
