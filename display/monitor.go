package display

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/godbus/dbus"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/strv"
)

const (
	dbusInterfaceMonitor = dbusInterface + ".Monitor"
)

const (
	fillModeDefault string = "Full aspect"
	fillModeCenter  string = "Center"
	fillModeFull    string = "Full"
)

type Monitor struct {
	m                 *Manager
	service           *dbusutil.Service
	uuid              string
	PropsMu           sync.RWMutex
	lastConnectedTime time.Time

	ID           uint32
	Name         string
	Connected    bool
	Manufacturer string
	Model        string
	// dbusutil-gen: equal=uint16SliceEqual
	Rotations []uint16
	// dbusutil-gen: equal=uint16SliceEqual
	Reflects []uint16
	BestMode ModeInfo
	// dbusutil-gen: equal=modeInfosEqual
	Modes []ModeInfo
	// dbusutil-gen: equal=modeInfosEqual
	PreferredModes []ModeInfo
	MmWidth        uint32
	MmHeight       uint32

	Enabled           bool
	X                 int16
	Y                 int16
	Width             uint16
	Height            uint16
	Rotation          uint16
	Reflect           uint16
	RefreshRate       float64
	Brightness        float64
	CurrentRotateMode uint8

	oldRotation uint16

	CurrentMode     ModeInfo
	CurrentFillMode string `prop:"access:rw"`
	// dbusutil-gen: equal=method:Equal
	AvailableFillModes strv.Strv

	backup *MonitorBackup
}

func (m *Monitor) String() string {
	return fmt.Sprintf("<Monitor id=%d name=%s>", m.ID, m.Name)
}

func (m *Monitor) GetInterfaceName() string {
	return dbusInterfaceMonitor
}

func (m *Monitor) getPath() dbus.ObjectPath {
	return dbus.ObjectPath(dbusPath + "/Monitor_" + strconv.Itoa(int(m.ID)))
}

type MonitorBackup struct {
	Enabled    bool
	Mode       ModeInfo
	X, Y       int16
	Reflect    uint16
	Rotation   uint16
	Brightness float64
}

func (m *Monitor) markChanged() {
	m.m.setPropHasChanged(true)
	if m.backup == nil {
		m.backup = &MonitorBackup{
			Enabled:    m.Enabled,
			Mode:       m.CurrentMode,
			X:          m.X,
			Y:          m.Y,
			Reflect:    m.Reflect,
			Rotation:   m.Rotation,
			Brightness: m.Brightness,
		}
	}
}

func (m *Monitor) Enable(enabled bool) *dbus.Error {
	logger.Debugf("monitor %v %v dbus call Enable %v", m.ID, m.Name, enabled)
	if m.Enabled == enabled {
		return nil
	}

	m.markChanged()
	m.enable(enabled)
	return nil
}

func (m *Monitor) enable(enabled bool) {
	m.setPropEnabled(enabled)
}

