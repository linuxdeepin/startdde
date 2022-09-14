// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

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

	obj := conn.Object("com.deepin.WMSwitcher", "/com/deepin/WMSwitcher")
	err = obj.Call("com.deepin.WMSwitcher.Start2DWM", 0).Store()
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
