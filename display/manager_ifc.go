package display

import (
	"errors"
	"fmt"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
	"os"
	"os/exec"
	"pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
	"strconv"
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

func (m *Manager) CanRotate() (bool, *dbus.Error) {
	if os.Getenv("DEEPIN_DISPLAY_DISABLE_ROTATE") == "1" {
		return false, nil
	}
	return true, nil
}

func (m *Manager) getBuiltinMonitor() *Monitor {
	m.builtinMonitorMu.Lock()
	defer m.builtinMonitorMu.Unlock()
	return m.builtinMonitor
}

func (m *Manager) GetBuiltinMonitor() (string, dbus.ObjectPath, *dbus.Error) {
	builtinMonitor := m.getBuiltinMonitor()
	if builtinMonitor == nil {
		return "", "/", nil
	}

	m.monitorMapMu.Lock()
	_, ok := m.monitorMap[randr.Output(builtinMonitor.ID)]
	m.monitorMapMu.Unlock()
	if !ok {
		return "", "/", dbusutil.ToError(fmt.Errorf("not found monitor %d", builtinMonitor.ID))
	}

	return builtinMonitor.Name, builtinMonitor.getPath(), nil
}

func (m *Manager) SetMethodAdjustCCT(adjustMethod int32) *dbus.Error {
	if adjustMethod > ColorTemperatureModeManual || adjustMethod < ColorTemperatureModeNormal {
		return dbusutil.ToError(errors.New("adjustMethod type out of range, not 0 or 1 or 2"))
	}
	m.ColorTemperatureMode.Set(adjustMethod)
	switch adjustMethod {
	case ColorTemperatureModeNormal: // 不调节色温，关闭redshift服务
		controlRedshift("disable") // 关闭开机启动,停止服务
		resetColorTemp()           // 色温重置
	case ColorTemperatureModeAuto: // 自动模式调节色温 启动服务
		resetColorTemp()
		controlRedshift("enable") // 开机启动,开启服务
	case ColorTemperatureModeManual: // 手动调节色温 关闭服务 调节色温(调用存在之前保存的手动色温值)
		controlRedshift("disable") // 关闭开机启动,停止服务
		lastManualCCT := m.ColorTemperatureManual.Get()
		err := m.SetColorTemperature(lastManualCCT)
		return err
	}
	return nil
}

func (m *Manager) SetColorTemperature(value int32) *dbus.Error {
	if m.ColorTemperatureMode.Get() != ColorTemperatureModeManual {
		return dbusutil.ToError(errors.New("current not manual mode, can not adjust CCT by manual"))
	}
	if value < 1000 || value > 25000 {
		return dbusutil.ToError(errors.New("value out of range"))
	}
	setColorTempOneShot(strconv.Itoa(int(value))) // 手动设置色温
	m.ColorTemperatureManual.Set(value)
	return nil
}

func controlRedshift(action string) {
	_, err := exec.Command("systemctl", "--user", action, "--now", "redshift.service").Output()
	if err != nil {
		logger.Warning("failed to ", action, " redshift.service:", err)
	} else {
		logger.Info("success to ", action, " redshift.service")
	}
}

func setColorTempOneShot(colorTemp string) {
	_, err := exec.Command("redshift", "-m", "vidmode", "-O", colorTemp).Output()
	if err != nil {
		logger.Warning("failed to set current ColorTemperature by redshift.service: ", err)
	} else {
		logger.Info("success to to set current ColorTemperature by redshift.service")
	}
}

func resetColorTemp() {
	_, err := exec.Command("redshift", "-m", "vidmode", "-x").Output()
	if err != nil {
		logger.Warning("failed to reset ColorTemperature ", err)
	} else {
		logger.Info("success to reset ColorTemperature")
	}
}
