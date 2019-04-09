/*
 * Copyright (C) 2017 ~ 2018 Deepin Technology Co., Ltd.
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

import (
	"errors"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strconv"

	"pkg.deepin.io/lib/procfs"
	"pkg.deepin.io/lib/utils"
	"pkg.deepin.io/lib/xdg/basedir"
)

const (
	ddePolkitAgentTaskName = "dde-polkit-agent"
	ddePolkitAgentCommand  = "/usr/lib/polkit-1-dde/dde-polkit-agent"
)

func isDdePolkitAgentRunning() (bool, error) {
	if !utils.IsFileExist(ddePolkitAgentCommand) {
		return false, errors.New("dde-polkit-agent bin not exist")
	}

	pidFile := filepath.Join(basedir.GetUserCacheDir(), "deepin", "dde-polkit-agent", "pid")
	pidFileContent, err := ioutil.ReadFile(pidFile)
	if err != nil {
		return false, nil
	}
	pid, err := strconv.ParseUint(string(pidFileContent), 10, 64)
	if err != nil {
		return false, nil
	}
	process := procfs.Process(pid)
	cmdline, err := process.Cmdline()
	if err != nil {
		// maybe pid is wrong
		return false, nil
	}
	if len(cmdline) == 0 {
		return false, nil
	}
	return cmdline[0] == ddePolkitAgentCommand, nil
}

func launchDdePolkitAgent() error {
	var cmd = exec.Command(ddePolkitAgentCommand)
	err := cmd.Start()
	if err != nil {
		logger.Warning("failed to start dde-polkit-agent:", err)
		return err
	}
	go func() {
		err := cmd.Wait()
		if err != nil {
			logger.Warning("dde-polkit-agent exit with error:", err)
		}
	}()
	return nil
}

func newDdePolkitAgent() *taskInfo {
	return newTaskInfo(ddePolkitAgentTaskName, isDdePolkitAgentRunning, launchDdePolkitAgent)
}
