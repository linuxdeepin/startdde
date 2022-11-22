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
	"time"

	dbus "github.com/godbus/dbus"
)

const (
	wmTaskName    = "wm"
	wmServiceName = "com.deepin.wm"
)

func isWMRunning() (bool, error) {
	return isDBusServiceExist(wmServiceName)
}

func launchWM() error {
	conn, err := dbus.SessionBus()
	if err != nil {
		return err
	}

	obj := conn.Object("org.deepin.dde.WMSwitcher1", "/org/deepin/dde/WMSwitcher1")
	err = obj.Call("org.deepin.dde.WMSwitcher1.Start2DWM", 0).Store()
	if err != nil {
		return err
	}
	return nil
}

func newWMTask() *taskInfo {
	task := newTaskInfo(wmTaskName, isWMRunning, launchWM)
	task.launchDelay = 3 * time.Second
	return task
}
