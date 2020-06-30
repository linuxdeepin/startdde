package display

import (
	"math"
	"strings"

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

	oldMonitorsId := m.monitorsId
	newMonitorsId := m.getMonitorsId()
	if newMonitorsId != oldMonitorsId {
		logger.Debug("new monitors id:", newMonitorsId)
		m.markClean()
		m.applyDisplayMode()
		m.monitorsId = newMonitorsId
		if m.DisplayMode == DisplayModeExtend {
			m.setHighPriorityMonitorAsPrimaryForExtend(newMonitorsId)
		}
		go func() {
			err := m.refreshDisplays()
			if err != nil {
				logger.Warning(err)
			}
		}()
	}
}

func (m *Manager) setHighPriorityMonitorAsPrimaryForExtend(newMonitorsId string) {
	monitors := strings.Split(newMonitorsId, ",")
	if len(monitors) >= 2 {
		logger.Info("new monitors is ", monitors)
		monitorName, monitorType := m.getHighPriorityMonitorName()
		monitor, err := m.getOutputPrimary()
		if err != nil {
			return
		}
		logger.Debug("get high priority monitor is ", monitorName, ",monitor type is ", monitorType, ",cur output monitor is ", m.monitorMap[monitor].Name)
		if monitorName != "" && !strings.Contains(m.monitorMap[monitor].Name, monitorType) {
			err := m.setPrimary(monitorName)
			if err != nil {
				logger.Warning(err)
			}
		}
	}
}

func (m *Manager) getHighPriorityMonitorName() (monitorName, monitorType string) {
	type Priority struct {
		name     string
		priority uint32
	}
	// the small 'priority' the value,the higher the priority
	priorities := []Priority{
		{name: "eDP", priority: 1},
		{name: "DP", priority: 2},
		{name: "HDMI", priority: 3},
		{name: "DVI", priority: 4},
		{name: "VGA", priority: 5},
	}
	monitorPriority := make([]Priority, 0)
	monitors := m.getConnectedMonitors()
	for _, monitor := range monitors {
		for _, standardMonitor := range priorities {
			if strings.Contains(monitor.Name, standardMonitor.name) {
				monitorPriority = append(monitorPriority, Priority{name: monitor.Name, priority: standardMonitor.priority})
				break
			}
		}
	}

	var highPriority uint32 = math.MaxUint32
	if len(monitorPriority) >= 1 {
		for i := 0; i < len(monitorPriority); i++ {
			if monitorPriority[i].priority < highPriority {
				highPriority = monitorPriority[i].priority
				monitorName = monitorPriority[i].name
				mt := strings.Split(monitorName, "-")
				if mt != nil {
					monitorType = mt[0]
				}
			}
		}
	}
	return
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

	if rOutputInfo != nil {
		m.PropsMu.Lock()
		if m.Primary == rOutputInfo.Name {
			m.setPropPrimaryRect(getCrtcRect(crtcInfo))
		}
		m.PropsMu.Unlock()
	}

	if rOutput != 0 {
		m.outputMapMu.Lock()
		monitor := m.monitorMap[rOutput]
		m.outputMapMu.Unlock()
		if monitor != nil {
			logger.Debug("update monitor crtc", monitor.ID, monitor.Name)
			m.updateMonitorCrtcInfo(monitor, crtcInfo)
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
		resources, err := m.getScreenResourcesCurrent()
		if err != nil {
			logger.Warning("failed to get screen resources:", err)
		}
		m.modes = resources.Modes
	}

	m.updateOutputPrimary()

	logger.Info("redo map touch screen")
	m.doMapTouches()
}
