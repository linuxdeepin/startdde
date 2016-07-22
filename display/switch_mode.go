/**
 * Copyright (C) 2014 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package display

import (
	"fmt"
	"sort"
	"strings"
)

const (
	DisplayModeUnknow  = 4
	DisplayModeCustom  = 0
	DisplayModeMirrors = 1
	DisplayModeExtend  = 2
	DisplayModeOnlyOne = 3
)

func (dpy *Display) SwitchMode(mode int16, outputName string) {
	logger.Debug("[SwitchMode] start to :", mode, outputName)
	if len(GetDisplayInfo().ListOutputs()) == 0 {
		logger.Warning("No output be found")
		return
	}
	switch mode {
	case DisplayModeMirrors:
		if len(dpy.Monitors) == 0 {
			logger.Error("Invoking SwitchMode with none Monitors.")
			return
		}

		dpy.syncDisplayMode(mode)
		dpy.cfg.attachCurrentMonitor(dpy)
		dpy.rebuildMonitors()

		n := len(dpy.Monitors)
		if n == 1 {
			m := dpy.Monitors[0]
			m.SetPos(0, 0)
			m.SetMode(m.BestMode.ID)
			m.SwitchOn(true)
		} else {
			for ; n > 1; n = len(dpy.Monitors) {
				err := dpy.JoinMonitor(dpy.Monitors[n-1].Name, dpy.Monitors[n-2].Name)
				if err != nil {
					logger.Warning("Switch to mirrors mode failed:", err)
					break
				}
			}
		}
		dpy.apply(false)
		dpy.cfg.Save()
	case DisplayModeExtend:
		logger.Debugf("-------------Switch to extend: '%s', %#v\n", dpy.Primary, dpy.cfg.Plans[dpy.QueryCurrentPlanName()])
		dpy.syncDisplayMode(mode)
		dpy.joinExtendMode()
		logger.Debugf("-------------Switch to extend done: '%s', %#v\n", dpy.Primary, dpy.cfg.Plans[dpy.QueryCurrentPlanName()])
	case DisplayModeOnlyOne:
		func() {
			dpy.lockMonitors()
			outputNameValid := GetDisplayInfo().QueryOutputs(outputName) != 0
			//validValue := mode >= DisplayModeOnlyOne && int(mode) <= len(dpy.Monitors)
			dpy.unlockMonitors()
			if !outputNameValid {
				logger.Warning("Invalid output:", outputName)
				return
			}

			dpy.syncDisplayMode(mode)
			dpy.cfg.attachCurrentMonitor(dpy)
			logger.Debugf("+++++++++++++++++++Switch only mode: '%s', %#v\n", dpy.cfg.CurrentPlanName, dpy.cfg.Plans)
			dpy.cfg.Plans[dpy.QueryCurrentPlanName()].DefaultOutput = outputName
			dpy.rebuildMonitors()

			for _, m := range dpy.Monitors {
				dpy.SplitMonitor(m.Name)
			}

			for _, m := range dpy.Monitors {
				if m.Name == outputName {
					m.SetPos(0, 0)
					m.SetMode(m.BestMode.ID)
					m.SwitchOn(true)
					dpy.changePrimary(m.Name, true)
				}
			}
			for _, m := range dpy.Monitors {
				if m.Name != outputName {
					m.SwitchOn(false)
				}
			}
			dpy.apply(false)
			dpy.cfg.Save()
		}()
	case DisplayModeCustom:
		logger.Debugf("-------------Switch to custom: '%s', %#v\n", dpy.Primary, dpy.cfg.Plans[dpy.QueryCurrentPlanName()])
		dpy.syncDisplayMode(mode)
		dpy.joinExtendMode()
		logger.Debugf("-------------Switch to custom done: '%s', %#v\n", dpy.Primary, dpy.cfg.Plans[dpy.QueryCurrentPlanName()])
	}
	dpy.detectChanged()
}

func (dpy *Display) joinExtendMode() {
	added := dpy.cfg.attachCurrentMonitor(dpy)
	dpy.rebuildMonitors()
	if len(dpy.Monitors) < 1 {
		return
	}
	if added {
		// sort monitor by primary
		dpy.sortMonitors()
		dpy.cfg.Plans[dpy.QueryCurrentPlanName()].DefaultOutput = dpy.Monitors[0].Name

		curX := int16(0)
		for _, m := range dpy.Monitors {
			m.changeSwitchOn(true)
			m.cfg.Enabled = true
			m.SetPos(curX, 0)
			m.SetMode(m.BestMode.ID)
			curX += int16(m.BestMode.Width)
		}
	}
	logger.Debugf("~~~~~~~~~~~~~Join exten added(%b): '%s' %#v\n",
		added, dpy.Primary, dpy.cfg.Plans[dpy.QueryCurrentPlanName()])
	dpy.Primary = dpy.cfg.Plans[dpy.QueryCurrentPlanName()].DefaultOutput
	dpy.apply(false)
	dpy.cfg.Save()
	dpy.SetPrimary(dpy.Primary)
	logger.Debugf("~~~~~~~~~~~~~Join exten done: '%s' %#v\n", dpy.Primary, dpy.cfg.Plans[dpy.QueryCurrentPlanName()])
}

func (dpy *Display) sortMonitors() {
	if len(dpy.Monitors) < 2 {
		return
	}

	var sorter = func(checker func(string) bool) []*Monitor {
		var (
			primaryM *Monitor
			tmpList  []*Monitor
		)
		for _, m := range dpy.Monitors {
			if primaryM != nil && checker(m.Name) {
				primaryM = m
				continue
			}
			tmpList = append(tmpList, m)
		}
		if primaryM == nil {
			return dpy.Monitors
		}

		var retList []*Monitor
		retList = append(retList, primaryM)
		retList = append(retList, tmpList...)
		return retList
	}

	logger.Debug("[sortMonitors] current plan:", dpy.getPlanNameByMonitors())
	group, ok := dpy.cfg.Plans[dpy.getPlanNameByMonitors()]
	if !ok {
		// sort by builtin output
		dpy.Monitors = sorter(isBuiltinOuput)
		return
	}

	// sort by primary
	logger.Debug("[sortMonitors] primary:", group.DefaultOutput)
	dpy.Monitors = sorter(func(name string) bool {
		if name == group.DefaultOutput {
			return true
		}
		return false
	})
}

func (dpy *Display) getPlanNameByMonitors() string {
	var names []string
	for _, m := range dpy.Monitors {
		names = append(names, m.Name)
	}
	sort.Strings(names)
	return strings.Join(names, ",") + fmt.Sprintf(",mode%v", dpy.DisplayMode)
}
