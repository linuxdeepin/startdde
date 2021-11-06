package display

import (
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	sysConfigVersion  = "1.0"
	userConfigVersion = "1.0"
)

type SysRootConfig struct {
	mu       sync.Mutex
	Version  string
	Config   SysConfig
	UpdateAt string
}

func (c *SysRootConfig) copyFrom(newSysConfig *SysRootConfig) {
	c.mu.Lock()

	c.Version = newSysConfig.Version
	c.Config = newSysConfig.Config
	c.UpdateAt = newSysConfig.UpdateAt

	c.mu.Unlock()
}

type SysConfig struct {
	DisplayMode  byte
	Screens      map[string]*SysScreenConfig
	ScaleFactors map[string]float64 // 缩放比例
	FillModes    map[string]string  // key 是特殊的 fillMode Key
	Cache        SysCache
}

type SysCache struct {
	BuiltinMonitor string
	ConnectTime    map[string]time.Time
}

type UserConfig struct {
	Version string
	Screens map[string]UserScreenConfig
}

type UserScreenConfig map[string]*UserMonitorModeConfig

const (
	KeyModeMirror        = "Mirror"
	KeyModeExtend        = "Extend"
	KeyModeOnlyOnePrefix = "OnlyOne-"
	KeySingle            = "Single"
)

type UserMonitorModeConfig struct {
	ColorTemperatureMode   int32
	ColorTemperatureManual int32
	// 以后如果有必要
	// Monitors UserMonitorConfigs
}

func getDefaultUserMonitorModeConfig() *UserMonitorModeConfig {
	return &UserMonitorModeConfig{
		ColorTemperatureMode:   defaultTemperatureMode,
		ColorTemperatureManual: defaultTemperatureManual,
	}
}

func (m *Manager) getUserScreenConfig() UserScreenConfig {
	id := m.getMonitorsId()
	screenCfg := m.userConfig.Screens[id]
	if screenCfg == nil {
		if m.userConfig.Screens == nil {
			m.userConfig.Screens = make(map[string]UserScreenConfig)
		}
		screenCfg = UserScreenConfig{}
		m.userConfig.Screens[id] = screenCfg
	}
	return screenCfg
}

func (usc UserScreenConfig) getMonitorModeConfig(mode byte, uuid string) (cfg *UserMonitorModeConfig) {
	switch mode {
	case DisplayModeMirror:
		return usc[KeyModeMirror]
	case DisplayModeExtend:
		return usc[KeyModeExtend]
	case DisplayModeOnlyOne:
		return usc[KeyModeOnlyOnePrefix+uuid]
	}
	return nil
}

func (usc UserScreenConfig) setMonitorModeConfig(mode byte, uuid string, cfg *UserMonitorModeConfig) {
	switch mode {
	case DisplayModeMirror:
		usc[KeyModeMirror] = cfg
	case DisplayModeExtend:
		usc[KeyModeExtend] = cfg
	case DisplayModeOnlyOne:
		usc[KeyModeOnlyOnePrefix+uuid] = cfg
	}
}

// SysScreenConfig 系统级屏幕配置
// NOTE: Single 可以看作是特殊的显示模式，和 Mirror,Extend 等模式共用 SysMonitorModeConfig 结构可以保持设计上的统一，不必在乎里面有 Monitors
type SysScreenConfig struct {
	Mirror      *SysMonitorModeConfig            `json:",omitempty"`
	Extend      *SysMonitorModeConfig            `json:",omitempty"`
	Single      *SysMonitorModeConfig            `json:",omitempty"`
	OnlyOneMap  map[string]*SysMonitorModeConfig `json:",omitempty"`
	OnlyOneUuid string                           `json:",omitempty"`
}

type SysMonitorModeConfig struct {
	Monitors SysMonitorConfigs
}

type SysMonitorConfig struct {
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
}

type SysMonitorConfigs []*SysMonitorConfig

