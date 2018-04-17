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
	"errors"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xevent"
	"github.com/BurntSushi/xgbutil/xprop"
	"github.com/BurntSushi/xgbutil/xwindow"
	"pkg.deepin.io/lib/dbus"
	"pkg.deepin.io/lib/log"
)

const (
	swDBusDest = "com.deepin.WMSwitcher"
	swDBusPath = "/com/deepin/WMSwitcher"
	swDBusIFC  = swDBusDest

	deepin3DWM = "deepin-wm"
	deepin2DWM = "deepin-metacity"
	unknownWM  = "unknown"

	osdSwitch2DWM    = "SwitchWM2D"
	osdSwitch3DWM    = "SwitchWM3D"
	osdSwitchWMError = "SwitchWMError"
)

var wmNameMap = map[string]string{
	deepin3DWM: "deepin wm",
	deepin2DWM: "deepin metacity",
}

//Switcher wm switch manager
type Switcher struct {
	goodWM bool
	logger *log.Logger
	info   *configInfo
	mu     sync.Mutex

	currentWM string
	WMChanged func(string)
}

func (s *Switcher) setCurrentWM(name string) {
	var changed bool
	s.mu.Lock()
	if s.currentWM != name {
		s.currentWM = name
		s.emitSignalWMChanged(name)
		changed = true
	}
	s.mu.Unlock()

	if changed {
		// show osd
		go func() {
			time.Sleep(1 * time.Second)
			switch name {
			case deepin3DWM:
				showOSD(osdSwitch3DWM)
			case deepin2DWM:
				showOSD(osdSwitch2DWM)
			}
		}()
	}
}

// CurrentWM show the current window manager
func (s *Switcher) CurrentWM() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	wmName := wmNameMap[s.currentWM]
	if wmName == "" {
		return "unknown"
	}
	return wmName
}

// RestartWM restart the last window manager
func (s *Switcher) RestartWM() error {
	return s.runWM(s.info.LastWM, true)
}

// Start2DWM called by startdde watchdog, run 2d window manager without --replace option.
func (s *Switcher) Start2DWM() error {
	err := s.runWM(deepin2DWM, false)
	if err != nil {
		return err
	}

	// NOTE: Don't save to config file.
	s.setLastWM(deepin2DWM)
	return nil
}

// RequestSwitchWM try to switch window manager
func (s *Switcher) RequestSwitchWM() error {
	if !s.goodWM {
		showOSD(osdSwitchWMError)
		return errors.New("refused to switch wm")
	}
	s.mu.Lock()
	currentWM := s.currentWM
	s.mu.Unlock()

	nextWM := deepin3DWM
	if currentWM == deepin3DWM {
		nextWM = deepin2DWM
	} else if currentWM == deepin2DWM {
		nextWM = deepin3DWM
	}

	err := s.runWM(nextWM, true)
	if err != nil {
		return err
	}

	s.setLastWM(nextWM)
	s.saveConfig()
	s.initSogou()
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

func (s *Switcher) emitSignalWMChanged(wm string) {
	dbus.Emit(s, "WMChanged", wmNameMap[wm])
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
	if err != nil || s.info.LastWM == "" {
		s.logger.Warning("Failed to load config:", err)
		s.initConfig()
		return
	}

	s.goodWM = s.info.AllowSwitch
}

func (s *Switcher) listenWMChanged() {
	s.currentWM = s.info.LastWM

	xu, err := xgbutil.NewConn()
	if err != nil {
		s.logger.Warning(err)
	}

	root := xu.RootWin()
	xwindow.New(xu, root).Listen(xproto.EventMaskPropertyChange)

	atomNetSupportingWMCheck, err := xprop.Atm(xu, "_NET_SUPPORTING_WM_CHECK")
	if err != nil {
		s.logger.Warning(err)
		return
	}

	xevent.PropertyNotifyFun(
		func(X *xgbutil.XUtil, e xevent.PropertyNotifyEvent) {
			if e.Atom != atomNetSupportingWMCheck {
				return
			}

			switch e.State {
			case xproto.PropertyNewValue:
				win, err := xprop.PropValWindow(xprop.GetProperty(xu, root,
					"_NET_SUPPORTING_WM_CHECK"))
				if err != nil {
					s.logger.Warning(err)
					return
				}
				s.logger.Debug("win:", win)
				wmName, err := xprop.PropValStr(xprop.GetProperty(xu, win, "_NET_WM_NAME"))
				if err != nil {
					s.logger.Warning(err)
					return
				}
				s.logger.Debug("wmName:", wmName)

				var currentWM string
				if wmName == "Metacity" {
					currentWM = deepin2DWM
				} else if strings.Contains(wmName, "DeepinGala") {
					currentWM = deepin3DWM
				} else {
					currentWM = unknownWM
				}
				s.setCurrentWM(currentWM)

			case xproto.PropertyDelete:
				s.logger.Debug("wm lost")
			}

		}).Connect(xu, root)
	go xevent.Main(xu)
}

func (s *Switcher) initSogou() {
	filename := getSogouConfigPath()
	v, _ := getSogouSkin(filename)
	if v != "" && s.goodWM && s.info.LastWM == deepin3DWM {
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
			LastWM:      deepin3DWM,
		}
	} else {
		s.info = &configInfo{
			AllowSwitch: s.goodWM,
			LastWM:      deepin2DWM,
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

func (s *Switcher) runWM(wm string, replace bool) error {
	conn, err := dbus.SessionBus()
	if err != nil {
		return err
	}

	args := []string{"GDK_SCALE=1", wm}
	if replace {
		args = append(args, "--replace")
	}
	obj := conn.Object("com.deepin.SessionManager", "/com/deepin/StartManager")
	err = obj.Call("com.deepin.StartManager.RunCommand", 0,
		"env", args).Store()
	if err != nil {
		return err
	}
	return nil
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
