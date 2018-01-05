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

package main

import (
	"fmt"
	"os/exec"
	"pkg.deepin.io/dde/startdde/memchecker"
	"pkg.deepin.io/lib/dbus"
	"sync"
)

func startMemChecker() error {
	return memchecker.Start(logger)
}

func handleMemInsufficient() error {
	action := getCurAction()
	if action != "" {
		logger.Info("The prev action is executing:", action)
		return fmt.Errorf("The prev action(%s) is executing", action)
	}

	if !memchecker.GetMemChecker(logger).IsMemInsufficient() {
		return nil
	}
	logger.Info("Notice: current memory insufficient, please free.....")
	// TODO: launch interaction UI
	go func() {
		err := exec.Command("dmemory-warning-dialog").Run()
		if err != nil {
			logger.Warning("Failed to launch dmemory dialog:", err)
		}
	}()
	return fmt.Errorf("Memory has insufficient, please free")
}

var (
	_curAction    = ""
	_actionLocker sync.Mutex
	_app          = struct {
		desktop   string
		timestamp uint32
		files     []string
	}{}
	_appAction = struct {
		desktop   string
		action    string
		timestamp uint32
	}{}
	_cmd = struct {
		exe  string
		args []string
	}{}
)

func setCurAction(action string) {
	_actionLocker.Lock()
	_curAction = action
	_actionLocker.Unlock()
}

func getCurAction() string {
	_actionLocker.Lock()
	defer _actionLocker.Unlock()
	return _curAction
}

func handleCurAction(action string) {
	if action == "" {
		return
	}

	exec.Command("killall", "dmemory-warning-dialog").Run()
	var err error
	switch action {
	case "LaunchApp":
		err = START_MANAGER.LaunchApp(_app.desktop, _app.timestamp,
			_app.files)
	case "LaunchAppAction":
		err = START_MANAGER.LaunchAppAction(_appAction.desktop,
			_appAction.action, _appAction.timestamp)
	case "RunCommand":
		err = START_MANAGER.RunCommand(_cmd.exe, _cmd.args)
	}
	if err != nil {
		logger.Warning("Failed to launch action:", err)
	}
}

func listemMemChecker() {
	conn, err := dbus.SessionBus()
	if err != nil {
		logger.Error("Failed to connect session bus")
		return
	}

	ifc := "org.freedesktop.DBus.Properties"
	memberChanged := "PropertiesChanged"
	rule := "type=signal,sender=com.deepin.MemChecker,path=/com/deepin/MemChecker,interface=" + ifc + ",member=" + memberChanged
	conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule)

	sigChan := conn.Signal()
	for s := range sigChan {
		//logger.Info("Signal:", s.Name)
		if s.Name != ifc+"."+memberChanged {
			continue
		}
		if len(s.Body) != 3 {
			logger.Debug("Invalid signal body")
			continue
		}
		// Body[0] --> interface
		var set = make(map[string]dbus.Variant)
		set = s.Body[1].(map[string]dbus.Variant)
		v, ok := set["Insufficient"]
		if !ok {
			logger.Info("Memory cahnged. No prop found")
			continue
		}
		tmp := v.Value().(bool)
		logger.Info("Memory changed:", tmp)
		if !tmp {
			action := getCurAction()
			setCurAction("")
			handleCurAction(action)
		}
	}
}
