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

import (
	"os/exec"
	"strings"
)

const (
	dueLauncherTaskName    = "due-launcher"
	dueLauncherCommand     = "/usr/bin/due-launcher"
)

func isDueLauncherRunning() (bool, error) {
	// due-launcher暂时没有服务，判断进程是否存在
	out, err := exec.Command("/bin/sh", "-c",  "ps -ef | grep " + dueLauncherTaskName).CombinedOutput()
	if err != nil {
		return false, err
	}
	return strings.Contains(string(out), dueLauncherCommand), nil
}

func launchDueLauncher() error {
	return launchCommand(dueLauncherCommand, nil, dueLauncherTaskName)
}

func newDuelauncherTask() *taskInfo {
	return newTaskInfo(dueLauncherTaskName, isDueLauncherRunning, launchDueLauncher)
}
