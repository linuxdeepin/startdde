/**
 * Copyright (C) 2016 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

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
