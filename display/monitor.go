package display

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/godbus/dbus"
	x "github.com/linuxdeepin/go-x11-client"
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
	m       *Manager
	service *dbusutil.Service
	uuid    string
	PropsMu sync.RWMutex

	ID            uint32
	Name          string
	Connected     bool
	realConnected bool
	Manufacturer  string
	Model         string
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

func (m *Monitor) clone() *Monitor {
	m.PropsMu.RLock()
	defer m.PropsMu.RUnlock()

	monitorCp := Monitor{
		m:                  m.m,
		service:            m.service,
		uuid:               m.uuid,
		ID:                 m.ID,
		Name:               m.Name,
		Connected:          m.Connected,
		realConnected:      m.realConnected,
		Manufacturer:       m.Manufacturer,
		Model:              m.Model,
		Rotations:          m.Rotations,
		Reflects:           m.Reflects,
		BestMode:           m.BestMode,
		Modes:              m.Modes,
		PreferredModes:     m.PreferredModes,
		MmWidth:            m.MmWidth,
		MmHeight:           m.MmHeight,
		Enabled:            m.Enabled,
		X:                  m.X,
		Y:                  m.Y,
		Width:              m.Width,
		Height:             m.Height,
		Rotation:           m.Rotation,
		Reflect:            m.Reflect,
		RefreshRate:        m.RefreshRate,
		Brightness:         m.Brightness,
		CurrentRotateMode:  m.CurrentRotateMode,
		oldRotation:        m.oldRotation,
		CurrentMode:        m.CurrentMode,
		CurrentFillMode:    m.CurrentFillMode,
		AvailableFillModes: m.AvailableFillModes,
		backup:             nil,
	}

	return &monitorCp
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
	// NOTE: 不用加锁
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
	m.PropsMu.Lock()
	defer m.PropsMu.Unlock()

	logger.Debugf("monitor %v %v dbus call Enable %v", m.ID, m.Name, enabled)
	if m.Enabled == enabled {
		return nil
	}

	m.markChanged()
	m.setPropEnabled(enabled)
	return nil
}

func (m *Monitor) SetMode(mode uint32) *dbus.Error {
	m.PropsMu.Lock()
	defer m.PropsMu.Unlock()

	logger.Debugf("monitor %v %v dbus call SetMode %v", m.ID, m.Name, mode)
	return m.setModeNoLock(mode)
}

func (m *Monitor) setModeNoLock(mode uint32) *dbus.Error {
	if m.CurrentMode.Id == mode {
		return nil
	}

	newMode := findMode(m.Modes, mode)
	if newMode.isZero() {
		return dbusutil.ToError(errors.New("invalid mode"))
	}

	m.markChanged()
	m.setMode(newMode)
	return nil
}

func (m *Monitor) setMode(mode ModeInfo) {
	m.setPropCurrentMode(mode)

	width := mode.Width
	height := mode.Height

	swapWidthHeightWithRotation(m.Rotation, &width, &height)

	m.setPropWidth(width)
	m.setPropHeight(height)
	m.setPropRefreshRate(mode.Rate)
}

// setMode 的不发送属性改变信号版本
func (m *Monitor) setModeNoEmitChanged(mode ModeInfo) {
	m.CurrentMode = mode

	width := mode.Width
	height := mode.Height

	swapWidthHeightWithRotation(m.Rotation, &width, &height)

	m.Width = width
	m.Height = height
	m.RefreshRate = mode.Rate
}

func (m *Monitor) selectMode(width, height uint16, rate float64) ModeInfo {
	mode := getFirstModeBySizeRate(m.Modes, width, height, rate)
	if !mode.isZero() {
		return mode
	}
	mode = getFirstModeBySize(m.Modes, width, height)
	if !mode.isZero() {
		return mode
	}
	return m.BestMode
}

func (m *Monitor) SetModeBySize(width, height uint16) *dbus.Error {
	m.PropsMu.Lock()
	defer m.PropsMu.Unlock()

	logger.Debugf("monitor %v %v dbus call SetModeBySize %v %v", m.ID, m.Name, width, height)
	mode := getFirstModeBySize(m.Modes, width, height)
	if mode.isZero() {
		return dbusutil.ToError(errors.New("not found match mode"))
	}
	return m.setModeNoLock(mode.Id)
}

func (m *Monitor) SetRefreshRate(value float64) *dbus.Error {
	m.PropsMu.Lock()
	defer m.PropsMu.Unlock()

	logger.Debugf("monitor %v %v dbus call SetRefreshRate %v", m.ID, m.Name, value)
	if m.Width == 0 || m.Height == 0 {
		return dbusutil.ToError(errors.New("width or height is 0"))
	}
	mode := getFirstModeBySizeRate(m.Modes, m.Width, m.Height, value)
	if mode.isZero() {
		return dbusutil.ToError(errors.New("not found match mode"))
	}
	return m.setModeNoLock(mode.Id)
}

