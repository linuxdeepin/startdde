// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus"
	display "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.display"
	"github.com/linuxdeepin/go-lib/utils"
	"github.com/linuxdeepin/go-lib/xdg/basedir"
)

const (
	versionFile = "/etc/os-version"
)

func maybeLaunchWMChooser() (launched bool) {
	logger.Debug("launch WMChooser in VM")
	cfgFile := filepath.Join(basedir.GetUserConfigDir(), "deepin", "deepin-wm-switcher", "config.json")
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		err := exec.Command("dde-wm-chooser", "-c", cfgFile).Run()
		launched = true
		if err != nil {
			logger.Warning(err)
		}
	}
	return
}

func isServer() bool {
	t := strings.ToLower(getProductType())
	return strings.Contains(t, "server")
}

func getProductType() string {
	keyFile, err := utils.NewKeyFileFromFile(versionFile)
	if err != nil {
		return ""
	}
	defer keyFile.Free()
	releaseType, _ := keyFile.GetString("Version", "ProductType")
	return releaseType
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

	height, err := output.Height().Get(0)
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
