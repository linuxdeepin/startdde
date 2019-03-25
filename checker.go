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
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"pkg.deepin.io/dde/startdde/memanalyzer"
	"pkg.deepin.io/dde/startdde/memchecker"
	"pkg.deepin.io/lib/dbus"
)

const (
	defaultNeededMem = 50 * 1024 // 50M

	envMemQueryWait = "DDE_MEM_QUERY_WAIT"
)

var (
	_memTicker     *time.Ticker
	_tickerStopped chan struct{}

	_curNeededMem = uint64(0)

	_memQueryWait = 15
)

func init() {
	s := os.Getenv(envMemQueryWait)
	if s == "" {
		return
	}

	v, _ := strconv.ParseInt(s, 10, 32)
	if v > 0 {
		_memQueryWait = int(v)
	}
}

// IsMemSufficient check the available memory whether sufficient
func (m *StartManager) IsMemSufficient() bool {
	if !globalGSettingsConfig.memcheckerEnabled {
		// memchecker disabled, always return true
		return true
	}

	return memchecker.IsSufficient()
}

// TryAgain launch the action which blocked with the memory insufficient
func (m *StartManager) TryAgain(launch bool) error {
	action := getCurAction()
	logger.Info("Try Action:", action, launch)
	setCurAction("")
	stopMemTicker()
	if !launch || action == "" {
		return nil
	}

	return handleCurAction(action)
}

// DumpMemRecord dump the process needed memory record
func (m *StartManager) DumpMemRecord() string {
	return memanalyzer.DumpDB()
}

func (m *StartManager) setPropNeededMemory(v uint64) {
	if m.NeededMemory == v {
		return
	}
	m.NeededMemory = v
	dbus.NotifyChange(m, "NeededMemory")
}

func handleMemInsufficient(v string) error {
	if !globalGSettingsConfig.memcheckerEnabled {
		return nil
	}
	if memchecker.IsSufficient() {
		return nil
	}

	action := getCurAction()
	if action != "" {
		logger.Info("The prev action is executing:", action)
		showWarningDialog(getActionName(action))
		return fmt.Errorf("the prev action(%s) is executing", action)
	}

	logger.Info("Notice: current memory insufficient, please free.....")
	_curNeededMem = getNeededMemory(v)
	updateNeededMemory()
	go startMemTicker()
	showWarningDialog(v)
	return fmt.Errorf("memory has insufficient, please free")
}

func startMemTicker() {
	_memTicker = time.NewTicker(time.Second * 1)
	_tickerStopped = make(chan struct{})
	logger.Info("Start memory ticker")
	for {
		select {
		case <-_tickerStopped:
			logger.Info("Ticker has stopped")
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
		logger.Warning("Failed to get memory info:", err)
		return
	}
	logger.Debug("Memory info:", _curNeededMem, info.MemAvailable, info.MinAvailable, info.MaxSwapUsed)
	v := int64(_curNeededMem) + int64(info.MinAvailable) - int64(info.MemAvailable)
	if v < 0 {
		v = 0
	}

	// available sufficient, check swap used
	if info.MaxSwapUsed != 0 {
		s := int64(info.SwapTotal) - int64(info.SwapFree) - int64(info.SwapCached) - int64(info.MaxSwapUsed)
		logger.Debug("Swap info:", info.SwapTotal, info.SwapFree, info.SwapCached, s)
		if s < 0 {
			s = 0
		}
		v += s
	}

	logger.Debug("Update needed memory:", START_MANAGER.NeededMemory, v)
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
		options   map[string]dbus.Variant
	}{}
	_appAction = struct {
		desktop   string
		action    string
		timestamp uint32
	}{}
	_cmd = struct {
		exe     string
		args    []string
		options map[string]dbus.Variant
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
		err = START_MANAGER.launchAppWithOptions(_app.desktop, _app.timestamp,
			_app.files, _app.options)
	case "LaunchAppAction":
		err = START_MANAGER.launchAppAction(_appAction.desktop,
			_appAction.action, _appAction.timestamp)
	case "RunCommand":
		err = START_MANAGER.runCommandWithOptions(_cmd.exe, _cmd.args, _cmd.options)
	}
	if err != nil {
		logger.Warning("Failed to launch action:", err)
	}
	return err
}

func getNeededMemory(name string) uint64 {
	v, err := memanalyzer.GetProcessMemory(name)
	logger.Info("[getNeededMemory] result:", name, v, err)
	if err != nil {
		return defaultNeededMem
	}
	return v
}

func saveNeededMemory(name, cgroupName string) error {
	tmp, _ := memanalyzer.GetProcessMemory(name)
	if tmp > 0 {
		logger.Debug("Process exists:", name, tmp)
		return nil
	}

	var (
		size uint64
		err  error
	)
	time.Sleep(time.Second * time.Duration(_memQueryWait))
	size, err = memanalyzer.GetCGroupMemory(cgroupName)
	logger.Info("process memory:", name, cgroupName, size, err)
	if err != nil || size == 0 {
		return err
	}

	if err != nil {
		return err
	}

	return memanalyzer.SaveProcessMemory(name, size)
}
