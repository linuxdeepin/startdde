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
	if len(dpy.disableList) == 0 {
		return
	}
	var cmd = "xrandr "
	for _, name := range dpy.disableList {
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
