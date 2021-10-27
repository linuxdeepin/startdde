package display

import (
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
)

func (m *Manager) listenXEvents() {
	eventChan := m.xConn.MakeAndAddEventChan(50)
	root := m.xConn.GetDefaultScreen().Root
	// 选择监听哪些 randr 事件
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
				case randr.NotifyOutputChange:
					e, _ := event.NewOutputChangeNotifyEvent()
					m.srm.HandleEvent(e)

				case randr.NotifyCrtcChange:
					e, _ := event.NewCrtcChangeNotifyEvent()
					m.srm.HandleEvent(e)

				case randr.NotifyOutputProperty:
					e, _ := event.NewOutputPropertyNotifyEvent()
					// TODO srm 可能也应该处理这个事件
					m.handleOutputPropertyChanged(e)
				}

			case randr.ScreenChangeNotifyEventCode + rrExtData.FirstEvent:
				event, _ := randr.NewScreenChangeNotifyEvent(ev)
				cfgTsChanged := m.srm.HandleScreenChanged(event)
				m.handleScreenChanged(event, cfgTsChanged)
			}
		}
	}()
}

func (m *Manager) handleMonitorChanged(id uint32) {
	xOutputInfo := m.srm.getMonitor(id)
	if xOutputInfo == nil {
		logger.Warning("monitor is nil")
		return
	}

	m.updateMonitor(xOutputInfo)
	prevNumMonitors := len(m.Monitors)
	m.updatePropMonitors()
	currentNumMonitors := len(m.Monitors)

	logger.Debugf("prevNumMonitors: %v, currentNumMonitors: %v", prevNumMonitors, currentNumMonitors)
	var options applyOptions
	if currentNumMonitors < prevNumMonitors && currentNumMonitors >= 1 {
		// 连接状态的显示器数量减少了，并且现存一个及以上连接状态的显示器。
		logger.Debug("should disable crtc in apply")
		if options == nil {
			options = applyOptions{}
		}
		options[optionDisableCrtc] = true
	}

	//m.initFillModes()
	oldMonitorsID := m.monitorsId
	newMonitorsID := m.getMonitorsId()
	if newMonitorsID != oldMonitorsID && newMonitorsID != "" {
		logger.Debug("new monitors id:", newMonitorsID)
		m.markClean()
		// 接入新屏幕点亮屏幕
		go func() {
			// NOTE: applyDisplayConfig 必须在另外一个 goroutine 中进行。
			err := m.applyDisplayConfig(m.DisplayMode, true, options)
			if err != nil {
				logger.Warning(err)
			}
		}()
		m.monitorsId = newMonitorsID
	}
}

func (m *Manager) handleOutputPropertyChanged(ev *randr.OutputPropertyNotifyEvent) {
	logger.Debug("output property changed", ev.Output, ev.Atom)
}

func (m *Manager) handleScreenChanged(ev *randr.ScreenChangeNotifyEvent, cfgTsChanged bool) {
	logger.Debugf("screen changed cfgTs: %v, screen size: %vx%v ", ev.ConfigTimestamp,
		ev.Width, ev.Height)

	m.PropsMu.Lock()
	m.setPropScreenWidth(ev.Width)
	m.setPropScreenHeight(ev.Height)
	m.PropsMu.Unlock()

	if cfgTsChanged {
		logger.Debug("config timestamp changed")
		if !_hasRandr1d2 {

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

	logger.Info("redo map touch screen")
	m.handleTouchscreenChanged()

	if cfgTsChanged {
		m.showTouchscreenDialogs()
	}
}
