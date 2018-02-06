/*
 * Copyright (C) 2017 ~ 2017 Deepin Technology Co., Ltd.
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
	"dbus/org/freedesktop/login1"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"pkg.deepin.io/lib/utils"
	"pkg.deepin.io/lib/xdg/basedir"
)

const (
	ddePolkitAgentCommand  = "/usr/lib/polkit-1-dde/dde-polkit-agent"
	ddePolkitAgentDBusPath = "/com/deepin/polkit/AuthenticationAgent"
)

func isDDEPolkitAgentRunning() bool {
	// only listen dde polkit agent
	if !utils.IsFileExist(ddePolkitAgentCommand) {
		return true
	}

	pidFile := filepath.Join(basedir.GetUserCacheDir(),
		"deepin",
		"dde-polkit-agent",
		"pid")
	contents, err := ioutil.ReadFile(pidFile)
	if err != nil {
		return false
	}
	cmdline := filepath.Join("/proc", string(contents), "cmdline")
	contents, err = ioutil.ReadFile(cmdline)
	if err != nil {
		return false
	}
	return (string(contents) == ddePolkitAgentCommand)
}

func launchDDEPolkitAgent() error {
	var cmd = exec.Command(ddePolkitAgentCommand)
	err := cmd.Start()
	if err != nil {
		return err
	}
	go func() {
		err := cmd.Wait()
		if err != nil {
			logger.Warning("Failed to wait dde polkit agent exec:", err)
		}
	}()
	return nil
}

func newDDEPolkitAgent() *taskInfo {
	return newTaskInfo("dde-polkit-agent", isDDEPolkitAgentRunning, launchDDEPolkitAgent)
}

func getCurrentSessionID() string {
	self, err := login1.NewSession("org.freedesktop.login1", "/org/freedesktop/login1/session/self")
	if err != nil {
		fmt.Println("Failed to create self session:", err)
		return ""
	}
	defer login1.DestroySession(self)

	return self.Id.Get()
}
