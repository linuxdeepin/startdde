package display

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/davecgh/go-spew/spew"
	"pkg.deepin.io/lib/log"
	"pkg.deepin.io/lib/xdg/basedir"
)

const configVersion = "4.0"

var (
	configFile        string
	configVersionFile string
)

func init() {
	cfgDir := filepath.Join(basedir.GetUserConfigDir(), "deepin/startdde")
	configFile = filepath.Join(cfgDir, "display.json")
	configVersionFile = filepath.Join(cfgDir, "config.version")
}

type Config map[string]*ScreenConfig

type ScreenConfig struct {
	Custom  []*CustomModeConfig `json:",omitempty"`
	Mirror  *MirrorModeConfig   `json:",omitempty"`
	Extend  *ExtendModeConfig   `json:",omitempty"`
	OnlyOne *OnlyOneModeConfig  `json:",omitempty"`
	Single  *MonitorConfig      `json:",omitempty"`
}

type CustomModeConfig struct {
	Name     string
	Monitors []*MonitorConfig
}

type MirrorModeConfig struct {
	Monitors []*MonitorConfig
}

type ExtendModeConfig struct {
	Monitors []*MonitorConfig
}

type OnlyOneModeConfig struct {
	Monitors []*MonitorConfig
}

func (s *ScreenConfig) getMonitorConfigs(mode uint8, customName string) []*MonitorConfig {
	switch mode {
	case DisplayModeCustom:
		for _, custom := range s.Custom {
			if custom.Name == customName {
				return custom.Monitors
			}
		}
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

func getMonitorConfigByUuid(configs []*MonitorConfig, uuid string) *MonitorConfig {
	for _, mc := range configs {
		if mc.UUID == uuid {
			return mc
		}
	}
	return nil
}

func setMonitorConfigsPrimary(configs []*MonitorConfig, uuid string) {
	for _, mc := range configs {
		if mc.UUID == uuid {
			mc.Primary = true
		} else {
			mc.Primary = false
		}
	}
}

func updateMonitorConfigsName(configs []*MonitorConfig, monitorMap map[uint32]*Monitor) {
	for _, mc := range configs {
		for _, m := range monitorMap {
			if mc.UUID == m.uuid {
				mc.Name = m.Name
				break
			}
		}
	}
}

func (s *ScreenConfig) setMonitorConfigs(mode uint8, customName string, configs []*MonitorConfig) {
	switch mode {
	case DisplayModeCustom:
		foundName := false
		for _, custom := range s.Custom {
			if custom.Name == customName {
				foundName = true
				custom.Monitors = configs
			}
		}

		// new custom
		if !foundName {
			s.Custom = append(s.Custom, &CustomModeConfig{
				Name:     customName,
				Monitors: configs,
			})
		}

	case DisplayModeMirror:
		if s.Mirror == nil {
			s.Mirror = &MirrorModeConfig{}
		}
		s.Mirror.Monitors = configs

	case DisplayModeExtend:
		if s.Extend == nil {
			s.Extend = &ExtendModeConfig{}
		}
		s.Extend.Monitors = configs

	case DisplayModeOnlyOne:
		s.setMonitorConfigsOnlyOne(configs)
	}
}

func (s *ScreenConfig) setMonitorConfigsOnlyOne(configs []*MonitorConfig) {
	if s.OnlyOne == nil {
		s.OnlyOne = &OnlyOneModeConfig{}
	}
	oldConfigs := s.OnlyOne.Monitors
	var newConfigs []*MonitorConfig
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

type MonitorConfig struct {
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
	Primary     bool
}

func loadConfigV4(filename string) (Config, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var c Config
	err = json.Unmarshal(data, &c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func loadConfig() (config Config) {
	cfgVer, err := getConfigVersion(configVersionFile)
	if err == nil {
		if cfgVer == "3.3" {
			cfg0, err := loadConfigV3_3(configFile)
			if err == nil {
				config = cfg0.toConfig()
			} else if !os.IsNotExist(err) {
				logger.Warning(err)
			}
		}

	} else if !os.IsNotExist(err) {
		logger.Warning(err)
	}

	if len(config) == 0 {
		config, err = loadConfigV4(configFile)
		if err != nil {
			config = make(Config)
			if !os.IsNotExist(err) {
				logger.Warning(err)
			}
		}
	}

	if logger.GetLogLevel() == log.LevelDebug {
		logger.Debug("load config:", spew.Sdump(config))
	}
	return
}

func (c Config) save(filename string) error {
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