func (m *Monitor) SetMode(mode uint32) *dbus.Error {
	logger.Debugf("monitor %v %v dbus call SetMode %v", m.ID, m.Name, mode)
	if m.CurrentMode.Id == mode {
		return nil
	}

	newMode := findMode(m.Modes, mode)
	if newMode.Id == 0 {
		return dbusutil.ToError(errors.New("invalid mode"))
	}

	m.markChanged()
	m.setMode(newMode)
	return nil
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

func (m *Monitor) selectMode(width, height uint16, rate float64) ModeInfo {
	mode := getFirstModeBySizeRate(m.Modes, width, height, rate)
	if mode.Id != 0 {
		return mode
	}
	mode = getFirstModeBySize(m.Modes, width, height)
	if mode.Id != 0 {
		return mode
	}
	return m.BestMode
}

func (m *Monitor) SetModeBySize(width, height uint16) *dbus.Error {
	logger.Debugf("monitor %v %v dbus call SetModeBySize %v %v", m.ID, m.Name, width, height)
	mode := getFirstModeBySize(m.Modes, width, height)
	if mode.Id == 0 {
		return dbusutil.ToError(errors.New("not found match mode"))
	}
	return m.SetMode(mode.Id)
}

func (m *Monitor) SetRefreshRate(value float64) *dbus.Error {
	logger.Debugf("monitor %v %v dbus call SetRefreshRate %v", m.ID, m.Name, value)
	if m.Width == 0 || m.Height == 0 {
		return dbusutil.ToError(errors.New("width or height is 0"))
	}
	mode := getFirstModeBySizeRate(m.Modes, m.Width, m.Height, value)
	if mode.Id == 0 {
		return dbusutil.ToError(errors.New("not found match mode"))
	}
	return m.SetMode(mode.Id)
}

func (m *Monitor) SetPosition(X, y int16) *dbus.Error {
	logger.Debugf("monitor %v %v dbus call SetPosition %v %v", m.ID, m.Name, X, y)
	if m.X == X && m.Y == y {
		return nil
	}

	m.markChanged()
	m.setPosition(X, y)
	return nil
}

func (m *Monitor) setPosition(X, y int16) {
	m.PropsMu.Lock()
	m.setPropX(X)
	m.setPropY(y)
	m.PropsMu.Unlock()
}

func (m *Monitor) SetReflect(value uint16) *dbus.Error {
	logger.Debugf("monitor %v %v dbus call SetReflect %v", m.ID, m.Name, value)
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
	logger.Debugf("monitor %v %v dbus call SetRotation %v", m.ID, m.Name, value)
	if m.Rotation == value {
		return nil
	}
	m.oldRotation = m.Rotation
	m.markChanged()
	m.setRotation(value)
	m.setPropCurrentRotateMode(RotationFinishModeManual)
	return nil
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

func (m *Monitor) setPropBrightnessWithLock(value float64) {
	m.PropsMu.Lock()
	m.setPropBrightness(value)
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
	m.setPropBrightness(b.Brightness)

	m.backup = nil
}

func getRandrStatusStr(status uint8) string {
	switch status {
	case randr.SetConfigSuccess:
		return "success"
	case randr.SetConfigFailed:
		return "failed"
	case randr.SetConfigInvalidConfigTime:
		return "invalid config time"
	case randr.SetConfigInvalidTime:
		return "invalid time"
	default:
		return fmt.Sprintf("unknown status %d", status)
	}
}

func toSysMonitorConfigs(monitors []*Monitor, primary string) SysMonitorConfigs {
	found := false
	result := make(SysMonitorConfigs, len(monitors))
	for i, m := range monitors {
		cfg := m.toSysConfig()
		if !found && m.Name == primary {
			cfg.Primary = true
			found = true
		}
		result[i] = cfg
	}
	return result
}

func (m *Monitor) toBasicSysConfig() *SysMonitorConfig {
	return &SysMonitorConfig{
		UUID: m.uuid,
		Name: m.Name,
	}
}

func (m *Monitor) toSysConfig() *SysMonitorConfig {
	return &SysMonitorConfig{
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
		Brightness:  m.Brightness,
	}
}

func (m *Monitor) dumpInfoForDebug() {
	logger.Debugf("dump info monitor %v, uuid: %v, id: %v, %v+%v,%vx%v, enable: %v, rotation: %v, reflect: %v, current mode: %+v",
		m.Name,
		m.uuid,
		m.ID,
		m.X, m.Y, m.Width, m.Height,
		m.Enabled,
		m.Rotation, m.Reflect,
		m.CurrentMode)
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

func (monitors Monitors) GetById(id uint32) *Monitor {
	for _, monitor := range monitors {
		if monitor.ID == id {
			return monitor
		}
	}
	return nil
}

func (monitors Monitors) GetByUuid(uuid string) *Monitor {
	for _, monitor := range monitors {
		if monitor.uuid == uuid {
			return monitor
		}
	}
	return nil
}

func (m *Monitor) generateFillModeKey() string {
	if needSwapWidthHeight(m.Rotation) {
		return fmt.Sprintf("%s:%dx%d", m.uuid, m.Height, m.Width)
	}

	return fmt.Sprintf("%s:%dx%d", m.uuid, m.Width, m.Height)
}

func (m *Monitor) setCurrentFillMode(write *dbusutil.PropertyWrite) *dbus.Error {
	value, _ := write.Value.(string)
	logger.Debugf("dbus call %v setCurrentFillMode %v", m, value)
	err := m.m.setMonitorFillMode(m, value)
	if err != nil {
		logger.Warning(err)
	}
	return dbusutil.ToError(err)
}
