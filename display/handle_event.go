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
		dpy.fixOutputNotClosed(info.Output)
	}
}

func (dpy *Manager) handleScreenChanged(ev randr.ScreenChangeNotifyEvent) {
	// firstly compare plan whether equal
	oldLen := len(dpy.outputInfos)
	oinfos, minfos, err := drandr.GetScreenInfo(dpy.conn)
	if err != nil {
		logger.Error("Get screen info failed:", err)
		return
	}
	dpy.outputInfos, dpy.modeInfos = oinfos.ListConnectionOutputs().ListValidOutputs(), minfos
	dpy.updateMonitors()
	logger.Debug("[Event] compare:", dpy.lastConfigTime, ev.ConfigTimestamp, oldLen, len(dpy.outputInfos))
	if dpy.lastConfigTime < ev.ConfigTimestamp {
		if oldLen != len(dpy.outputInfos) {
			logger.Infof("Detect new output config, try to apply it: %#v", oinfos)
			dpy.lastConfigTime = ev.ConfigTimestamp
			err := dpy.tryApplyConfig()
			if err != nil {
				logger.Error("Apply failed for event:", err)
			}
			dpy.setPropHasCustomConfig(dpy.config.get(dpy.Monitors.getMonitorsId()) != nil)
		}
	}
	dpy.setPropScreenSize(ev.Width, ev.Height)
	dpy.doSetPrimary(dpy.Primary, true) // update if monitor mode changed
	// TODO: map touchscreen
}
