package display

import (
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
)

// 在显示器断开或连接时，monitorsId 会改变，重新应用与之相符合的显示配置。
func (m *Manager) updateMonitorsId(options applyOptions) (changed bool) {
	oldMonitorsId := m.monitorsId
	newMonitorsId := m.getMonitorsId()
	logger.Debugf("old monitors id: %v, new monitors id: %v", oldMonitorsId, newMonitorsId)
	if newMonitorsId != oldMonitorsId && newMonitorsId != "" {
		m.monitorsId = newMonitorsId
		logger.Debug("new monitors id:", newMonitorsId)
		m.markClean()
		go func() {
			// NOTE: applyDisplayConfig 必须在另外一个 goroutine 中进行。
			err := m.applyDisplayConfig(m.DisplayMode, true, options)
			if err != nil {
				logger.Warning(err)
			}
		}()
	}
	return false
}

// 在 X 下，显示器属性改变，断开或者连接显示器。
// 在 wayland 下，仅显示器属性改变。
func (m *Manager) handleMonitorChanged(monitorInfo *MonitorInfo) {
	m.updateMonitor(monitorInfo)
	if _useWayland {
		return
	}

	// 后续只在 X 下需要
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
	m.updateMonitorsId(options)
}

// wayland 下连接显示器
func (m *Manager) handleMonitorAdded(monitorInfo *MonitorInfo) {
	err := m.addMonitor(monitorInfo)
	if err != nil {
		logger.Warning(err)
		return
	}
	m.updatePropMonitors()
	m.updateMonitorsId(nil)
}

// wayland 下断开显示器
func (m *Manager) handleMonitorRemoved(monitorId uint32) {
	logger.Debug("monitor removed", monitorId)
	monitor := m.removeMonitor(monitorId)
	if monitor == nil {
		logger.Warning("remove monitor failed, invalid id", monitorId)
		return
	}

	m.handleMonitorConnectedChanged(monitor, false)
	m.updatePropMonitors()
	m.updateMonitorsId(nil)
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
