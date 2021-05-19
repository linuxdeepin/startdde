/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     weizhixiang <weizhixiang@uniontech.com>
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

const (
	dueShellTaskName    = "due-shell"
	dueShellServiceName = "com.deepin.due.shell"
	dueShellCommand     = "due-shell"
)

func isDueShellRunning() (bool, error) {
	return isDBusServiceExist(dueShellServiceName)
}

func launchDueShell() error {
	return launchCommand(dueShellCommand, nil, dueShellTaskName)
}

func newDueShellTask() *taskInfo {
	return newTaskInfo(dueShellTaskName, isDueShellRunning, launchDueShell)
}
