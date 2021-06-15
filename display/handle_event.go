package display

import (
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
)

func (m *Manager) listenEvent() {
	eventChan := make(chan x.GenericEvent, 100)
	m.xConn.AddEventChan(eventChan)

	root := m.xConn.GetDefaultScreen().Root
	err := randr.SelectInputChecked(m.xConn, root,
		randr.NotifyMaskOutputChange|randr.NotifyMaskOutputProperty|
			randr.NotifyMaskCrtcChange|randr.NotifyMaskScreenChange).Check(m.xConn)
	if err != nil {
		logger.Warning("failed to select randr event:", err)
		return
	}

	rrExtData := m.xConn.GetExtensionData(randr.Ext())

	go func() {
		for ev := range eventChan {
			switch ev.GetEventCode() {
			case randr.NotifyEventCode + rrExtData.FirstEvent:
				event, _ := randr.NewNotifyEvent(ev)
				switch event.SubCode {
				case randr.NotifyCrtcChange:
					e, _ := event.NewCrtcChangeNotifyEvent()
					m.handleCrtcChanged(e)

				case randr.NotifyOutputChange:
					e, _ := event.NewOutputChangeNotifyEvent()
					m.handleOutputChanged(e)

				case randr.NotifyOutputProperty:
					e, _ := event.NewOutputPropertyNotifyEvent()
					m.handleOutputPropertyChanged(e)
				}

			case randr.ScreenChangeNotifyEventCode + rrExtData.FirstEvent:
				e, _ := randr.NewScreenChangeNotifyEvent(ev)
				m.handleScreenChanged(e)
			}
		}
	}()
}

func (m *Manager) handleOutputChanged(ev *randr.OutputChangeNotifyEvent) {
	logger.Debug("output changed", ev.Output)

	outputInfo, err := m.updateOutputInfo(ev.Output)
	if err != nil {
		logger.Warning(err)
	}

	if outputInfo.Connection != randr.ConnectionConnected &&
		outputInfo.Name == m.Primary {

		for output0, outputInfo0 := range m.outputMap {
			if outputInfo0.Connection == randr.ConnectionConnected {
				// set first connected output as primary
				err = m.setOutputPrimary(output0)
				if err != nil {
					logger.Warning(err)
				}
				if m.DisplayMode == DisplayModeExtend {
					err = m.setPrimary(outputInfo0.Name)
					if err != nil {
						logger.Warning(err)
					}
				}
				break
			}
		}
	}

	m.updateMonitor(ev.Output, outputInfo)
	m.updatePropMonitors()

	oldMonitorsID := m.monitorsId
	newMonitorsID := m.getMonitorsId()
	if newMonitorsID != oldMonitorsID {
		logger.Debug("new monitors id:", newMonitorsID)
		m.markClean()
		m.applyDisplayMode()
		m.monitorsId = newMonitorsID
	}
}

func (m *Manager) handleOutputPropertyChanged(ev *randr.OutputPropertyNotifyEvent) {
	logger.Debug("output property changed", ev.Output, ev.Atom)
}

func (m *Manager) handleCrtcChanged(ev *randr.CrtcChangeNotifyEvent) {
	logger.Debug("crtc changed", ev.Crtc)
	crtcInfo, err := m.updateCrtcInfo(ev.Crtc)
	if err != nil {
		logger.Warning(err)
		return
	}

	var rOutput randr.Output
	var rOutputInfo *randr.GetOutputInfoReply

	m.outputMapMu.Lock()
	for output, outputInfo := range m.outputMap {
		if outputInfo.Crtc == ev.Crtc {
			rOutput = output
			rOutputInfo = outputInfo
			break
		}
	}
	m.outputMapMu.Unlock()

	if rOutput != 0 {
		m.outputMapMu.Lock()
		monitor := m.monitorMap[rOutput]
		m.outputMapMu.Unlock()
		if monitor != nil {
			logger.Debug("update monitor crtc", monitor.ID, monitor.Name)
			m.updateMonitorCrtcInfo(monitor, crtcInfo)
		}
	}

	if rOutputInfo != nil {
		m.PropsMu.Lock()
		if m.Primary == rOutputInfo.Name {
			m.setPropPrimaryRect(getCrtcRect(crtcInfo))
		}
		m.PropsMu.Unlock()
	}

	// 如果实际的屏幕排列与设置的不同，设置为自定义（比如用户自己用 xrandr 命令设置屏幕排列后）
	if m.DisplayMode != DisplayModeCustom {
		displayMode, _ := m.GetRealDisplayMode()
		if displayMode != m.DisplayMode {
			m.PropsMu.Lock()
			m.setPropDisplayMode(DisplayModeCustom)
			m.PropsMu.Unlock()
		}
	}
}

func (m *Manager) handleScreenChanged(ev *randr.ScreenChangeNotifyEvent) {
	logger.Debugf("screen changed cfgTs: %v, screen size: %vx%v ", ev.ConfigTimestamp,
		ev.Width, ev.Height)

	m.PropsMu.Lock()
	m.setPropScreenWidth(ev.Width)
	m.setPropScreenHeight(ev.Height)
	cfgTsChanged := false
	if m.configTimestamp != ev.ConfigTimestamp {
		m.configTimestamp = ev.ConfigTimestamp
		cfgTsChanged = true
	}
	m.PropsMu.Unlock()

	if cfgTsChanged {
		logger.Debug("config timestamp changed")
		if _hasRandr1d2 {
			resources, err := m.getScreenResourcesCurrent()
			if err != nil {
				logger.Warning("failed to get screen resources:", err)
			} else {
				m.modes = resources.Modes
			}
		} else {
			// randr 版本低于 1.2
			root := m.xConn.GetDefaultScreen().Root
			screenInfo, err := randr.GetScreenInfo(m.xConn, root).Reply(m.xConn)
			if err == nil {
				monitor := m.updateMonitorFallback(screenInfo)
				m.setPropPrimaryRect(x.Rectangle{
					X:      monitor.X,
					Y:      monitor.Y,
					Width:  monitor.Width,
					Height: monitor.Height,
				})
			} else {
				logger.Warning(err)
			}
		}
	}

	if _hasRandr1d2 {
		m.updateOutputPrimary()
	}

	logger.Info("redo map touch screen")
	m.handleTouchscreenChanged()

	if cfgTsChanged {
		m.showTouchscreenDialogs()
	}
}
