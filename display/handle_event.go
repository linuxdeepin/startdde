package display

import (
	"github.com/BurntSushi/xgb/randr"
	"github.com/BurntSushi/xgb/xproto"
	"pkg.deepin.io/dde/api/drandr"
)

func (dpy *Manager) listenEvent() {
	randr.SelectInputChecked(dpy.conn, xproto.Setup(dpy.conn).DefaultScreen(dpy.conn).Root,
		randr.NotifyMaskOutputChange|randr.NotifyMaskOutputProperty|
			randr.NotifyMaskCrtcChange|randr.NotifyMaskScreenChange)
	for {
		e, err := dpy.conn.WaitForEvent()
		if err != nil {
			continue
		}
		dpy.eventLocker.Lock()
		switch ee := e.(type) {
		case randr.NotifyEvent:
			switch ee.SubCode {
			case randr.NotifyCrtcChange:
			case randr.NotifyOutputChange:
				dpy.handleOutputChanged(ee)
			case randr.NotifyOutputProperty:
			}
		case randr.ScreenChangeNotifyEvent:
			dpy.handleScreenChanged(ee)
		}
		dpy.eventLocker.Unlock()
	}
}

func (dpy *Manager) handleOutputChanged(ev randr.NotifyEvent) {
	info := ev.U.Oc
	if info.Connection != randr.ConnectionConnected && info.Mode != 0 {
		randr.SetCrtcConfig(dpy.conn, info.Crtc,
			xproto.TimeCurrentTime, dpy.lastConfigTime, 0, 0, 0,
			randr.RotationRotate0, nil)
	}
	if info.Mode == 0 || info.Crtc == 0 {
		// TODO: if info in blacklist, what happen?
		dpy.fixOutputNotClosed(info.Output)
	}
}

var autoOpenTimes int = 0

func (dpy *Manager) handleScreenChanged(ev randr.ScreenChangeNotifyEvent) {
	// firstly compare plan whether equal
	oldLen := len(dpy.outputInfos)
	screenInfo, err := drandr.GetScreenInfo(dpy.conn)
	if err != nil {
		logger.Error("Get screen info failed:", err)
		return
	}
	outputInfos := screenInfo.Outputs.ListConnectionOutputs().ListValidOutputs()
	tmpInfos, tmpList := dpy.filterOutputs(outputInfos)
	if oldLen != len(tmpInfos) && dpy.isOutputCrtcInvalid(tmpInfos) && autoOpenTimes < 2 {
		// only try 2 times
		autoOpenTimes += 1
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
	if dpy.lastConfigTime < ev.ConfigTimestamp {
		if oldLen != len(dpy.outputInfos) {
			logger.Infof("Detect new output config, try to apply it: %#v", screenInfo.Outputs)
			dpy.lastConfigTime = ev.ConfigTimestamp
			err := dpy.tryApplyConfig()
			if err != nil {
				logger.Error("Apply failed for event:", err)
			}
		}
	}
	dpy.updateScreenSize()
	dpy.doSetPrimary(dpy.Primary, true) // update if monitor mode changed
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
