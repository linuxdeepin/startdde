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

package main

import (
	"dbus/com/deepin/daemon/display"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"pkg.deepin.io/lib/dbus"
	"pkg.deepin.io/lib/utils"
	"pkg.deepin.io/lib/xdg/basedir"
	"strings"
)

func tryMatchVM() {
	inVM, err := isInVM()
	if err != nil {
		logger.Warning("launchWindowManager detect VM failed:", err)
		return
	}

	if !inVM {
		return
	}

	logger.Debug("launchWindowManager in VM")
	cfgFile := filepath.Join(basedir.GetUserConfigDir(), "deepin-wm-switcher", "config.json")
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		err := exec.Command("dde-wm-chooser", "-c", cfgFile).Run()
		if err != nil {
			logger.Warning(err)
		}
	}
}

func correctVMResolution() {
	// check user config whether exists
	if utils.IsFileExist(filepath.Join(basedir.GetUserConfigDir(),
		"deepin", "startdde", "display.json")) {
		return
	}

	dispDest := "com.deepin.daemon.Display"
	dispPath := dbus.ObjectPath("/com/deepin/daemon/Display")
	disp, err := display.NewDisplay(dispDest, dispPath)
	if err != nil {
		logger.Warning("Failed to connect display dbus:", err)
		return
	}
	defer display.DestroyDisplay(disp)

	output, err := display.NewMonitor(dispDest, dbus.ObjectPath(fmt.Sprintf("%v/Monitor%s", dispPath,
		strings.Replace(disp.Primary.Get(), "-", "_", -1))))
	if err != nil {
		logger.Warningf("Failed to connect %q dbus: %v", disp.Primary.Get(), err)
		return
	}
	defer display.DestroyMonitor(output)

	// if resolution < 1024x768, try set to 1024x768
	if int16(output.Width.Get())-output.X.Get() > 1024 ||
		int16(output.Height.Get())-output.Y.Get() > 768 {
		return
	}
	err = output.SetModeBySize(1024, 768)
	if err != nil {
		logger.Warning("Failed to set mode to 1024x768 for:", output.Name.Get(), err)
		return
	}

	err = disp.ApplyChanges()
	if err != nil {
		logger.Warning("Failed to apply mode change for:", output.Name.Get(), err)
		return
	}
}

func isInVM() (bool, error) {
	cmd := exec.Command("systemd-detect-virt", "-v", "-q")
	err := cmd.Start()
	if err != nil {
		return false, err
	}

	err = cmd.Wait()
	return err == nil, nil
}
