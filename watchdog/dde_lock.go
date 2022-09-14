// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package watchdog

const (
	ddeLockTaskName    = "dde-lock"
	ddeLockServiceName = "com.deepin.dde.lockFront"
)

func launchDdeLock() error {
	return startService(ddeLockServiceName)
}

func newDdeLock(getLockedFn func() bool) *taskInfo {
	isDdeLockRunning := func() (bool, error) {
		if getLockedFn() {
			return isDBusServiceExist(ddeLockServiceName)
		} else {
			return false, errNoNeedLaunch
		}
	}
	return newTaskInfo(ddeLockTaskName, isDdeLockRunning, launchDdeLock)
}
