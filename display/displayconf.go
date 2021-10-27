package display

type SysRootConfig struct {
	Version  string
	Config   SysConfig
	UpdateAt string
}

type SysConfig struct {
	BuiltinMonitor string
	DisplayMode    byte
	Screens        map[string]*SysScreenConfig
	ScaleFactors   map[string]float64 // 缩放比例
	FillModes      map[string]string  // key 是特殊的 fillMode Key
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

type SysScreenConfig struct {
	Mirror  *SysMonitorModeConfig `json:",omitempty"`
	Extend  *SysMonitorModeConfig `json:",omitempty"`
	OnlyOne *SysMonitorModeConfig `json:",omitempty"`
	// NOTE: Single 可以看作是特殊的显示模式，共用结构可以保持设计上的统一，不必在乎里面有 Monitors
	Single *SysMonitorModeConfig `json:",omitempty"`
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

func (s *SysScreenConfig) getMonitorConfigs(mode uint8) SysMonitorConfigs {
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

func (s *SysScreenConfig) setSingleMonitorConfigs(configs SysMonitorConfigs) {
	if s.Single == nil {
		s.Single = &SysMonitorModeConfig{}
	}
	s.Single.Monitors = configs
}

func (s *SysScreenConfig) setMonitorConfigs(mode uint8, configs SysMonitorConfigs) {
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
		s.setMonitorConfigsOnlyOne(configs)
	}
}

func (s *SysScreenConfig) setMonitorConfigsOnlyOne(configs SysMonitorConfigs) {
	if s.OnlyOne == nil {
		s.OnlyOne = &SysMonitorModeConfig{}
	}
	oldConfigs := s.OnlyOne.Monitors
	var newConfigs SysMonitorConfigs
	for _, cfg := range configs {
		if !cfg.Enabled {
			oldCfg := oldConfigs.getByUuid(cfg.UUID)
			if oldCfg != nil {
				// 不设置 X,Y 是因为它们总是 0
				cfg.Width = oldCfg.Width
				cfg.Height = oldCfg.Height
				cfg.RefreshRate = oldCfg.RefreshRate
				cfg.Rotation = oldCfg.Rotation
				cfg.Reflect = oldCfg.Reflect
				cfg.Brightness = oldCfg.Brightness
			}
		}
		newConfigs = append(newConfigs, cfg)
	}
	s.OnlyOne.Monitors = newConfigs
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
