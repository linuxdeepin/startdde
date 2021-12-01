package display

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	dbus "github.com/godbus/dbus"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/strv"
	"pkg.deepin.io/lib/xdg/basedir"
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

func (m *Manager) AssociateTouch(outputName, touchSerial string) *dbus.Error {
	var touchUUID string
	for _, v := range m.Touchscreens {
		if v.Serial == touchSerial {
			touchUUID = v.uuid
			break
		}
	}

	if touchUUID == "" {
		return dbusutil.ToError(errors.New("touchscreen not exists"))
	}

	monitor := m.getConnectedMonitors().GetByName(outputName)
	if monitor == nil {
		return dbusutil.ToError(errors.New("monitor not exists"))
	}

	err := m.associateTouch(monitor, touchUUID, false)
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
	return dbusutil.ToError(errors.New("obsoleted interface"))
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
	can, _ := m.CanSetBrightness(outputName)
	if !can {
		return dbusutil.ToError(fmt.Errorf("the port %s cannot set brightness", outputName))
	}
	err := m.doSetBrightness(value, outputName)
	if err == nil {
		m.saveBrightness(outputName, value)
		//保存到配置文件
		err = m.saveConfig()
		m.syncBrightness()
	}
	return dbusutil.ToError(err)
}

func (m *Manager) SetBrightness(outputName string, value float64) *dbus.Error {
	if value > 1 || value < 0 {
		return dbusutil.ToError(fmt.Errorf("the brightness value range is 0-1"))
	}

	can, _ := m.CanSetBrightness(outputName)
	if !can {
		return dbusutil.ToError(fmt.Errorf("the port %s cannot set brightness", outputName))
	}

	err := m.doSetBrightness(value, outputName)
	m.syncBrightness()
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

func (m *Manager) CanSetBrightness(outputName string) (bool, *dbus.Error) {
	if outputName == "" {
		return false, dbusutil.ToError(errors.New("monitor Name is err"))
	}

	//如果是龙芯集显，且不是内置显示器，则不支持调节亮度
	if os.Getenv("CAN_SET_BRIGHTNESS") == "N" {
		if m.builtinMonitor == nil || m.builtinMonitor.Name != outputName {
			return false, nil
		}
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
	m.setPropColorTemperatureMode(adjustMethod)
	m.saveColorTemperatureModeToConfigs(adjustMethod)
	switch adjustMethod {
	case ColorTemperatureModeNormal: // 不调节色温，关闭redshift服务
		controlRedshift("stop") // 停止服务
		resetColorTemp()        // 色温重置
	case ColorTemperatureModeAuto: // 自动模式调节色温 启动服务
		resetColorTemp()
		controlRedshift("start") // 开启服务
	case ColorTemperatureModeManual: // 手动调节色温 关闭服务 调节色温(调用存在之前保存的手动色温值)
		controlRedshift("stop") // 停止服务
		lastManualCCT := m.ColorTemperatureManual
		err := m.SetColorTemperature(lastManualCCT)
		if err != nil {
			return err
		}
	}

	err := m.saveConfig()
	if err != nil {
		return dbusutil.ToError(err)
	}
	return nil
}

func (m *Manager) SetColorTemperature(value int32) *dbus.Error {
	if m.ColorTemperatureMode != ColorTemperatureModeManual {
		return dbusutil.ToError(errors.New("current not manual mode, can not adjust CCT by manual"))
	}
	if value < 1000 || value > 25000 {
		return dbusutil.ToError(errors.New("value out of range"))
	}
	setColorTempOneShot(strconv.Itoa(int(value))) // 手动设置色温
	m.ColorTemperatureManual = value
	err := m.emitPropChangedColorTemperatureManual(value)
	if err != nil {
		return dbusutil.ToError(errors.New("emitPropChangedColorTemperatureManual failed"))
	}

	m.saveColorTemperatureToConfigs(value)

	err = m.saveConfig()
	if err != nil {
		return dbusutil.ToError(err)
	}
	return nil
}

func (m *Manager) GetRealDisplayMode() (uint8, *dbus.Error) {
	monitors := m.getConnectedMonitors()

	mode := DisplayModeUnknow
	var pairs strv.Strv
	for _, m := range monitors {
		if !m.Enabled {
			continue
		}

		pair := fmt.Sprintf("%d,%d", m.X, m.Y)

		// 左上角座标相同，是复制
		if pairs.Contains(pair) {
			mode = DisplayModeMirror
		}

		pairs = append(pairs, pair)
	}

	if mode == DisplayModeUnknow && len(pairs) != 0 {
		if len(pairs) == 1 {
			mode = DisplayModeOnlyOne
		} else {
			mode = DisplayModeExtend
		}
	}

	return mode, nil
}

// saveColorTemperatureToConfigs 保存手动色温值到配置文件，但这里并未保存到文件。
func (m *Manager) saveColorTemperatureToConfigs(colorTemperatureManual int32) {
	monitors := m.getConnectedMonitors()
	screenCfg := m.getScreenConfig()
	if len(monitors) == 1 {
		screenCfg.Single.Monitor = monitors[0].toConfig()
		screenCfg.Single.ColorTemperatureManual = colorTemperatureManual
	} else {
		configs := screenCfg.getModeConfigs(m.DisplayMode)
		for _, monitorConfig := range configs.Monitors {
			if monitorConfig.Enabled {
				monitorConfig.ColorTemperatureManual = colorTemperatureManual
			}
		}
		for _, monitor := range monitors {
			if monitor.Enabled {
				monitor.colorTemperatureManual = colorTemperatureManual
			}
		}
	}
}

// saveColorTemperatureModeToConfigs 保存色温模式到配置文件，但这里并未保存到文件。
func (m *Manager) saveColorTemperatureModeToConfigs(colorTemperatureMode int32) {
	monitors := m.getConnectedMonitors()
	screenCfg := m.getScreenConfig()
	if len(monitors) == 1 {
		screenCfg.Single.Monitor = monitors[0].toConfig()
		screenCfg.Single.ColorTemperatureMode = colorTemperatureMode
	} else {
		configs := screenCfg.getModeConfigs(m.DisplayMode)
		for _, monitorConfig := range configs.Monitors {
			if monitorConfig.Enabled {
				monitorConfig.ColorTemperatureMode = colorTemperatureMode
			}
		}
		for _, monitor := range monitors {
			if monitor.Enabled {
				monitor.colorTemperatureMode = colorTemperatureMode
			}
		}
	}
}

func controlRedshift(action string) {
	_, err := exec.Command("systemctl", "--user", action, "redshift.service").Output()
	if err != nil {
		logger.Warning("failed to ", action, " redshift.service:", err)
	} else {
		logger.Info("success to ", action, " redshift.service")
	}
}

// setColorTempOneShot 调用 redshift 命令设置一次色温
func setColorTempOneShot(colorTemp string) {
	_, err := exec.Command("redshift", "-m", "vidmode", "-O", colorTemp, "-P").Output()
	if err != nil {
		logger.Warning("failed to set current ColorTemperature by redshift.service: ", err)
	} else {
		logger.Info("success to to set current ColorTemperature by redshift.service")
	}
}

// resetColorTemp 调用 redshift 命令重置色温，即删除色温设置。
func resetColorTemp() {
	_, err := exec.Command("redshift", "-m", "vidmode", "-x").Output()
	if err != nil {
		logger.Warning("failed to reset ColorTemperature ", err)
	} else {
		logger.Info("success to reset ColorTemperature")
	}
}

// generateRedshiftConfFile 用来生成 redshift 的配置文件，路径为“~/.config/redshift/redshift.conf”。
// 如果配置文件已经存在，则不生成。
func generateRedshiftConfFile() error {
	controlRedshift("disable")
	configFilePath := basedir.GetUserConfigDir() + "/redshift/redshift.conf"
	_, err := os.Stat(configFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			dir := filepath.Dir(configFilePath)
			err := os.MkdirAll(dir, 0755)
			if err != nil {
				return err
			}
			content := []byte("[redshift]\n" +
				"temp-day=6500\n" + // 自动模式下，白天的色温
				"temp-night=3500\n" + // 自动模式下，夜晚的色温
				"transition=1\n" +
				"gamma=1\n" +
				"location-provider=geoclue2\n" +
				"adjustment-method=vidmode\n" +
				"[vidmode]\n" +
				"screen=0")
			err = ioutil.WriteFile(configFilePath, content, 0644)
			return err
		} else {
			return err
		}
	} else {
		logger.Debug("redshift.conf file exist , don't need create config file")
	}
	return nil
}
