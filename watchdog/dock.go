/**
 * Copyright (C) 2016 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package watchdog

const (
	dockName = "dde-dock"
	dockDest = "com.deepin.dde.Dock"
)

func isDockRunning() bool {
	return isDBusDestExist(dockDest)
}

func launchDock() error {
	return startService(dockDest)
}

func newDockTask() *taskInfo {
	return newTaskInfo(dockName, isDockRunning, launchDock)
}
