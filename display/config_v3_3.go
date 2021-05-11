package display

import (
	"encoding/json"
	"io/ioutil"
	"sort"
	"strings"
)

type ScreenConfigV3_3 struct {
	Name      string
	Primary   string
	BaseInfos []*MonitorConfiV3_3
}

func (sc *ScreenConfigV3_3) toMonitorConfigs(m *Manager) []*MonitorConfig {
	result := make([]*MonitorConfig, len(sc.BaseInfos))
	var brightness float64
	for idx, bi := range sc.BaseInfos {
		for brightnessName, value := range m.Brightness {
			if brightnessName == bi.Name {
				brightness = value
			}
		}
		primary := bi.Name == sc.Primary
		result[idx] = &MonitorConfig{
			UUID:        bi.UUID,
			Name:        bi.Name,
			Enabled:     bi.Enabled,
			X:           bi.X,
			Y:           bi.Y,
			Width:       bi.Width,
			Height:      bi.Height,
			Rotation:    bi.Rotation,
			Reflect:     bi.Reflect,
			RefreshRate: bi.RefreshRate,
			Brightness:  brightness,
			Primary:     primary,
		}
	}
	return result
}

func (sc *ScreenConfigV3_3) toOtherConfigs(m *Manager) []*MonitorConfig {
	result := make([]*MonitorConfig, len(sc.BaseInfos))
	var brightness float64
	for idx, bi := range sc.BaseInfos {
		for brightnessName, value := range m.Brightness {
			if brightnessName == bi.Name {
				brightness = value
			}
		}
		primary := bi.Name == sc.Primary
		result[idx] = &MonitorConfig{
			UUID:        bi.UUID,
			Name:        bi.Name,
			Enabled:     bi.Enabled,
			X:           bi.X,
			Y:           bi.Y,
			Width:       bi.Width,
			Height:      bi.Height,
			Rotation:    bi.Rotation,
			Reflect:     bi.Reflect,
			RefreshRate: bi.RefreshRate,
			Brightness:  brightness,
			Primary:     primary,
		}
	}
	return result
}

type MonitorConfiV3_3 struct {
	UUID        string // sum md5 of name and modes, for config
	Name        string
	Enabled     bool
	X           int16
	Y           int16
	Width       uint16
	Height      uint16
	Rotation    uint16
	Reflect     uint16
	RefreshRate float64
}

type ConfigV3_3 map[string]*ScreenConfigV3_3

func loadConfigV3_3(filename string) (ConfigV3_3, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var c ConfigV3_3
	err = json.Unmarshal(data, &c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c ConfigV3_3) toConfig(m *Manager) Config {
	newConfig := make(Config)
	var brightness float64
	for id, sc := range c {
		cfgKey := parseConfigKey(id)
		jId := cfgKey.getJoinedId()
		if cfgKey.name == "" {
			// 单屏幕，可设置分辨率
			if len(cfgKey.idFields) == 1 &&
				len(sc.BaseInfos) == 1 {

				bi := sc.BaseInfos[0]
				if bi != nil {
					for brightnessName, value := range m.Brightness {
						if brightnessName == bi.Name {
							brightness = value
						}
					}

					newConfig[jId] = &ScreenConfig{
						Mirror:  nil,
						Extend:  nil,
						OnlyOne: nil,
						Single: &SingleModeConfig{
							Monitor: &MonitorConfig{
								UUID:        bi.UUID,
								Name:        bi.Name,
								Enabled:     bi.Enabled,
								X:           bi.X,
								Y:           bi.Y,
								Width:       bi.Width,
								Height:      bi.Height,
								Rotation:    bi.Rotation,
								Reflect:     bi.Reflect,
								RefreshRate: bi.RefreshRate,
								Brightness:  brightness,
								Primary:     true,
							},
							ColorTemperatureMode:   m.gsColorTemperatureMode,
							ColorTemperatureManual: m.gsColorTemperatureManual,
						},
					}
				}
			}
		} else {
			screenCfg := newConfig[jId]
			if screenCfg == nil {
				screenCfg = &ScreenConfig{}
				newConfig[jId] = screenCfg
			}

			configs := sc.toMonitorConfigs(m)
			//只有合并和拆分模式 只判断两个屏
			if configs[0].X == configs[1].X {
				screenCfg.setModeConfigs(DisplayModeMirror, m.gsColorTemperatureMode, m.gsColorTemperatureManual, configs)
				//如果升级之前是自定义模式.重新判断是拆分/合并模式
				if m.DisplayMode == DisplayModeCustom {
					m.setDisplayMode(DisplayModeMirror)
				}
			} else {
				screenCfg.setModeConfigs(DisplayModeExtend, m.gsColorTemperatureMode, m.gsColorTemperatureManual, configs)
				//如果升级之前是自定义模式.重新判断是拆分/合并模式
				if m.DisplayMode == DisplayModeCustom {
					m.setDisplayMode(DisplayModeExtend)
				}

			}
		}
	}
	return newConfig
}

type configKey struct {
	name     string
	idFields []string
}

func (ck *configKey) getJoinedId() string {
	return strings.Join(ck.idFields, monitorsIdDelimiter)
}

func parseConfigKey(str string) configKey {
	var name string
	var idFields []string
	idx := strings.LastIndex(str, customModeDelim)
	if idx == -1 {
		idFields = strings.Split(str, monitorsIdDelimiter)
	} else {
		name = str[:idx]
		idFields = strings.Split(str[idx+1:], monitorsIdDelimiter)
	}

	sort.Strings(idFields)
	return configKey{
		name:     name,
		idFields: idFields,
	}
}
