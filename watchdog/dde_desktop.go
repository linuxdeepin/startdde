// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package watchdog

const (
	ddeDesktopTaskName    = "dde-desktop"
	ddeDesktopServiceName = "com.deepin.dde.desktop"
)

func isDdeDesktopRunning() (bool, error) {
	return isDBusServiceExist(ddeDesktopServiceName)
}

func launchDdeDesktop() error {
	return startService(ddeDesktopServiceName)
}

func newDdeDesktopTask() *taskInfo {
	return newTaskInfo(ddeDesktopTaskName, isDdeDesktopRunning, launchDdeDesktop)
}
