// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package watchdog

import (
	"errors"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/linuxdeepin/go-lib/procfs"
	"github.com/linuxdeepin/go-lib/utils"
	"github.com/linuxdeepin/go-lib/xdg/basedir"
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