func (m *Monitor) SetPosition(X, y int16) *dbus.Error {
	logger.Debugf("monitor %v %v dbus call SetPosition %v %v", m.ID, m.Name, X, y)
	if _dpy == nil {
		return dbusutil.ToError(errors.New("_dpy is nil"))
	}

	if _dpy.getInApply() {
		logger.Debug("reject set position, in apply")
		return nil
	}

	m.PropsMu.Lock()
	defer m.PropsMu.Unlock()

	if m.X == X && m.Y == y {
		logger.Debug("reject set position, no change")
		return nil
	}

	m.markChanged()
	m.setPropX(X)
	m.setPropY(y)
	return nil
}

func (m *Monitor) SetReflect(value uint16) *dbus.Error {
	m.PropsMu.Lock()
	defer m.PropsMu.Unlock()

	logger.Debugf("monitor %v %v dbus call SetReflect %v", m.ID, m.Name, value)
	if m.Reflect == value {
		return nil
	}
	m.markChanged()
	m.setPropReflect(value)
	return nil
}

func (m *Monitor) SetRotation(value uint16) *dbus.Error {
	m.PropsMu.Lock()
	defer m.PropsMu.Unlock()

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
	width := m.CurrentMode.Width
	height := m.CurrentMode.Height

	swapWidthHeightWithRotation(value, &width, &height)

	m.setPropRotation(value)
	m.setPropWidth(width)
	m.setPropHeight(height)
}

func (m *Monitor) setPropBrightnessWithLock(value float64) {
	m.PropsMu.Lock()
	m.setPropBrightness(value)
	m.PropsMu.Unlock()
}

func (m *Monitor) resetChanges() {
	m.PropsMu.Lock()
	defer m.PropsMu.Unlock()

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

func (monitors Monitors) getMonitorsId() string {
	if len(monitors) == 0 {
		return ""
	}
	var ids []string
	for _, monitor := range monitors {
		ids = append(ids, monitor.uuid)
	}
	sort.Strings(ids)
	return strings.Join(ids, monitorsIdDelimiter)
}

func (monitors Monitors) getPaths() []dbus.ObjectPath {
	sort.Slice(monitors, func(i, j int) bool {
		return monitors[i].ID < monitors[j].ID
	})
	paths := make([]dbus.ObjectPath, len(monitors))
	for i, monitor := range monitors {
		paths[i] = monitor.getPath()
	}
	return paths
}

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
	width, height := m.Width, m.Height
	swapWidthHeightWithRotation(m.Rotation, &width, &height)
	return fmt.Sprintf("%s:%dx%d", m.uuid, width, height)
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

// MonitorInfo X 和 Wayland 共用的显示器信息
type MonitorInfo struct {
	// 只在 X 下使用
	crtc               randr.Crtc
	Enabled            bool
	ID                 uint32
	UUID               string
	Name               string
	Connected          bool // 实际的是否连接，对应于 Monitor 的 realConnected
	VirtualConnected   bool // 用于前端，对应于 Monitor 的 Connected
	Modes              []ModeInfo
	CurrentMode        ModeInfo
	PreferredMode      ModeInfo
	X                  int16
	Y                  int16
	Width              uint16
	Height             uint16
	Rotation           uint16
	Rotations          uint16
	MmHeight           uint32
	MmWidth            uint32
	EDID               []byte
	Manufacturer       string
	Model              string
	CurrentFillMode    string
	AvailableFillModes []string
}

func (m *MonitorInfo) dumpForDebug() {
	logger.Debugf("MonitorInfo{crtc: %d,\nID: %v,\nName: %v,\nConnected: %v,\nCurrentMode: %v,\nPreferredMode: %v,\n"+
		"X: %v, Y: %v, Width: %v, Height: %v,\nRotation: %v,\nRotations: %v,\nMmWidth: %v,\nMmHeight: %v,\nModes: %v}",
		m.crtc, m.ID, m.Name, m.Connected, m.CurrentMode, m.PreferredMode, m.X, m.Y, m.Width, m.Height, m.Rotation, m.Rotations,
		m.MmWidth, m.MmHeight, m.Modes)
}

func (m *MonitorInfo) outputId() randr.Output {
	return randr.Output(m.ID)
}

func (m *MonitorInfo) equal(other *MonitorInfo) bool {
	return reflect.DeepEqual(m, other)
}

func (m *MonitorInfo) getRect() x.Rectangle {
	return x.Rectangle{
		X:      m.X,
		Y:      m.Y,
		Width:  m.Width,
		Height: m.Height,
	}
}
