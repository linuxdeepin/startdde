package display

import (
	"encoding/json"
	"io/ioutil"
)

type ConfigV4 map[string]*ScreenConfig_V4

type ScreenConfig_V4 struct {
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

func loadConfigV4(filename string) (ConfigV4, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var c ConfigV4
	err = json.Unmarshal(data, &c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c ConfigV4) toConfig(m *Manager) Config {
	newConfig := make(Config)

	for id, sc := range c {
		cfgKey := parseConfigKey(id)
		jId := cfgKey.getJoinedId()
		// 单屏幕，可设置分辨率
		if len(cfgKey.idFields) == 1 {
			//配置文件中保存的可能为空值
			if sc.Single != nil {
				//把亮度,色温写入配置文件
				sc.Single.Brightness = m.Brightness[sc.Single.Name]
				newConfig[jId] = &ScreenConfig{
					Mirror:  nil,
					Extend:  nil,
					OnlyOne: nil,
					Single: &SingleModeConfig{
						Monitors:               sc.Single,
						ColorTemperatureMode:   m.gsColorTemperatureMode,
						ColorTemperatureManual: m.gsColorTemperatureManual,
					},
				}
			}
		} else {
			screenCfg := newConfig[id]
			if screenCfg == nil {
				screenCfg = &ScreenConfig{}
				newConfig[id] = screenCfg
			}
			sc.toModeConfigs(screenCfg, m)
		}
	}
	return newConfig
}

func (sc *ScreenConfig_V4) toModeConfigs(screenCfg *ScreenConfig, m *Manager) {
	if sc.OnlyOne != nil {
		result := make([]*MonitorConfig, 1)
		for idx, monitor := range sc.OnlyOne.Monitors {
			monitor.Brightness = m.Brightness[monitor.Name]
			result[idx] = monitor
		}
		screenCfg.setModeConfigs(DisplayModeOnlyOne, m.gsColorTemperatureMode, m.gsColorTemperatureManual, result)
	}

	//默认自定义数据,自定义没数据就用复制 扩展的数据
	if sc.Custom != nil {
		result := make([]*MonitorConfig, 2)
		for _, custom := range sc.Custom {
			for idx, monitor := range custom.Monitors {
				monitor.Brightness = m.Brightness[monitor.Name]
				result[idx] = monitor
			}
		}

		if result[0].X == result[1].X {
			screenCfg.setModeConfigs(DisplayModeMirror, m.gsColorTemperatureMode, m.gsColorTemperatureManual, result)
			//如果升级之前是自定义模式.重新判断是拆分/合并模式
			if m.DisplayMode == DisplayModeCustom {
				m.setDisplayMode(DisplayModeMirror)
			}
		} else {
			screenCfg.setModeConfigs(DisplayModeExtend, m.gsColorTemperatureMode, m.gsColorTemperatureManual, result)
			//如果升级之前是自定义模式.重新判断是拆分/合并模式
			if m.DisplayMode == DisplayModeCustom {
				m.setDisplayMode(DisplayModeExtend)
			}
		}
		return
	} else {
		if m.DisplayMode == DisplayModeCustom {
			m.setDisplayMode(DisplayModeMirror)
		}
	}

	if sc.Mirror != nil {
		result := make([]*MonitorConfig, len(sc.Mirror.Monitors))
		for idx, monitor := range sc.Mirror.Monitors {
			monitor.Brightness = m.Brightness[monitor.Name]
			result[idx] = monitor
		}
		screenCfg.setModeConfigs(DisplayModeMirror, m.gsColorTemperatureMode, m.gsColorTemperatureManual, result)
	}

	if sc.Extend != nil {
		result := make([]*MonitorConfig, len(sc.Extend.Monitors))
		for idx, monitor := range sc.Extend.Monitors {
			monitor.Brightness = m.Brightness[monitor.Name]
			result[idx] = monitor
		}
		screenCfg.setModeConfigs(DisplayModeExtend, m.gsColorTemperatureMode, m.gsColorTemperatureManual, result)
	}
}
