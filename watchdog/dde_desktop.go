/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package watchdog

import "os"

const (
	ddeDesktopTaskName    = "dde-desktop"
	ddeDesktopServiceName = "com.deepin.dde.desktop"
)

func isDdeDesktopRunning() (bool, error) {
	return isDBusServiceExist(ddeDesktopServiceName)
}

func launchDdeDesktop() error {
	if os.Getenv("XDG_SESSION_DESKTOP") != "deepin-tablet" {
		return startService(ddeDesktopServiceName)
	} else {
		return launchCommand(ddeDesktopTaskName, []string{"--filedialog-only"}, ddeDesktopTaskName)
	}
}

func newDdeDesktopTask() *taskInfo {
	return newTaskInfo(ddeDesktopTaskName, isDdeDesktopRunning, launchDdeDesktop)
}
