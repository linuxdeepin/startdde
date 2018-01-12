/*
 * Copyright (C) 2016 ~ 2017 Deepin Technology Co., Ltd.
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

package watchdog

import (
	"os"
	"pkg.deepin.io/lib/log"
	"strconv"
)

const (
	envMaxLaunchTimes = "DDE_WATCHDOG_MAX_LAUNCH_TIMES"
)

var (
	logger   = log.NewLogger("daemon/watchdog")
	_manager *Manager
	// if times == 0, unlimit
	maxLaunchTimes = 10
)

func Start() {
	if _manager != nil {
		return
	}

	logger.BeginTracing()
	times := os.Getenv(envMaxLaunchTimes)
	if len(times) != 0 {
		v, err := strconv.ParseInt(times, 10, 64)
		if err == nil {
			maxLaunchTimes = int(v)
		}
	}
	logger.Debug("[WATCHDOG] max launch times:", maxLaunchTimes)
	_manager = newManager()
	_manager.AddTask(newDockTask())
	_manager.AddTask(newDesktopTask())
	_manager.AddTask(newDDEPolkitAgent())
	_manager.AddTask(newWMTask())
	go _manager.StartLoop()
	return
}

func Stop() {
	if _manager == nil {
		return
	}

	_manager.QuitLoop()
	_manager = nil
	destroyDBusDaemon()
	logger.EndTracing()
	return
}

func SetLogLevel(level log.Priority) {
	logger.SetLogLevel(level)
}
