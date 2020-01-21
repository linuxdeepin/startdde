package display

import (
	"pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
)

func (m *Manager) GetInterfaceName() string {
	return dbusInterface
}

func (m *Manager) ApplyChanges() *dbus.Error {
	if !m.HasChanged {
		return nil
	}
	err := m.apply()
	return dbusutil.ToError(err)
}

func (m *Manager) ResetChanges() *dbus.Error {
	if !m.HasChanged {
		return nil
	}

	for _, monitor := range m.monitorMap {
		monitor.resetChanges()
	}

	err := m.apply()
	if err != nil {
		return dbusutil.ToError(err)
	}

	m.setPropHasChanged(false)
	return nil
}

func (m *Manager) SwitchMode(mode byte, name string) *dbus.Error {
	err := m.switchMode(mode, name)
	return dbusutil.ToError(err)
}

func (m *Manager) Save() *dbus.Error {
	err := m.save()
	return dbusutil.ToError(err)
}

func (dpy *Manager) AssociateTouch(outputName, touch string) *dbus.Error {
	err := dpy.associateTouch(outputName, touch)
	return dbusutil.ToError(err)
}

func (m *Manager) ChangeBrightness(raised bool) *dbus.Error {
	err := m.changeBrightness(raised)
	return dbusutil.ToError(err)
}

func (m *Manager) GetBrightness() (map[string]float64, *dbus.Error) {
	return m.Brightness, nil
}

func (m *Manager) ListOutputNames() ([]string, *dbus.Error) {
	var names []string
	monitors := m.getConnectedMonitors()
	for _, monitor := range monitors {
		names = append(names, monitor.Name)
	}
	return names, nil
}

func (m *Manager) ListOutputsCommonModes() ([]ModeInfo, *dbus.Error) {
	monitors := m.getConnectedMonitors()
	if len(monitors) == 0 {
		return nil, nil
	}

	commonSizes := getMonitorsCommonSizes(monitors)
	result := make([]ModeInfo, len(commonSizes))
	for i, size := range commonSizes {
		result[i] = getFirstModeBySize(monitors[0].Modes, size.width, size.height)
	}
	return result, nil
}

func (m *Manager) ModifyConfigName(name, newName string) *dbus.Error {
	err := m.modifyConfigName(name, newName)
	return dbusutil.ToError(err)
}

func (m *Manager) DeleteCustomMode(name string) *dbus.Error {
	err := m.deleteCustomMode(name)
	return dbusutil.ToError(err)
}

func (m *Manager) RefreshBrightness() *dbus.Error {
	for k, v := range m.Brightness {
		err := m.doSetBrightness(v, k)
		if err != nil {
			logger.Warning(err)
		}
	}
	return nil
}

func (m *Manager) Reset() *dbus.Error {
	// TODO
	return nil
}

func (m *Manager) SetAndSaveBrightness(outputName string, value float64) *dbus.Error {
	err := m.doSetBrightness(value, outputName)
	if err == nil {
		m.saveBrightness()
	}
	return dbusutil.ToError(err)
}

func (m *Manager) SetBrightness(outputName string, value float64) *dbus.Error {
	err := m.doSetBrightness(value, outputName)
	return dbusutil.ToError(err)
}

func (m *Manager) SetPrimary(outputName string) *dbus.Error {
	err := m.setPrimary(outputName)
	return dbusutil.ToError(err)
}
