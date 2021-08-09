package display

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"sync"

	x "github.com/linuxdeepin/go-x11-client"

	dbus "github.com/godbus/dbus"
	"pkg.deepin.io/lib/dbusutil"
)

const (
	dbusInterfaceMonitor = dbusInterface + ".Monitor"
)

type Monitor struct {
	m       *Manager
	service *dbusutil.Service
	//crtc    randr.Crtc
	uuid    string
	edid    string //base64编码
	PropsMu sync.RWMutex

	ID           uint32
	Name         string
	Manufacturer string
	Model        string
	Connected    bool

	// dbusutil-gen: equal=nil
	Rotations []uint16
	// dbusutil-gen: equal=nil
	Reflects []uint16
	// dbusutil-gen: equal=nil
	BestMode ModeInfo
	// dbusutil-gen: equal=nil
	Modes []ModeInfo
	// dbusutil-gen: equal=nil
	PreferredModes []ModeInfo
	MmWidth        uint32
	MmHeight       uint32

	Enabled     bool
	X           int16
	Y           int16
	Width       uint16
	Height      uint16
	Rotation    uint16
	Reflect     uint16
	RefreshRate float64

	// dbusutil-gen: equal=nil
	CurrentMode ModeInfo
	modeSetFlag  uint8 // 自定义单屏模式保存

	model        string
	manufacturer string

	backup *MonitorBackup
}

func (m *Monitor) GetInterfaceName() string {
	return dbusInterfaceMonitor
}

func (m *Monitor) getPath() dbus.ObjectPath {
	return dbus.ObjectPath(dbusPath + "/Monitor_" + strconv.Itoa(int(m.ID)))
}

type MonitorBackup struct {
	Enabled  bool
	Mode     ModeInfo
	X, Y     int16
	Reflect  uint16
	Rotation uint16
}

func (m *Monitor) markChanged() {
	m.m.setPropHasChanged(true)
	if m.backup == nil {
		m.backup = &MonitorBackup{
			Enabled:  m.Enabled,
			Mode:     m.CurrentMode,
			X:        m.X,
			Y:        m.Y,
			Reflect:  m.Reflect,
			Rotation: m.Rotation,
		}
	}
}

func (m *Monitor) Enable(enabled bool) *dbus.Error {
	if m.Enabled == enabled {
		return nil
	}

	m.markChanged()
	if enabled == true {
		if m.modeSetFlag == DisplayModeExtendOnlyOne {
			m.modeSetFlag = DisplayModeCustom
			logger.Debug("Enable DisplayModeExtendOnlyOne->DisplayModeCustom ", m.modeSetFlag)

		}
		if m.modeSetFlag == DisplayModeMirrorOnlyOne {
			m.modeSetFlag = DisplayModeCustom
			logger.Debug("Enable DisplayModeMirrorOnlyOne->DisplayModeCustom ", m.modeSetFlag)
		}

	}
	if enabled == false {

		if m.m.DisplayMode == DisplayModeCustom {
			a, _ := m.m.GetRealDisplayMode()
			if a == DisplayModeExtend {
				m.modeSetFlag = DisplayModeExtendOnlyOne
				logger.Debug("Disable DisplayModeCustom->DisplayModeExtendOnlyOne ", m.modeSetFlag)
			}
			if a == DisplayModeMirror {
				m.modeSetFlag = DisplayModeMirrorOnlyOne
				logger.Debug("Disable DisplayModeCustom->DisplayModeMirrorOnlyOne ", m.modeSetFlag)
			}
		}
	}
	m.enable(enabled)
	return nil
}

func (m *Monitor) enable(enabled bool) {
	m.setPropEnabled(enabled)
}

func (m *Monitor) SetMode(mode uint32) *dbus.Error {
	if m.CurrentMode.Id == mode {
		return nil
	}

	var newMode ModeInfo
	modeFound := false
	for _, modeInfo := range m.Modes {
		if modeInfo.Id == mode {
			newMode = modeInfo
			modeFound = true
			break
		}
	}

	if !modeFound {
		return dbusutil.ToError(fmt.Errorf("invalid mode id %d", mode))
	}

	m.m.mutiMonitorsPos = m.m.getMonitorsPosition()
	m.markChanged()
	m.setModeNoProp(newMode)
	return nil
}


