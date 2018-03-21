/**
 * Copyright (C) 2018 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package watchdog

import (
	"pkg.deepin.io/lib/dbus"
)

const (
	wmName = "wm"
	wmDest = "com.deepin.wm"
)

func isWMRunning() (bool, error) {
	return isDBusServiceExist(wmDest)
}

func launchWM() error {
	conn, err := dbus.SessionBus()
	if err != nil {
		return err
	}

	obj := conn.Object("com.deepin.WMSwitcher", "/com/deepin/WMSwitcher")
	err = obj.Call("com.deepin.WMSwitcher.RestartWM", 0).Store()
	if err != nil {
		return err
	}
	return nil
}

func newWMTask() *taskInfo {
	return newTaskInfo(wmName, isWMRunning, launchWM)
}
