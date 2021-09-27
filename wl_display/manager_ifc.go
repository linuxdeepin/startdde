package display

import (
	"errors"
	"fmt"
	"math"
	"os"

	dbus "github.com/godbus/dbus"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/strv"
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
	if !m.canSwitchMode() {
		logger.Info("Forbidden to switch mode")
		return dbusutil.MakeError(m, "Forbidden to switch mode")
	}

	if len(m.getConnectedMonitors()) < 2 {
		return dbusutil.MakeError(m, "no enough connected monitors for switch mode")
	}

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
		result[i], _ = getFirstModeBySize(monitors[0].Modes, size.width, size.height)
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
	if m.getBrightnessSetter() != "backlight" {
		err := m.doSetBrightness(value, outputName)
		if err == nil {
			m.saveBrightness()
		}
		return dbusutil.ToError(err)
	}

	var step float64 = 0.004
	var times float64
	var br float64
	var err error
	//　规避rt背光芯片在低亮度下设置出现频闪问题,将调节步长设置为0.004,并在0.1 -0.3亮度区间采用多次调节
	m.brightnessMapMu.Lock()
	v, ok := m.Brightness[outputName]
	m.brightnessMapMu.Unlock()
	if !ok {
		v = 1.0
	}

	if v > value {
		step = -step
	}

	times = math.Abs((v - value) / step)
	if times == 0 {
		return nil
	}
	if v <= 0.3 && value <= 0.3 {
		for i := 1; i <= int(times); i++ {
			br = v + step*float64(i)
			if br > 1.0 {
				br = 1.0
			}
			if br < 0.1 {
				br = 0.1
			}
			logger.Info("[changeBrightness] will set to:", outputName, br)
			err = m.doSetBrightness(br, outputName)
			if err == nil {
				m.saveBrightness()
			} else {
				return dbusutil.ToError(err)
			}
		}
	} else {
		err = m.doSetBrightness(value, outputName)
		if err == nil {
			m.saveBrightness()
		}
	}
	return nil
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

func (m *Manager) CanSetBrightness(outputName string) (bool, *dbus.Error) {
	if outputName == "" {
		return false, dbusutil.ToError(errors.New("monitor Name is err"))
	}

	//如果是龙芯集显，且不是内置显示器，则不支持调节亮度
	// if os.Getenv("CAN_SET_BRIGHTNESS") == "N" {
	// 	if m.builtinMonitor == nil || m.builtinMonitor.Name != outputName {
	// 		return false, nil
	// 	}
	// }
	return true, nil
}

func (m *Manager) CanSwitchMode() (bool, *dbus.Error) {
	return m.canSwitchMode(), nil
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

func (m *Manager) GetCustomDisplayMode() (uint8, *dbus.Error) {
	realMode, _ := m.GetRealDisplayMode()
	mode := m.customDisplayMode
	if realMode != DisplayModeOnlyOne {
		if realMode != mode {
			mode = realMode
		}
	}
	return mode, nil
}

func (m *Manager) SetCustomDisplayMode(mode uint8) *dbus.Error {
	m.customDisplayMode = mode
	m.settings.SetInt("custom-display-mode", int32(mode))
	logger.Debug("custom display mode ", m.customDisplayMode)
	return nil
}