func (m *Monitor) setModeNoProp(mode ModeInfo) {
	m.PropsMu.Lock()
	m.setCurrentMode(mode)

	width := mode.Width
	height := mode.Height

	if needSwapWidthHeight(m.Rotation) {
		width, height = height, width
	}

	m.setWidth(width)
	m.setHeight(height)
	m.setRefreshRate(mode.Rate)
	m.PropsMu.Unlock()
}

func (m *Monitor) setMode(mode ModeInfo) {
	m.PropsMu.Lock()
	m.setPropCurrentMode(mode)

	width := mode.Width
	height := mode.Height

	if needSwapWidthHeight(m.Rotation) {
		width, height = height, width
	}

	m.setPropWidth(width)
	m.setPropHeight(height)
	m.setPropRefreshRate(mode.Rate)
	m.PropsMu.Unlock()
}

//func (m *Monitor) getBestMode(manager *Manager, outputInfo *randr.GetOutputInfoReply) ModeInfo {
//	mode := manager.getModeInfo(outputInfo.GetPreferredMode())
//	if mode.Id == 0 && len(m.Modes) > 0 {
//		mode = m.Modes[0]
//	} else if mode.Id != 0 {
//		mode = getFirstModeBySize(m.Modes, mode.Width, mode.Height)
//	}
//	return mode
//}

func (m *Monitor) selectMode(width, height uint16, rate float64) ModeInfo {
	mode, ok := getFirstModeBySizeRate(m.Modes, width, height, rate)
	if ok {
		return mode
	}
	mode, ok = getFirstModeBySize(m.Modes, width, height)
	if ok {
		return mode
	}
	return m.BestMode
}

func (m *Monitor) SetModeBySize(width, height uint16) *dbus.Error {
	mode, ok := getFirstModeBySize(m.Modes, width, height)
	if !ok {
		return dbusutil.ToError(errors.New("not found match mode"))
	}
	return m.SetMode(mode.Id)
}

func (m *Monitor) SetRefreshRate(value float64) *dbus.Error {
	if m.Width == 0 || m.Height == 0 {
		return dbusutil.ToError(errors.New("width or height is 0"))
	}
	mode, ok := getFirstModeBySizeRate(m.Modes, m.Width, m.Height, value)
	if !ok {
		return dbusutil.ToError(errors.New("not found match mode"))
	}
	return m.SetMode(mode.Id)
}

func getFirstModeBySize(modes []ModeInfo, width, height uint16) (ModeInfo, bool) {
	for _, modeInfo := range modes {
		if modeInfo.Width == width && modeInfo.Height == height {
			return modeInfo, true
		}
	}
	return ModeInfo{}, false
}

func getFirstModeBySizeRate(modes []ModeInfo, width, height uint16, rate float64) (ModeInfo, bool) {
	for _, modeInfo := range modes {
		if modeInfo.Width == width && modeInfo.Height == height &&
			math.Abs(modeInfo.Rate-rate) <= 0.01 {
			return modeInfo, true
		}
	}
	return ModeInfo{}, false
}

func (m *Monitor) SetPosition(X, y int16) *dbus.Error {
	if m.X == X && m.Y == y {
		return nil
	}

	m.markChanged()
	m.PropsMu.Lock()
	m.setX(X)
	m.setY(y)
	m.PropsMu.Unlock()
	mode, _ := m.m.GetRealDisplayMode()
	if mode == DisplayModeOnlyOne {
		if X == 0 && y == 0 {
			m.modeSetFlag = DisplayModeMirrorOnlyOne
		} else {
			m.modeSetFlag = DisplayModeExtendOnlyOne
		}

	}
	m.m.mutiMonitorsPos = MonitorsUnknow

	//m.setPosition(X, y)
	return nil
}

func (m *Monitor) setPosition(X, y int16) {
	m.PropsMu.Lock()
	m.setPropX(X)
	m.setPropY(y)
	m.PropsMu.Unlock()
}

func (m *Monitor) SetReflect(value uint16) *dbus.Error {
	if m.Reflect == value {
		return nil
	}
	m.markChanged()
	m.setPropReflect(value)
	return nil
}

func (m *Monitor) setReflect(value uint16) {
	m.PropsMu.Lock()
	m.setPropReflect(value)
	m.PropsMu.Unlock()
}

func (m *Monitor) SetRotation(value uint16) *dbus.Error {
	if m.Rotation == value {
		return nil
	}
	m.markChanged()
	m.setRotationNoProp(value)
	return nil
}

