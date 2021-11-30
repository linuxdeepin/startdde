package display

import (
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/linuxdeepin/go-lib/xdg/basedir"
)

const (
	// ColorTemperatureModeNone 不调整色温
	ColorTemperatureModeNone int32 = iota
	// ColorTemperatureModeAuto 自动调整色温
	ColorTemperatureModeAuto
	// ColorTemperatureModeManual 手动调整色温
	ColorTemperatureModeManual
)

func isValidColorTempMode(mode int32) bool {
	return mode >= ColorTemperatureModeNone && mode <= ColorTemperatureModeManual
}

// dbus 上导出的方法
func (m *Manager) setColorTempMode(mode int32) error {
	if !isValidColorTempMode(mode) {
		return errors.New("mode out of range, not 0 or 1 or 2")
	}
	m.setPropColorTemperatureMode(mode)
	m.setColorTempModeReal(mode)
	m.saveColorTempModeInCfg(mode)
	return nil
}

func (m *Manager) setColorTempModeReal(mode int32) {
	if _greeterMode {
		return
	}
	switch mode {
	case ColorTemperatureModeNone: // 不调节色温，关闭redshift服务
		controlRedshift("stop") // 停止服务
		resetColorTemp()        // 色温重置
	case ColorTemperatureModeAuto: // 自动模式调节色温 启动服务
		resetColorTemp()
		// NOTE: 不用 start 而要用 restart ，因为可能用户注销之后，redshift 并没有退出，再次登录
		// 之后，如果只 start 它，已经 start 的服务不会再次 start，导致色温设置没有应用成功。
		controlRedshift("restart") // 开启服务
	case ColorTemperatureModeManual: // 手动调节色温 关闭服务 调节色温(调用存在之前保存的手动色温值)
		controlRedshift("stop") // 停止服务
		value := m.ColorTemperatureManual
		setColorTempOneShot(value)
	}
}

// dbus 上导出的方法
func (m *Manager) setColorTempValue(value int32) error {
	if m.ColorTemperatureMode != ColorTemperatureModeManual {
		return errors.New("current not manual mode, can not adjust CCT by manual")
	}
	if !isValidColorTempValue(value) {
		return errors.New("value out of range")
	}
	setColorTempOneShot(value) // 手动设置色温
	m.setPropColorTemperatureManual(value)
	m.saveColorTempValueInCfg(value)
	return nil
}

func isValidColorTempValue(value int32) bool {
	return value >= 1000 && value <= 25000
}

// saveColorTempValueInCfg 保存手动色温值到用户配置
func (m *Manager) saveColorTempValueInCfg(value int32) {
	m.modifySuitableUserMonitorModeConfig(func(cfg *UserMonitorModeConfig) {
		cfg.ColorTemperatureManual = value
	})
	err := m.saveUserConfig()
	if err != nil {
		logger.Warning(err)
	}
}

// saveColorTempModeInCfg 保存色温模式到用户配置
func (m *Manager) saveColorTempModeInCfg(mode int32) {
	m.modifySuitableUserMonitorModeConfig(func(cfg *UserMonitorModeConfig) {
		cfg.ColorTemperatureMode = mode
	})
	err := m.saveUserConfig()
	if err != nil {
		logger.Warning(err)
	}
}

// applyColorTempConfig 应用色温设置
func (m *Manager) applyColorTempConfig(displayMode byte) {
	cfg := m.getSuitableUserMonitorModeConfig(displayMode)
	if cfg == nil {
		cfg = getDefaultUserMonitorModeConfig()
	}
	m.setPropColorTemperatureMode(cfg.ColorTemperatureMode)
	m.setPropColorTemperatureManual(cfg.ColorTemperatureManual)
	m.setColorTempModeReal(m.ColorTemperatureMode)
}

func controlRedshift(action string) {
	// #nosec G204
	_, err := exec.Command("systemctl", "--user", action, "redshift.service").Output()
	if err != nil {
		logger.Warning("failed to ", action, " redshift.service:", err)
	} else {
		logger.Info("success to ", action, " redshift.service")
	}
}

var _setColorTempMu sync.Mutex

// setColorTempOneShot 调用 redshift 命令设置一次色温
func setColorTempOneShot(value int32) {
	_setColorTempMu.Lock()
	defer _setColorTempMu.Unlock()

	valueStr := strconv.Itoa(int(value))
	// #nosec G204
	_, err := exec.Command("redshift", "-m", "vidmode", "-O", valueStr, "-P").Output()
	if err != nil {
		logger.Warning("failed to set current ColorTemperature by redshift command: ", err)
	} else {
		logger.Info("success to to set current ColorTemperature by redshift command")
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
// 配置文件用于自动模式下色温值设置。
// 如果配置文件已经存在，则不生成。
func generateRedshiftConfFile() error {
	controlRedshift("disable")
	configFilePath := basedir.GetUserConfigDir() + "/redshift/redshift.conf"
	_, err := os.Stat(configFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			dir := filepath.Dir(configFilePath)
			err := os.MkdirAll(dir, 0750)
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
			err = ioutil.WriteFile(configFilePath, content, 0600)
			return err
		} else {
			return err
		}
	} else {
		logger.Debug("redshift.conf file exist , don't need create config file")
	}
	return nil
}
