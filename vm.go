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

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.display"
	"pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/utils"
	"pkg.deepin.io/lib/xdg/basedir"
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
	cfgFile := filepath.Join(basedir.GetUserConfigDir(), "deepin", "deepin-wm-switcher", "config.json")
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		err := exec.Command("dde-wm-chooser", "-c", cfgFile).Run()
		globalWmChooserLaunched = true
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

	sessionBus, err := dbus.SessionBus()
	if err != nil {
		logger.Warning(err)
		return
	}
	disp := display.NewDisplay(sessionBus)

	primary, err := disp.Primary().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}

	monitorPath := dbus.ObjectPath(fmt.Sprintf("%v/Monitor%s", disp.Path_(),
		strings.Replace(primary, "-", "_", -1)))
	output, err := display.NewMonitor(sessionBus, monitorPath)
	if err != nil {
		logger.Warning(err)
		return
	}

	width, err := output.Width().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}

	height, err := output.Width().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}

	X, err := output.X().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}

	y, err := output.Y().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}

	// if resolution < 1024x768, try set to 1024x768
	if int16(width)-X > 1024 ||
		int16(height)-y > 768 {
		return
	}

	outputName, err := output.Name().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}
	err = output.SetModeBySize(0, 1024, 768)
	if err != nil {
		logger.Warning("Failed to set mode to 1024x768 for:", outputName, err)
		return
	}

	err = disp.ApplyChanges(0)
	if err != nil {
		logger.Warning("Failed to apply mode change for:", outputName, err)
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