func (m *Monitor) setRotationNoProp(value uint16) {
	m.PropsMu.Lock()
	width := m.CurrentMode.Width
	height := m.CurrentMode.Height

	if needSwapWidthHeight(value) {
		width, height = height, width
		for _, t := range m.m.monitorMap {
			if t.Name == m.Name {
				t.Width = uint16(width)
				t.Height = uint16(height)
				logger.Debug("setRotationNoProp 90/270", t.Width, t.Height)

			}
		}
	}

	m.setRotationOnly(value)
	m.setWidth(width)
	m.setHeight(height)
	m.PropsMu.Unlock()
}

func (m *Monitor) setRotation(value uint16) {
	m.PropsMu.Lock()
	width := m.CurrentMode.Width
	height := m.CurrentMode.Height

	if needSwapWidthHeight(value) {
		width, height = height, width
	}

	m.setPropRotation(value)
	m.setPropWidth(width)
	m.setPropHeight(height)
	m.PropsMu.Unlock()
}

func (m *Monitor) resetChanges() {
	if m.backup == nil {
		return
	}

	logger.Debug("restore from backup", m.ID)
	b := m.backup
	m.setPropEnabled(b.Enabled)
	m.setPropX(b.X)
	m.setPropY(b.Y)
	m.setPropRotation(b.Rotation)
	m.setPropReflect(b.Reflect)

	m.setPropCurrentMode(b.Mode)
	m.setPropWidth(b.Mode.Width)
	m.setPropHeight(b.Mode.Height)
	m.setPropRefreshRate(b.Mode.Rate)

	m.backup = nil
}

//func getRandrStatusStr(status uint8) string {
//	switch status {
//	case randr.SetConfigSuccess:
//		return "success"
//	case randr.SetConfigFailed:
//		return "failed"
//	case randr.SetConfigInvalidConfigTime:
//		return "invalid config time"
//	case randr.SetConfigInvalidTime:
//		return "invalid time"
//	default:
//		return fmt.Sprintf("unknown status %d", status)
//	}
//}

//func (m *Monitor) applyConfig(cfg crtcConfig) error {
//	m.PropsMu.RLock()
//	name := m.Name
//	m.PropsMu.RUnlock()
//	logger.Debugf("applyConfig output: %v %v", m.ID, name)
//
//	m.m.PropsMu.RLock()
//	cfgTs := m.m.configTimestamp
//	m.m.PropsMu.RUnlock()
//
//	logger.Debugf("setCrtcConfig crtc: %v, cfgTs: %v, x: %v, y: %v,"+
//		" mode: %v, rotation|reflect: %v, outputs: %v",
//		cfg.crtc, cfgTs, cfg.x, cfg.y, cfg.mode, cfg.rotation, cfg.outputs)
//	setCfg, err := randr.SetCrtcConfig(m.m.xConn, cfg.crtc, 0, cfgTs,
//		cfg.x, cfg.y, cfg.mode, cfg.rotation,
//		cfg.outputs).Reply(m.m.xConn)
//	if err != nil {
//		return err
//	}
//	if setCfg.Status != randr.SetConfigSuccess {
//		err = fmt.Errorf("failed to configure crtc %v: %v",
//			cfg.crtc, getRandrStatusStr(setCfg.Status))
//		return err
//	}
//	return nil
//}

func toMonitorConfigs(monitors []*Monitor, primary string) []*MonitorConfig {
	found := false
	result := make([]*MonitorConfig, len(monitors))
	for i, m := range monitors {
		cfg := m.toConfig()
		if !found && m.Name == primary {
			cfg.Primary = true
			found = true
		}
		result[i] = cfg
	}
	return result
}

func (m *Monitor) toConfig() *MonitorConfig {
	return &MonitorConfig{
		UUID:        m.uuid,
		Name:        m.Name,
		Enabled:     m.Enabled,
		X:           m.X,
		Y:           m.Y,
		Width:       m.Width,
		Height:      m.Height,
		Rotation:    m.Rotation,
		Reflect:     m.Reflect,
		RefreshRate: m.RefreshRate,
	}
}

func (m *Monitor) getRect() x.Rectangle {
	return x.Rectangle{
		X:      m.X,
		Y:      m.Y,
		Width:  m.Width,
		Height: m.Height,
	}
}

type Monitors []*Monitor

func (monitors Monitors) GetByName(name string) *Monitor {
	for _, monitor := range monitors {
		if monitor.Name == name {
			return monitor
		}
	}
	return nil
}
