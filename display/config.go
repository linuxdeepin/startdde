package display

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"pkg.deepin.io/lib/log"
	"pkg.deepin.io/lib/xdg/basedir"
)

// 目前最新配置文件版本
const configVersion = "5.0"

var (
	// 旧版本配置文件，~/.config/deepin/startdde/display.json
	configFile string
	// 目前最新版本配置文件， ~/.config/deepin/startdde/display_v5.json
	configFile_v5 string
	// ~/.config/deepin/startdde/config.version
	configVersionFile string
	// 内置显示器配置文件，~/.config/deepin/startdde/builtin-monitor
	builtinMonitorConfigFile string
	userConfigFile           string
)

func init() {
	cfgDir := filepath.Join(basedir.GetUserConfigDir(), "deepin/startdde")
	configFile = filepath.Join(cfgDir, "display.json")
	configFile_v5 = filepath.Join(cfgDir, "display_v5.json")
	configVersionFile = filepath.Join(cfgDir, "config.version")
	builtinMonitorConfigFile = filepath.Join(cfgDir, "builtin-monitor")
	userConfigFile = filepath.Join(cfgDir, "display-user.json")
}

type ConfigV5 map[string]*ScreenConfigV5

type ConfigV6 struct {
	ConfigV5 ConfigV5
	FillMode *FillModeConfigs
}

type FillModeConfigs struct {
	FillModeMap map[string]string
}

type ScreenConfigV5 struct {
	Mirror  *ModeConfigsV5    `json:",omitempty"`
	Extend  *ModeConfigsV5    `json:",omitempty"`
	OnlyOne *ModeConfigsV5    `json:",omitempty"`
	Single  *SingleModeConfig `json:",omitempty"`
}

type ModeConfigsV5 struct {
	Monitors []*MonitorConfigV5
}

type SingleModeConfig struct {
	// 这里其实不能用 Monitors，因为是单数
	Monitor                *MonitorConfigV5 `json:"Monitors"` // 单屏时,该配置文件中色温相关数据未生效;增加json的tag是为了兼容之前配置文件
	ColorTemperatureMode   int32
	ColorTemperatureManual int32
}

func (s *ScreenConfigV5) getMonitorConfigs(mode uint8) []*MonitorConfigV5 {
	switch mode {
	case DisplayModeMirror:
		if s.Mirror == nil {
			return nil
		}
		return s.Mirror.Monitors

	case DisplayModeExtend:
		if s.Extend == nil {
			return nil
		}
		return s.Extend.Monitors

	case DisplayModeOnlyOne:
		if s.OnlyOne == nil {
			return nil
		}
		return s.OnlyOne.Monitors
	}

	return nil
}

func (s *ScreenConfigV5) getModeConfigs(mode uint8) *ModeConfigsV5 {
	switch mode {
	case DisplayModeMirror:
		if s.Mirror == nil {
			s.Mirror = &ModeConfigsV5{}
		}
		return s.Mirror

	case DisplayModeExtend:
		if s.Extend == nil {
			s.Extend = &ModeConfigsV5{}
		}
		return s.Extend

	case DisplayModeOnlyOne:
		if s.OnlyOne == nil {
			s.OnlyOne = &ModeConfigsV5{}
		}
		return s.OnlyOne
	}

	return nil
}

func getMonitorConfigByUuid(configs []*MonitorConfigV5, uuid string) *MonitorConfigV5 {
	for _, mc := range configs {
		if mc.UUID == uuid {
			return mc
		}
	}
	return nil
}

func getMonitorConfigPrimary(configs []*MonitorConfigV5) *MonitorConfigV5 { //unused
	for _, mc := range configs {
		if mc.Primary {
			return mc
		}
	}
	return &MonitorConfigV5{}
}

func setMonitorConfigsPrimary(configs []*MonitorConfigV5, uuid string) {
	for _, mc := range configs {
		if mc.UUID == uuid {
			mc.Primary = true
		} else {
			mc.Primary = false
		}
	}
}

func updateMonitorConfigsName(configs SysMonitorConfigs, monitorMap map[uint32]*Monitor) {
	for _, mc := range configs {
		for _, m := range monitorMap {
			if mc.UUID == m.uuid {
				mc.Name = m.Name
				break
			}
		}
	}
}

func (s *ScreenConfigV5) setMonitorConfigs(mode uint8, configs []*MonitorConfigV5) {
	switch mode {
	case DisplayModeMirror:
		if s.Mirror == nil {
			s.Mirror = &ModeConfigsV5{}
		}
		s.Mirror.Monitors = configs

	case DisplayModeExtend:
		if s.Extend == nil {
			s.Extend = &ModeConfigsV5{}
		}
		s.Extend.Monitors = configs

	case DisplayModeOnlyOne:
		s.setMonitorConfigsOnlyOne(configs)
	}
}

func (s *ScreenConfigV5) setModeConfigs(mode uint8, colorTemperatureMode int32, colorTemperatureManual int32, monitorConfig []*MonitorConfigV5) {
	s.setMonitorConfigs(mode, monitorConfig)
	cfg := s.getModeConfigs(mode)
	for _, monitorConfig := range cfg.Monitors {
		if monitorConfig.Enabled {
			monitorConfig.ColorTemperatureMode = colorTemperatureMode
			monitorConfig.ColorTemperatureManual = colorTemperatureManual
		}
	}
}

