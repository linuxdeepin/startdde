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
	"github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
	"pkg.deepin.io/dde/api/drandr"
)

func (dpy *Manager) listenEvent() {
	eventChan := make(chan x.GenericEvent, 100)
	dpy.conn.AddEventChan(eventChan)

	root := dpy.conn.GetDefaultScreen().Root
	err := randr.SelectInputChecked(dpy.conn, root,
		randr.NotifyMaskOutputChange|randr.NotifyMaskOutputProperty|
			randr.NotifyMaskCrtcChange|randr.NotifyMaskScreenChange).Check(dpy.conn)
	if err != nil {
		logger.Warning("failed to select randr event:", err)
		return
	}

	rrExtData := dpy.conn.GetExtensionData(randr.Ext())

	go func() {
		for ev := range eventChan {
			dpy.eventLocker.Lock()
			switch ev.GetEventCode() {
			case randr.NotifyEventCode + rrExtData.FirstEvent:
				event, _ := randr.NewNotifyEvent(ev)
				switch event.SubCode {
				case randr.NotifyOutputChange:
					ocEv, _ := event.NewOutputChangeNotifyEvent()
					dpy.handleOutputChanged(ocEv)
				}
			case randr.ScreenChangeNotifyEventCode + rrExtData.FirstEvent:
				event, _ := randr.NewScreenChangeNotifyEvent(ev)
				dpy.handleScreenChanged(event)
			}
			dpy.eventLocker.Unlock()
		}
	}()
}

func (dpy *Manager) handleOutputChanged(ev *randr.OutputChangeNotifyEvent) {
	if ev.Connection != randr.ConnectionConnected && ev.Mode != 0 {
		randr.SetCrtcConfig(dpy.conn, ev.Crtc,
			x.CurrentTime, dpy.lastConfigTime, 0, 0, 0,
			randr.RotationRotate0, nil)
	}
	if ev.Mode == 0 || ev.Crtc == 0 {
		// TODO: if info in blacklist, what happen?
		dpy.fixOutputNotClosed(ev.Output)
	}
}

var autoOpenTimes int = 0

func (dpy *Manager) handleScreenChanged(ev *randr.ScreenChangeNotifyEvent) {
	logger.Debugf("handleScreenChanged ev: %#v", ev)
	// firstly compare plan whether equal
	oldLen := len(dpy.outputInfos)
	screenInfo, err := drandr.GetScreenInfo(dpy.conn)
	if err != nil {
		logger.Error("Get screen info failed:", err)
		return
	}
	outputInfos := screenInfo.Outputs.ListConnectionOutputs()
	tmpInfos, tmpList := dpy.filterOutputs(outputInfos)
	if oldLen != len(tmpInfos) && dpy.isOutputCrtcInvalid(tmpInfos) && autoOpenTimes < 2 {
		// only try 2 times
		autoOpenTimes++
		doAction("xrandr --auto")
		return
	}
	if autoOpenTimes != 0 || (len(tmpList) != 0 && len(tmpList) != len(dpy.disableList)) {
		dpy.disableList = tmpList
		dpy.disableOutputs()
	}
	autoOpenTimes = 0

	dpy.outputInfos = tmpInfos
	dpy.modeInfos = screenInfo.Modes
	dpy.updateMonitors()
	logger.Debug("[Event] compare:", dpy.lastConfigTime, ev.ConfigTimestamp, oldLen, len(dpy.outputInfos))
	// some platform the config timestamp maybe equal between twice event, so not check
	// if dpy.lastConfigTime < ev.ConfigTimestamp {
	if oldLen != len(dpy.outputInfos) {
		logger.Infof("Detect new output config, try to apply it: %#v", screenInfo.Outputs)
		dpy.lastConfigTime = ev.ConfigTimestamp
		err := dpy.tryApplyConfig()
		if err != nil {
			logger.Error("Apply failed for event:", err)
		}
	}
	dpy.updateScreenSize()
	dpy.doSetPrimary(dpy.Primary, true, false) // update if monitor mode changed
	// TODO: map touchscreen
}

func (dpy *Manager) isOutputCrtcInvalid(infos drandr.OutputInfos) bool {
	// output connected, but no crtc
	for _, info := range infos {
		if info.Crtc.Id == 0 || info.Crtc.Width == 0 || info.Crtc.Height == 0 {
			return true
		}
	}
	return false
}
