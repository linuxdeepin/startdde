// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package watchdog

import (
	"os"
	"strconv"
	"time"

	dbus "github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/go-lib/procfs"
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

func Start(getLockedFn func() bool, useKwin bool) {
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
	_manager.AddTimedTask(newDdeDesktopTask())
	_manager.AddTimedTask(newDdePolkitAgent())
	_manager.AddDBusTask(ddeDockServiceName, newDdeDockTask())
	_manager.AddDBusTask(ddeShutdownServiceName, newDdeShutdownTask())
	go _manager.StartLoop()

	if useKwin {
		_manager.AddDBusTask(kWinServiceName, newDdeKWinTask())
	} else {
		_manager.AddDBusTask(wmServiceName, newWMTask())
	}

	if getLockedFn != nil {
		ddeLockTask := newDdeLock(getLockedFn)
		_manager.AddDBusTask(ddeLockServiceName, ddeLockTask)
	}

	err = _manager.listenDBusSignals()
	if err != nil {
		logger.Warning(err)
	}
	time.AfterFunc(10*time.Second, func() {
		for _, task := range _manager.dbusTasks {
			err = task.Launch()
			if err != nil {
				logger.Warning(err)
			}
		}
	})
}

func (m *Manager) listenDBusSignals() error {
	bus, err := dbus.SessionBus()
	if err != nil {
		return err
	}
	signalChan := make(chan *dbus.Signal, 10)
	bus.Signal(signalChan)

	rule := "type='signal',interface='org.freedesktop.DBus',member='NameOwnerChanged'"
	err = bus.BusObject().Call(orgFreedesktopDBus+".AddMatch", 0, rule).Err
	if err != nil {
		return err
	}

	rule = "type='signal',interface='com.deepin.WMSwitcher',member='WMChanged'"
	err = bus.BusObject().Call(orgFreedesktopDBus+".AddMatch", 0, rule).Err
	if err != nil {
		return err
	}

	go func() {
		for signal := range signalChan {
			if signal.Name == orgFreedesktopDBus+".NameOwnerChanged" &&
				signal.Path == "/org/freedesktop/DBus" && len(signal.Body) == 3 {
				name, ok := signal.Body[0].(string)
				if !ok {
					continue
				}
				taskInfo := m.dbusTasks[name]
				if taskInfo == nil {
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
					logger.Debugf("name lost %q, old owner: %q", name, oldOwner)

					time.AfterFunc(taskInfo.launchDelay, func() {
						err := taskInfo.Launch()
						if err != nil {
							logger.Warningf("failed to launch task %s: %v", taskInfo.Name, err)
						}
					})

				} else if oldOwner == "" && newOwner != "" {
					logger.Debugf("name acquired %q, new owner: %q", name, newOwner)
					var pid uint32
					err := busObj.Call(orgFreedesktopDBus+".GetConnectionUnixProcessID", 0,
						newOwner).Store(&pid)
					if err != nil {
						logger.Warningf("failed to get conn %q pid: %v", newOwner, err)
						continue
					}

					process := procfs.Process(pid)
					exe, err := process.Exe()
					if err != nil {
						logger.Warningf("failed to get process %d exe:", pid)
						continue
					}
					logger.Debugf("exe: %q", exe)
				}
			} else if signal.Name == "com.deepin.WMSwitcher.WMChanged" &&
				signal.Path == "/com/deepin/WMSwitcher" && len(signal.Body) == 1 {
				name, ok := signal.Body[0].(string)
				if !ok {
					continue
				}
				logger.Debugf("wm changed %q", name)
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
}

func SetLogLevel(level log.Priority) {
	logger.SetLogLevel(level)
}

func GetManager() *Manager {
	return _manager
}