func (s *ScreenConfigV5) setMonitorConfigsOnlyOne(configs []*MonitorConfigV5) {
	if s.OnlyOne == nil {
		s.OnlyOne = &ModeConfigsV5{}
	}
	oldConfigs := s.OnlyOne.Monitors
	var newConfigs []*MonitorConfigV5
	for _, cfg := range configs {
		if !cfg.Enabled {
			oldCfg := getMonitorConfigByUuid(oldConfigs, cfg.UUID)
			if oldCfg != nil {
				// 不设置 X,Y 是因为它们总是 0
				cfg.Width = oldCfg.Width
				cfg.Height = oldCfg.Height
				cfg.RefreshRate = oldCfg.RefreshRate
				cfg.Rotation = oldCfg.Rotation
				cfg.Reflect = oldCfg.Reflect
			} else {
				continue
			}
		}
		newConfigs = append(newConfigs, cfg)
	}
	s.OnlyOne.Monitors = newConfigs
}

type MonitorConfigV5 struct {
	UUID        string
	Name        string
	Enabled     bool
	X           int16
	Y           int16
	Width       uint16
	Height      uint16
	Rotation    uint16
	Reflect     uint16
	RefreshRate float64
	Brightness  float64
	Primary     bool

	ColorTemperatureMode   int32
	ColorTemperatureManual int32
}

func loadConfigV5(filename string) (ConfigV5, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var c ConfigV5
	err = json.Unmarshal(data, &c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func loadConfigV6(filename string) (ConfigV6, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return ConfigV6{}, err
	}

	var c ConfigV6
	err = json.Unmarshal(data, &c)
	if err != nil {
		return ConfigV6{}, err
	}

	if c.FillMode == nil {
		c.FillMode = &FillModeConfigs{}
	}

	// 存在，没有V6的情况，只有V5,将此时数据读取存到V6
	if c.ConfigV5 == nil {
		var configV5 ConfigV5
		err = json.Unmarshal(data, &configV5)
		if err != nil {
			return ConfigV6{}, err
		}

		c.ConfigV5 = configV5
	}
	return c, nil
}

// 从文件加载配置
func loadConfig(m *Manager) (config ConfigV5) {
	cfgVer, err := getConfigVersion(configVersionFile)
	if err == nil {
		//3.3配置文件转换
		if cfgVer == "3.3" {
			cfg0, err := loadConfigV3_3(configFile)
			if err == nil {
				config = cfg0.toConfig(m)
			} else if !os.IsNotExist(err) {
				logger.Warning(err)
			}
		} else if cfgVer == "4.0" { //4.0配置文件转换
			cfg0, err := loadConfigV4(configFile)
			if err == nil {
				config = cfg0.toConfig(m)
			} else if !os.IsNotExist(err) {
				logger.Warning(err)
			}
		}
	} else if !os.IsNotExist(err) {
		logger.Warning(err)
	}

	if len(config) == 0 {
		configV6, err := loadConfigV6(configFile_v5)
		if err != nil {
			// 加载 v5 和 v6 配置文件都失败
			config = make(ConfigV5)
			//配置文件为空，且当前模式为自定义，则设置当前模式为复制模式
			if m.DisplayMode == DisplayModeCustom {
				m.DisplayMode = DisplayModeMirror
			}
			if !os.IsNotExist(err) {
				logger.Warning(err)
			}
			m.configV6.ConfigV5 = config
			m.configV6.FillMode = &FillModeConfigs{}
		} else {
			// 加载 v5 或 v6 配置文件成功
			config = configV6.ConfigV5
			m.configV6.FillMode = configV6.FillMode
		}
		if m.configV6.FillMode.FillModeMap == nil {
			m.configV6.FillMode.FillModeMap = make(map[string]string)
		}
	} else {
		// 加载 v5 之前配置文件成功
		m.configV6.FillMode = &FillModeConfigs{
			FillModeMap: make(map[string]string),
		}
	}

	if logger.GetLogLevel() == log.LevelDebug {
		logger.Debug("load config:", spew.Sdump(config))
	}
	logger.Debugf("loadConfig fillMode: %#v", m.configV6.FillMode)
	return
}

func (c ConfigV6) save(filename string) error {
	var data []byte
	var err error
	if logger.GetLogLevel() == log.LevelDebug {
		data, err = json.MarshalIndent(c, "", "    ")
		if err != nil {
			return err
		}
	} else {
		data, err = json.Marshal(c)
		if err != nil {
			return err
		}
	}

	err = ioutil.WriteFile(filename, data, 0644)
	if err != nil {
		return err
	}
	return nil
}

func loadBuiltinMonitorConfig(filename string) (string, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

func saveBuiltinMonitorConfig(filename, name string) error {
	dir := filepath.Dir(filename)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filename, []byte(name), 0644)
	if err != nil {
		return err
	}
	return nil
}
