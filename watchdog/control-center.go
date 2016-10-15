/**
 * Copyright (C) 2016 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package watchdog

import (
	"dbus/com/deepin/dde/controlcenter"
)

const (
	controlCenterName = "dde-control-center"
	controlCenterDest = "com.deepin.dde.ControlCenter"
	controlCenterPath = "/com/deepin/dde/ControlCenter"
)

func isControlCenterRunning() bool {
	return isDBusDestExist(controlCenterDest)
}

func launchControlCenter() error {
	caller, err := controlcenter.NewControlCenter(controlCenterDest, controlCenterPath)
	if err != nil {
		return err
	}

	_, err = caller.IsNetworkCanShowPassword()
	return err
}

func newControlCenterTask() *taskInfo {
	return newTaskInfo(controlCenterName, isControlCenterRunning, launchControlCenter)
}
