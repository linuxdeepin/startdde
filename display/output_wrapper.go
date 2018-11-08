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

package display

import (
	"pkg.deepin.io/dde/api/drandr"
	"pkg.deepin.io/lib/strv"
)

const (
	gsKeyBlacklist = "blacklist"
	gsKeyPriority  = "priority"
)

func (dpy *Manager) filterOutputs(outputInfos drandr.OutputInfos) (drandr.OutputInfos, []string) {
	var list = dpy.setting.GetStrv(gsKeyBlacklist)
	if len(list) == 0 {
		return outputInfos, nil
	}
	logger.Debug("----------Blacklist:", list)
	logger.Debugf("----------Outputs: %#v", outputInfos)
	var infos drandr.OutputInfos
	var disableList []string
	for _, info := range outputInfos {
		if strv.Strv(list).Contains(info.Name) {
			disableList = append(disableList, info.Name)
			continue
		}
		infos = append(infos, info)
	}
	if len(disableList) == 0 {
		return outputInfos, nil
	}
	logger.Debugf("----------Outputs DONE: %#v", infos)
	logger.Debug("-----------Disable list:", disableList)
	return infos, disableList
}

func (dpy *Manager) disableOutputs() {
	turnOffOutputs(dpy.disableList)
}

func turnOffOutputs(names []string) {
	if len(names) == 0 {
		return
	}
	var cmd = "xrandr "
	for _, name := range names {
		cmd += " --output " + name + " --off"
	}
	err := doAction(cmd)
	if err != nil {
		logger.Warningf("Disable outputs(cmd: %s) failed: %v", cmd, err)
	}
}

func (dpy *Manager) sortMonitors() {
	var list = dpy.setting.GetStrv(gsKeyPriority)
	if len(list) == 0 {
		return
	}
	logger.Debugf("----------Priority: %v", list)
	logger.Debugf("----------Monitors: %#v", dpy.allMonitors)
	dpy.allMonitors = dpy.allMonitors.sort(list)
	logger.Debugf("----------Monitors DONE: %#v", dpy.allMonitors)
}
