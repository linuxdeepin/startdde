package display

import (
	"time"

	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
)

// 在显示器断开或连接时，monitorsId 会改变，重新应用与之相符合的显示配置。
func (m *Manager) updateMonitorsId(options applyOptions) (changed bool) {
	m.monitorsIdMu.Lock()
	defer m.monitorsIdMu.Unlock()
	// NOTE: 这个函数可能同时在 X 事件处理和 applyDisplayConfig 的不同 goroutine 中执行，因此需要加锁。

	oldMonitorsId := m.monitorsId
	monitorMap := m.cloneMonitorMap()
	newMonitorsId := getConnectedMonitors(monitorMap).getMonitorsId()
	logger.Debugf("old monitors id: %v, new monitors id: %v", oldMonitorsId, newMonitorsId)
	if newMonitorsId != oldMonitorsId && newMonitorsId != "" {
		m.monitorsId = newMonitorsId
		logger.Debug("monitors id changed, new monitors id:", newMonitorsId)
		m.markClean()

		const delayApplyDuration = 1 * time.Second
		if m.delayApplyTimer == nil {
			m.delayApplyTimer = time.AfterFunc(delayApplyDuration, func() {
				// NOTE: applyDisplayConfig 在非 X 事件处理的另外一个 goroutine 中进行。
				monitorMap := m.cloneMonitorMap()
				monitors := getConnectedMonitors(monitorMap)
				monitorsId := monitors.getMonitorsId()
				logger.Debug("delay call applyDisplayConfig", monitorsId)
				err := m.applyDisplayConfig(m.DisplayMode, monitorsId, monitorMap, true, options)
				if err != nil {
					logger.Warning(err)
				}
				paths := monitors.getPaths()
				logger.Debug("update prop Monitors:", paths)
				m.PropsMu.Lock()
				m.setPropMonitors(paths)
				m.PropsMu.Unlock()
			})
		}
		m.delayApplyTimer.Stop()
		// timer Reset 之前需要 Stop
		m.delayApplyTimer.Reset(delayApplyDuration)
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
	currentNumMonitors := len(m.getConnectedMonitors())
	m.PropsMu.Lock()
	prevNumMonitors := m.prevNumMonitors
	m.prevNumMonitors = currentNumMonitors
	m.PropsMu.Unlock()

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
	width, height := ev.Width, ev.Height
	swapWidthHeightWithRotation(uint16(ev.Rotation), &width, &height)
	logger.Debugf("screen changed cfgTs: %v, rotation:%v, screen size: %vx%v", ev.ConfigTimestamp,
		ev.Rotation, width, height)

	m.PropsMu.Lock()
	m.setPropScreenWidth(width)
	m.setPropScreenHeight(height)
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