func (s *SysScreenConfig) getSingleMonitorConfigs() SysMonitorConfigs {
	if s.Single == nil {
		return nil
	}
	return s.Single.Monitors
}

func (s *SysScreenConfig) getMonitorConfigs(mode uint8, uuid string) SysMonitorConfigs {
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
		if uuid == "" {
			return nil
		}
		if s.OnlyOneMap[uuid] == nil {
			return nil
		}
		return s.OnlyOneMap[uuid].Monitors
	}

	return nil
}

func (s *SysScreenConfig) setSingleMonitorConfigs(configs SysMonitorConfigs) {
	if s.Single == nil {
		s.Single = &SysMonitorModeConfig{}
	}
	s.Single.Monitors = configs
}

func (s *SysScreenConfig) setMonitorConfigs(mode uint8, uuid string, configs SysMonitorConfigs) {
	switch mode {
	case DisplayModeMirror:
		if s.Mirror == nil {
			s.Mirror = &SysMonitorModeConfig{}
		}
		s.Mirror.Monitors = configs

	case DisplayModeExtend:
		if s.Extend == nil {
			s.Extend = &SysMonitorModeConfig{}
		}
		s.Extend.Monitors = configs

	case DisplayModeOnlyOne:
		s.setMonitorConfigsOnlyOne(uuid, configs)
	}
}

func (s *SysScreenConfig) setMonitorConfigsOnlyOne(uuid string, configs SysMonitorConfigs) {
	if uuid == "" {
		return
	}
	if s.OnlyOneMap == nil {
		s.OnlyOneMap = make(map[string]*SysMonitorModeConfig)
	}

	if s.OnlyOneMap[uuid] == nil {
		s.OnlyOneMap[uuid] = &SysMonitorModeConfig{}
	}

	if len(configs) > 1 {
		// 去除非使能的 monitor
		var tmpCfg *SysMonitorConfig
		for _, config := range configs {
			if config.Enabled {
				tmpCfg = config
				break
			}
		}
		if tmpCfg != nil {
			configs = SysMonitorConfigs{tmpCfg}
		}
	}

	s.OnlyOneMap[uuid].Monitors = configs
}

func (cfgs SysMonitorConfigs) getByUuid(uuid string) *SysMonitorConfig {
	for _, mc := range cfgs {
		if mc.UUID == uuid {
			return mc
		}
	}
	return nil
}

func (cfgs SysMonitorConfigs) setPrimary(uuid string) {
	for _, mc := range cfgs {
		if mc.UUID == uuid {
			mc.Primary = true
		} else {
			mc.Primary = false
		}
	}
}

// cfgs 和 otherCfgs 之间是否仅亮度不同
// 前置条件：cfgs 和 otherCfgs 不相同
func (cfgs SysMonitorConfigs) onlyBrNotEq(otherCfgs SysMonitorConfigs) bool {
	if len(cfgs) != len(otherCfgs) {
		return false
	}
	// 除了亮度设置为 0， 其他字段都复制
	partCpCfgs := func(cfgs SysMonitorConfigs) SysMonitorConfigs {
		copyCfgs := make(SysMonitorConfigs, len(cfgs))
		for i, cfg := range cfgs {
			cpCfg := &SysMonitorConfig{}
			*cpCfg = *cfg
			cpCfg.Brightness = 0
			copyCfgs[i] = cpCfg
		}
		return copyCfgs
	}

	c1 := partCpCfgs(cfgs)
	c2 := partCpCfgs(otherCfgs)
	// 把亮度都安全的设置为0, 如果 c1 和 c2 是相同的，则可以说明是仅亮度不同。
	if reflect.DeepEqual(c1, c2) {
		return true
	}
	return false
}

func (cfgs SysMonitorConfigs) sort() {
	sort.Slice(cfgs, func(i, j int) bool {
		return strings.Compare(cfgs[i].UUID, cfgs[j].UUID) < 0
	})
}
