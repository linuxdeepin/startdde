/**
 * Copyright (C) 2016 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package display

import (
	"fmt"
	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/randr"
	"strings"
)

const (
	backlightDest = "com.deepin.daemon.helper.Backlight"
	backlightPath = "/com/deepin/daemon/helper/Backlight"
)

func (dpy *Display) getNumOfOpenedMonitor() int32 {
	var num int32 = 0
	for _, m := range dpy.Monitors {
		if m.Opened {
			num += 1
		}
	}
	return num
}

func (dpy *Display) supportedBacklight(c *xgb.Conn, output randr.Output) bool {
	if dpy.blHelper == nil {
		return false
	}

	list, _ := dpy.blHelper.ListSysPath()
	if (dpy.getNumOfOpenedMonitor() == 1) && (len(list) != 0) {
		return true
	}

	return hasPropBacklight(c, output)
}

func (dpy *Display) listBacklightSysPath() []string {
	if dpy.blHelper == nil {
		return nil
	}
	list, _ := dpy.blHelper.ListSysPath()
	return list
}

func (dpy *Display) getBacklightSysPath(ty string) (string, error) {
	if dpy.blHelper == nil {
		return "", fmt.Errorf("Create backlight helper failed")
	}

	var key string
	switch {
	case strings.Contains(ty, "raw"):
		key = "raw"
	case strings.Contains(ty, "platform"):
		key = "platform"
	case strings.Contains(ty, "firmware"):
		key = "firmware"
	}
	return dpy.blHelper.GetSysPathByType(key)
}

func (dpy *Display) getBacklight(setter string) float64 {
	sysPath, _ := dpy.getBacklightSysPath(setter)
	if len(sysPath) == 0 {
		sysPath, _ = dpy.getBacklightSysPath("raw")
		if len(sysPath) == 0 {
			list := dpy.listBacklightSysPath()
			if len(list) == 0 {
				return 1
			}
			sysPath = list[0]
		}
	}

	v, _ := dpy.doGetBacklight(sysPath)
	max, _ := dpy.doGetMaxBacklight(sysPath)
	return float64(v) / float64(max)
}

func (dpy *Display) doGetBacklight(sysPath string) (int32, error) {
	if dpy.blHelper == nil {
		return 1, fmt.Errorf("Create backlight helper failed")
	}
	return dpy.blHelper.GetBrightness(sysPath)
}

func (dpy *Display) doGetMaxBacklight(sysPath string) (int32, error) {
	if dpy.blHelper == nil {
		return 1, fmt.Errorf("Create backlight helper failed")
	}
	return dpy.blHelper.GetMaxBrightness(sysPath)
}

func (dpy *Display) setBacklight(name, setter string, value float64) {
	if dpy.blHelper == nil {
		return
	}

	old := dpy.Brightness[name]
	sysPath, _ := dpy.getBacklightSysPath(setter)
	if len(sysPath) != 0 {
		err := dpy.doSetBacklight(sysPath, old, value)
		if err != nil {
			logger.Warningf("Set backlight for (%s - %s) to (%lf -> %lf) failed: %v", name, sysPath, old, value, err)
		}
		return
	}

	for _, p := range dpy.listBacklightSysPath() {
		err := dpy.doSetBacklight(p, old, value)
		if err != nil {
			logger.Warningf("Set backlight for (%s - %s) to (%lf -> %lf) failed: %v", name, p, old, value, err)
		}
	}
}

func (dpy *Display) doSetBacklight(sysPath string, old, value float64) error {
	if dpy.blHelper == nil {
		return fmt.Errorf("Create backlight helper failed")
	}

	now, err := dpy.blHelper.GetBrightness(sysPath)
	if err != nil {
		return err
	}
	max, _ := dpy.blHelper.GetMaxBrightness(sysPath)
	if max < 1 {
		return fmt.Errorf("Can not get max brightness for %s", sysPath)
	}

	tmp := int32(float64(max) * value)
	if tmp == now {
		// if max < 20, such as 9, the 55% and 60% was both 5
		if old < value {
			tmp += 1
		} else {
			tmp -= 1
		}
	}

	return dpy.blHelper.SetBrightness(sysPath, tmp)
}
