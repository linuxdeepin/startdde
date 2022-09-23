// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package watchdog

const (
	ddeDockTaskName    = "dde-dock"
	ddeDockServiceName = "com.deepin.dde.Dock"
	ddeDockCommand     = "dde-dock"
)

func isDdeDockRunning() (bool, error) {
	return isDBusServiceExist(ddeDockServiceName)
}

func launchDdeDock() error {
	return launchCommand(ddeDockCommand, nil, ddeDockTaskName)
}

func newDdeDockTask() *taskInfo {
	return newTaskInfo(ddeDockTaskName, isDdeDockRunning, launchDdeDock)
}
