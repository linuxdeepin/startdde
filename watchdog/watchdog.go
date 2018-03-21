/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
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
	"strconv"
	"time"

	"pkg.deepin.io/lib/dbus"
	"pkg.deepin.io/lib/log"
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

	err := initDBusObject()
	if err != nil {
		logger.Warning(err)
		return
	}

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

	_manager.taskMap = map[string]*taskInfo{
		wmDest: newWMTask(),
	}
	_manager.listenNameOwnerChanged()
	return
}

func (m *Manager) listenNameOwnerChanged() error {
	bus, err := dbus.SessionBus()
	if err != nil {
		return err
	}
	signalChan := bus.Signal()

	rule := "type='signal',interface='org.freedesktop.DBus',member='NameOwnerChanged'"
	err = bus.BusObject().Call(orgFreedesktopDBus+".AddMatch", 0, rule).Err
	if err != nil {
		return err
	}

	go func() {
		for signal := range signalChan {
			// log.Printf("signal: %#v\n", signal)
			if signal.Name == orgFreedesktopDBus+".NameOwnerChanged" &&
				signal.Path == "/org/freedesktop/DBus" && len(signal.Body) == 3 {
				name, ok := signal.Body[0].(string)
				if !ok {
					continue
				}
				if name == "" || name[0] == ':' {
					continue
				}

				oldOwner, ok := signal.Body[1].(string)
				if !ok {
					continue
				}

				newOwner, ok := signal.Body[2].(string)
				if !ok {
					continue
				}

				if oldOwner != "" && newOwner == "" {
					logger.Debugf("name lost %q", name)
				}
				taskInfo := m.taskMap[name]
				if taskInfo == nil {
					continue
				}

				go func() {
					time.Sleep(3 * time.Second)
					taskInfo.Launch()
				}()
			}
		}

	}()
	return nil
}

func Stop() {
	if _manager == nil {
		return
	}

	_manager.QuitLoop()
	_manager = nil
	return
}

func SetLogLevel(level log.Priority) {
	logger.SetLogLevel(level)
}
