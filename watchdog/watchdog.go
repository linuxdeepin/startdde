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
	"pkg.deepin.io/lib/log"
)

var (
	logger   = log.NewLogger("daemon/watchdog")
	_manager *Manager
)

func Start() {
	if _manager != nil {
		return
	}

	logger.BeginTracing()
	_manager = newManager()
	_manager.AddTask(newDockTask())
	_manager.AddTask(newDesktopTask())
	go _manager.StartLoop()
	return
}

func Stop() {
	if _manager == nil {
		return
	}

	_manager.QuitLoop()
	_manager = nil
	logger.EndTracing()
	return
}
