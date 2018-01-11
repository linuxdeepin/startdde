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
	"pkg.deepin.io/dde/startdde/memchecker"
	"pkg.deepin.io/lib/dbus"
	"strings"
	"sync"
	"time"
)

const (
	defaultNeededMem = 50 * 1024 // 50M
)

var (
	_memTicker     *time.Ticker
	_tickerStopped chan struct{}

	_curNeededMem = uint64(0)
)

// IsMemSufficient check the available memory whether sufficient
func (m *StartManager) IsMemSufficient() bool {
	return memchecker.IsSufficient()
}

// TryAgain launch the action which blocked with the memory insufficient
func (m *StartManager) TryAgain(launch bool) error {
	action := getCurAction()
	logger.Info("------Try Action:", action, launch)
	setCurAction("")
	stopMemTicker()
	if !launch || action == "" {
		return nil
	}

	return handleCurAction(action)
}

func (m *StartManager) setPropNeededMemory(v uint64) {
	if m.NeededMemory == v {
		return
	}
	m.NeededMemory = v
	dbus.NotifyChange(m, "NeededMemory")
}

func handleMemInsufficient(v string) error {
	if memchecker.IsSufficient() {
		return nil
	}

	action := getCurAction()
	if action != "" {
		logger.Info("The prev action is executing:", action)
		showWarningDialog(getActionName(action))
		return fmt.Errorf("The prev action(%s) is executing", action)
	}

	logger.Info("Notice: current memory insufficient, please free.....")
	// TODO: get needed memory
	_curNeededMem = defaultNeededMem
	updateNeededMemory()
	go startMemTicker()
	showWarningDialog(v)
	return fmt.Errorf("Memory has insufficient, please free")
}

func startMemTicker() {
	_memTicker = time.NewTicker(time.Second * 1)
	_tickerStopped = make(chan struct{})
	logger.Info("------------Start memory ticker")
	for {
		select {
		case <-_tickerStopped:
			logger.Info("----------Ticker has stopped")
			START_MANAGER.setPropNeededMemory(0)
			return
		case <-_memTicker.C:
			updateNeededMemory()
		}
	}
}

func updateNeededMemory() {
	info, err := memchecker.GetMemInfo()
	if err != nil {
		logger.Info("-------Failed to get memory info:", err)
		return
	}
	logger.Info("------------Memory info:", info.MemAvailable, info.MinAvailable)
	v := int64(_curNeededMem) + int64(info.MinAvailable) - int64(info.MemAvailable)
	logger.Info("------------Update needed memory:", START_MANAGER.NeededMemory, v)
	if v < 0 {
		v = 0
	}

	// available sufficient, check swap used
	s := int64(info.SwapTotal) - int64(info.SwapFree) - int64(info.MaxSwapUsed)
	logger.Info("-------Swap info:", info.SwapTotal, info.SwapFree, s)
	if s < 0 {
		s = 0
	}
	v += s

	if uint64(v) == START_MANAGER.NeededMemory {
		return
	}
	START_MANAGER.setPropNeededMemory(uint64(v))
}

func stopMemTicker() {
	if _memTicker == nil {
		return
	}
	_memTicker.Stop()
	_memTicker = nil

	if _tickerStopped != nil {
		close(_tickerStopped)
	}
	_tickerStopped = nil
}

func showWarningDialog(action string) {
	conn, err := dbus.SessionBus()
	if err != nil {
		logger.Warning("Failed to get session bus:", err)
		return
	}

	dialog := conn.Object("com.deepin.dde.MemoryWarningDialog",
		"/com/deepin/dde/MemoryWarningDialog")
	err = dialog.Call("com.deepin.dde.MemoryWarningDialog.Show",
		0, action).Store()
	if err != nil {
		logger.Warning("Failed to show memory warning dialog:", err)
	}
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

func getActionName(action string) string {
	switch action {
	case "LaunchApp":
		return _app.desktop
	case "LaunchAppAction":
		return _appAction.desktop
	case "RunCommand":
		var _name = _cmd.exe
		if len(_cmd.args) != 0 {
			_name += " " + strings.Join(_cmd.args, " ")
		}
		return _name
	}
	return ""
}

func handleCurAction(action string) error {
	if action == "" {
		return nil
	}

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
	return err
}
