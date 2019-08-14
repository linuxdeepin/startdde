package display

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"pkg.deepin.io/lib/xdg/basedir"

	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
	"pkg.deepin.io/gir/gio-2.0"
	"pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
)

const (
	DisplayModeCustom uint8 = iota
	DisplayModeMirror
	DisplayModeExtend
	DisplayModeOnlyOne
	DisplayModeUnknow
)

const (
	gsSchemaDisplay  = "com.deepin.dde.display"
	gsKeyDisplayMode = "display-mode"
	gsKeyBrightness  = "brightness"
	gsKeySetter      = "brightness-setter"
	gsKeyMapOutput   = "map-output"
	//gsKeyPrimary     = "primary"
	gsKeyCustomMode = "current-custom-mode"

	customModeDelim     = "+"
	monitorsIdDelimiter = ","
)

//go:generate dbusutil-gen -output display_dbusutil.go -import pkg.deepin.io/lib/dbus1,github.com/linuxdeepin/go-x11-client -type Manager,Monitor manager.go monitor.go

type Manager struct {
	service              *dbusutil.Service
	xConn                *x.Conn
	PropsMu              sync.RWMutex
	config               Config
	recommendScaleFactor float64
	modes                []randr.ModeInfo
	monitorMap           map[randr.Output]*Monitor
	monitorMapMu         sync.Mutex
	crtcMap              map[randr.Crtc]*randr.GetCrtcInfoReply
	crtcMapMu            sync.Mutex
	outputMap            map[randr.Output]*randr.GetOutputInfoReply
	outputMapMu          sync.Mutex
	configTimestamp      x.Timestamp
	settings             *gio.Settings
	monitorsId           string

	// dbusutil-gen: equal=nil
	Monitors []dbus.ObjectPath
	// dbusutil-gen: equal=nil
	CustomIdList []string
	HasChanged   bool
	DisplayMode  byte
	// dbusutil-gen: equal=nil
	Brightness map[string]float64
	// dbusutil-gen: equal=nil
	TouchMap        map[string]string
	CurrentCustomId string
	Primary         string
	// dbusutil-gen: equal=nil
	PrimaryRect  x.Rectangle
	ScreenWidth  uint16
	ScreenHeight uint16

	methods *struct {
		AssociateTouch         func() `in:"outputName,touch"`
		ChangeBrightness       func() `in:"raised"`
		DeleteCustomMode       func() `in:"name"`
		GetBrightness          func() `out:"values"`
		ListOutputNames        func() `out:"names"`
		ListOutputsCommonModes func() `out:"modes"`
		ModifyConfigName       func() `in:"name,newName"`
		SetAndSaveBrightness   func() `in:"outputName,value"`
		SetBrightness          func() `in:"outputName,value"`
		SetPrimary             func() `in:"outputName"`
		SwitchMode             func() `in:"mode,name"`
	}
}

type ModeInfo struct {
	Id     uint32
	Width  uint16
	Height uint16
	Rate   float64
}

var configFile = filepath.Join(basedir.GetUserConfigDir(), "deepin/startdde/display.json")

func getXConn() (*x.Conn, error) {
	conn, err := x.NewConn()
	if err != nil {
		return nil, err
	}
	_, err = randr.QueryVersion(conn, randr.MajorVersion, randr.MinorVersion).Reply(conn)
	if err != nil {
		logger.Warning(err)
	}
	return conn, nil
}

func newManager(service *dbusutil.Service) *Manager {
	m := &Manager{
		service:    service,
		monitorMap: make(map[randr.Output]*Monitor),
	}

	m.settings = gio.NewSettings(gsSchemaDisplay)
	m.DisplayMode = uint8(m.settings.GetEnum(gsKeyDisplayMode))
	if m.DisplayMode == DisplayModeUnknow {
		m.DisplayMode = DisplayModeExtend
	}
	m.CurrentCustomId = m.settings.GetString(gsKeyCustomMode)

	var err error
	m.xConn, err = getXConn()
	if err != nil {
		logger.Fatal(err)
	}

	screen := m.xConn.GetDefaultScreen()
	m.ScreenWidth = screen.WidthInPixels
	m.ScreenHeight = screen.HeightInPixels

	resources, err := m.getScreenResources()
	if err == nil {
		m.modes = resources.Modes
		m.configTimestamp = resources.ConfigTimestamp
		err = m.initCrtcMap(resources.Crtcs)
		if err != nil {
			logger.Warning(err)
		}
		err = m.initOutputMap(resources.Outputs)
		if err != nil {
			logger.Warning(err)
		}
	} else {
		logger.Warning(err)
	}

	for output, outputInfo := range m.outputMap {
		err = m.addMonitor(output, outputInfo)
		if err != nil {
			logger.Warning(err)
		}
	}
	m.updatePropMonitors()
	m.recommendScaleFactor = m.calcRecommendedScaleFactor()
	m.updateOutputPrimary()

	m.config, err = loadConfig(configFile)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warning(err)
		}
		m.config = make(Config)
	}
	m.CustomIdList = m.getCustomIdList()
	return m
}

func (m *Manager) applyDisplayMode() {
	logger.Debug("applyDisplayMode")
	var err error
	if m.DisplayMode == DisplayModeCustom {
		err = m.switchModeCustom(m.CurrentCustomId)
		if err != nil {
			logger.Warning(err)
		}
		return
	}

	monitors := m.getConnectedMonitors()
	if len(monitors) == 1 {
		// 单屏
		screenCfg := m.getScreenConfig()
		if screenCfg.Single != nil {
			err = m.applyMonitorConfigs([]*MonitorConfig{screenCfg.Single})
			if err != nil {
				logger.Warning(err)
			}
		}

		err = m.setOutputPrimary(randr.Output(monitors[0].ID))
		if err != nil {
			logger.Warning(err)
		}
	}

	switch m.DisplayMode {
	case DisplayModeMirror:
		err = m.switchModeMirror()
	case DisplayModeExtend:
		err = m.switchModeExtend()
	case DisplayModeOnlyOne:
		err = m.switchModeOnlyOne("")
	}

	if err != nil {
		logger.Warning(err)
	}

}

func (m *Manager) init() {
	m.listenEvent()
	m.applyDisplayMode()
	m.initBrightness()
	m.initTouchMap()
}

func (m *Manager) calcRecommendedScaleFactor() float64 {
	minScaleFactor := 3.0
	monitors := m.getConnectedMonitors()
	if len(monitors) == 0 {
		return 1.0
	}
	for _, monitor := range monitors {
		scaleFactor := calcRecommendedScaleFactor(float64(monitor.Width), float64(monitor.MmWidth))
		if minScaleFactor > scaleFactor {
			minScaleFactor = scaleFactor
		}
	}

	return minScaleFactor
}

func calcRecommendedScaleFactor(pxWidth, mmWidth float64) float64 {
	if mmWidth == 0 {
		return 1
	}
	ppm := pxWidth / mmWidth
	scaleFactor := ppm / (1366.0 / 310.0)
	return toListedScaleFactor(scaleFactor)
}

func toListedScaleFactor(s float64) float64 {
	const (
		min  = 1.0
		max  = 3.0
		step = 0.25
	)
	if s <= min {
		return min
	} else if s >= max {
		return max
	}

	for i := min; i <= max; i += step {
		if i > s {
			ii := i - step
			d1 := s - ii
			d2 := i - s

			if d1 >= d2 {
				return i
			} else {
				return ii
			}
		}
	}
	return max
}

func (m *Manager) getScreenResources() (*randr.GetScreenResourcesReply, error) {
	root := m.xConn.GetDefaultScreen().Root
	resources, err := randr.GetScreenResources(m.xConn, root).Reply(m.xConn)
	return resources, err
}

func (m *Manager) initCrtcMap(crtcs []randr.Crtc) error {
	m.crtcMap = make(map[randr.Crtc]*randr.GetCrtcInfoReply)
	for _, crtc := range crtcs {
		crtcInfo, err := m.getCrtcInfo(crtc)
		if err != nil {
			return err
		}
		m.crtcMap[crtc] = crtcInfo
	}
	return nil
}

func (m *Manager) initOutputMap(outputs []randr.Output) error {
	m.outputMap = make(map[randr.Output]*randr.GetOutputInfoReply)
	for _, output := range outputs {
		outputInfo, err := m.getOutputInfo(output)
		if err != nil {
			return err
		}
		m.outputMap[output] = outputInfo
	}
	return nil
}

func (m *Manager) getCrtcInfo(crtc randr.Crtc) (*randr.GetCrtcInfoReply, error) {
	m.PropsMu.RLock()
	cfgTs := m.configTimestamp
	m.PropsMu.RUnlock()

	crtcInfo, err := randr.GetCrtcInfo(m.xConn, crtc, cfgTs).Reply(m.xConn)
	if err != nil {
		return nil, err
	}
	if crtcInfo.Status != randr.StatusSuccess {
		return nil, fmt.Errorf("status is not success, is %v", crtcInfo.Status)
	}
	return crtcInfo, err
}

func (m *Manager) updateCrtcInfo(crtc randr.Crtc) (*randr.GetCrtcInfoReply, error) {
	crtcInfo, err := m.getCrtcInfo(crtc)
	if err != nil {
		return nil, err
	}
	m.crtcMapMu.Lock()
	m.crtcMap[crtc] = crtcInfo
	m.crtcMapMu.Unlock()
	return crtcInfo, nil
}

func (m *Manager) getOutputInfo(output randr.Output) (*randr.GetOutputInfoReply, error) {
	m.PropsMu.RLock()
	cfgTs := m.configTimestamp
	m.PropsMu.RUnlock()

	outputInfo, err := randr.GetOutputInfo(m.xConn, output, cfgTs).Reply(m.xConn)
	if err != nil {
		return nil, err
	}
	if outputInfo.Status != randr.StatusSuccess {
		return nil, fmt.Errorf("status is not success, is %v", outputInfo.Status)
	}
	return outputInfo, err
}

func (m *Manager) updateOutputInfo(output randr.Output) (*randr.GetOutputInfoReply, error) {
	outputInfo, err := m.getOutputInfo(output)
	if err != nil {
		return nil, err
	}
	m.outputMapMu.Lock()
	m.outputMap[output] = outputInfo
	m.outputMapMu.Unlock()
	return outputInfo, nil
}

func (m *Manager) getModeInfo(mode randr.Mode) ModeInfo {
	if mode == 0 {
		return ModeInfo{}
	}
	for _, modeInfo := range m.modes {
		if modeInfo.Id == uint32(mode) {
			return toModeInfo(modeInfo)
		}
	}
	return ModeInfo{}
}

func (m *Manager) getModeInfos(modes []randr.Mode) []ModeInfo {
	var result []ModeInfo
	for _, mode := range modes {
		modeInfo := m.getModeInfo(mode)
		if modeInfo.Id != 0 {
			result = append(result, modeInfo)
		}
	}
	return result
}

func (m *Manager) addMonitor(output randr.Output, outputInfo *randr.GetOutputInfoReply) error {
	m.monitorMapMu.Lock()
	_, ok := m.monitorMap[output]
	m.monitorMapMu.Unlock()
	if ok {
		return nil
	}

	connected := outputInfo.Connection == randr.ConnectionConnected
	enabled := outputInfo.Crtc != 0
	var err error
	var crtcInfo *randr.GetCrtcInfoReply
	if outputInfo.Crtc != 0 {
		m.crtcMapMu.Lock()
		crtcInfo = m.crtcMap[outputInfo.Crtc]
		m.crtcMapMu.Unlock()

		if crtcInfo == nil {
			crtcInfo, err = m.updateCrtcInfo(outputInfo.Crtc)
			if err != nil {
				logger.Warning(err)
			}
		}
	}

	edid, err := getOutputEDID(m.xConn, output)
	if err != nil {
		logger.Warning(err)
	}
	logger.Debug("addMonitor", output, outputInfo.Name)
	monitor := &Monitor{
		service:   m.service,
		m:         m,
		ID:        uint32(output),
		Name:      outputInfo.Name,
		Connected: connected,
		MmWidth:   outputInfo.MmWidth,
		MmHeight:  outputInfo.MmHeight,
		Enabled:   enabled,
		crtc:      outputInfo.Crtc,
		uuid:      getOutputUUID(outputInfo.Name, edid),
	}

	monitor.Modes = m.getModeInfos(outputInfo.Modes)
	monitor.BestMode = monitor.getBestMode(m, outputInfo)
	monitor.PreferredModes = []ModeInfo{monitor.BestMode}

	if crtcInfo != nil {
		monitor.X = crtcInfo.X
		monitor.Y = crtcInfo.Y
		monitor.Width = crtcInfo.Width
		monitor.Height = crtcInfo.Height

		monitor.Reflects = getReflects(crtcInfo.Rotations)
		monitor.Rotations = getRotations(crtcInfo.Rotations)
		monitor.Rotation, monitor.Reflect = parseCrtcRotation(crtcInfo.Rotation)

		monitor.CurrentMode = m.getModeInfo(crtcInfo.Mode)
		monitor.RefreshRate = monitor.CurrentMode.Rate
	}

	err = m.service.Export(monitor.getPath(), monitor)
	if err != nil {
		return err
	}
	m.monitorMapMu.Lock()
	m.monitorMap[output] = monitor
	m.monitorMapMu.Unlock()
	return nil
}

func (m *Manager) updateMonitor(output randr.Output, outputInfo *randr.GetOutputInfoReply) {
	m.monitorMapMu.Lock()
	monitor, ok := m.monitorMap[output]
	m.monitorMapMu.Unlock()
	if !ok {
		err := m.addMonitor(output, outputInfo)
		if err != nil {
			logger.Warning(err)
			return
		}

		m.updatePropMonitors()
		return
	}

	connected := outputInfo.Connection == randr.ConnectionConnected
	enabled := outputInfo.Crtc != 0
	var err error
	var crtcInfo *randr.GetCrtcInfoReply
	if outputInfo.Crtc != 0 {
		m.crtcMapMu.Lock()
		crtcInfo = m.crtcMap[outputInfo.Crtc]
		m.crtcMapMu.Unlock()

		if crtcInfo == nil {
			crtcInfo, err = m.updateCrtcInfo(outputInfo.Crtc)
			if err != nil {
				logger.Warning(err)
			}
		}
	}
	monitor.PropsMu.Lock()
	monitor.crtc = outputInfo.Crtc
	monitor.setPropConnected(connected)
	monitor.setPropEnabled(enabled)
	monitor.setPropModes(m.getModeInfos(outputInfo.Modes))
	monitor.setPropBestMode(monitor.getBestMode(m, outputInfo))
	monitor.setPropPreferredModes([]ModeInfo{monitor.BestMode})
	monitor.PropsMu.Unlock()
	m.updateMonitorCrtcInfo(monitor, crtcInfo)
}

func (m *Manager) updateMonitorCrtcInfo(monitor *Monitor, crtcInfo *randr.GetCrtcInfoReply) {
	if crtcInfo == nil {
		monitor.PropsMu.Lock()
		monitor.setPropX(0)
		monitor.setPropY(0)
		monitor.setPropWidth(0)
		monitor.setPropHeight(0)

		monitor.setPropReflects(nil)
		monitor.setPropRotations(nil)
		monitor.setPropRotation(0)
		monitor.setPropReflect(0)

		monitor.setPropCurrentMode(ModeInfo{})
		monitor.setPropRefreshRate(0)
		monitor.PropsMu.Unlock()
		return
	}

	rotation, reflect := parseCrtcRotation(crtcInfo.Rotation)
	monitor.PropsMu.Lock()
	monitor.setPropX(crtcInfo.X)
	monitor.setPropY(crtcInfo.Y)
	monitor.setPropWidth(crtcInfo.Width)
	monitor.setPropHeight(crtcInfo.Height)

	monitor.setPropReflects(getReflects(crtcInfo.Rotations))
	monitor.setPropRotations(getRotations(crtcInfo.Rotations))
	monitor.setPropRotation(rotation)
	monitor.setPropReflect(reflect)

	monitor.setPropCurrentMode(m.getModeInfo(crtcInfo.Mode))
	monitor.setPropRefreshRate(monitor.CurrentMode.Rate)
	monitor.PropsMu.Unlock()
}

func (m *Manager) findFreeCrtc(output randr.Output) randr.Crtc {
	m.crtcMapMu.Lock()
	defer m.crtcMapMu.Unlock()

	for crtc, crtcInfo := range m.crtcMap {
		if len(crtcInfo.Outputs) == 0 && outputSliceContains(crtcInfo.PossibleOutputs, output) {
			return crtc
		}
	}
	return 0
}

func (m *Manager) switchModeMirror() (err error) {
	logger.Debug("switch mode mirror")
	screenCfg := m.getScreenConfig()
	configs := screenCfg.getMonitorConfigs(DisplayModeMirror, "")
	monitors := m.getConnectedMonitors()
	commonSizes := getMonitorsCommonSizes(monitors)
	if len(commonSizes) == 0 {
		err = errors.New("not found common size")
		return
	}
	maxSize := getMaxAreaSize(commonSizes)
	logger.Debug("max common size:", maxSize)
	for _, monitor := range m.monitorMap {
		if monitor.Connected {
			monitor.enable(true)

			cfg := getMonitorConfigByUuid(configs, monitor.uuid)
			var mode ModeInfo
			if cfg != nil {
				mode = monitor.selectMode(cfg.Width, cfg.Height, cfg.RefreshRate)
			} else {
				mode = getFirstModeBySize(monitor.Modes, maxSize.width, maxSize.height)
			}
			monitor.setMode(mode)
			monitor.setPosition(0, 0)
			monitor.setRotation(randr.RotationRotate0)
			monitor.setReflect(0)

		} else {
			monitor.enable(false)
		}
	}

	err = m.applyMonitorsConfig()
	if err != nil {
		return
	}

	monitor0 := getMinIDMonitor(m.getConnectedMonitors())
	if monitor0 != nil {
		err = m.setOutputPrimary(randr.Output(monitor0.ID))
		if err != nil {
			return
		}
	}
	return
}

type screenSize struct {
	width    uint16
	height   uint16
	mmWidth  uint32
	mmHeight uint32
}

func (m *Manager) getScreenSize1() screenSize {
	width, height := m.getScreenSize()
	mmWidth := uint32(float64(width) / 3.792)
	mmHeight := uint32(float64(height) / 3.792)
	return screenSize{
		width:    width,
		height:   height,
		mmWidth:  mmWidth,
		mmHeight: mmHeight,
	}
}

func (m *Manager) setScreenSize(ss screenSize) error {
	root := m.xConn.GetDefaultScreen().Root
	err := randr.SetScreenSizeChecked(m.xConn, root, ss.width, ss.height, ss.mmWidth,
		ss.mmHeight).Check(m.xConn)
	logger.Debugf("set screen size %dx%d, mm: %dx%d",
		ss.width, ss.height, ss.mmWidth, ss.mmHeight)
	return err
}

type crtcConfig struct {
	crtc    randr.Crtc
	outputs []randr.Output

	x        int16
	y        int16
	rotation uint16
	mode     randr.Mode
}

// TODO rename this method
func (m *Manager) applyMonitorsConfig() error {
	x.GrabServer(m.xConn)
	defer func() {
		err := x.UngrabServerChecked(m.xConn).Check(m.xConn)
		if err != nil {
			logger.Warning(err)
		}
	}()

	monitorCrtcCfgMap := make(map[randr.Output]crtcConfig)
	for output, monitor := range m.monitorMap {
		if monitor.Enabled {
			crtc := monitor.crtc
			if crtc == 0 {
				crtc = m.findFreeCrtc(output)
				if crtc == 0 {
					return errors.New("failed to find free crtc")
				}
			}
			monitorCrtcCfgMap[output] = crtcConfig{
				crtc:     crtc,
				x:        monitor.X,
				y:        monitor.Y,
				mode:     randr.Mode(monitor.CurrentMode.Id),
				rotation: monitor.Rotation | monitor.Reflect,
				outputs:  []randr.Output{output},
			}
		} else {
			if monitor.crtc != 0 {
				monitorCrtcCfgMap[output] = crtcConfig{
					crtc:     monitor.crtc,
					rotation: randr.RotationRotate0,
				}
			}
		}
	}

	m.PropsMu.RLock()
	cfgTs := m.configTimestamp
	m.PropsMu.RUnlock()

	screenSize := m.getScreenSize1()

	m.crtcMapMu.Lock()
	for crtc, crtcInfo := range m.crtcMap {
		rect := getCrtcRect(crtcInfo)
		logger.Debugf("crtc %v, rect: %+v", crtc, rect)
		if int(rect.X)+int(rect.Width) <= int(screenSize.width) &&
			int(rect.Y)+int(rect.Height) <= int(screenSize.height) {
			// 适合
		} else {
			// 不适合新的屏幕大小，如果已经启用，则需要禁用它
			if len(crtcInfo.Outputs) == 0 {
				continue
			}
			logger.Debugf("disable crtc %v, it's outputs: %v", crtc, crtcInfo.Outputs)
			err := m.disableCrtc(crtc, cfgTs)
			if err != nil {
				return err
			}
		}
	}
	m.crtcMapMu.Unlock()

	err := m.setScreenSize(screenSize)
	if err != nil {
		return err
	}

	for output, monitor := range m.monitorMap {
		crtcCfg, ok := monitorCrtcCfgMap[output]
		if !ok {
			continue
		}
		err := monitor.applyConfig(crtcCfg)
		if err != nil {
			return err
		}

		outputInfo, err := m.updateOutputInfo(output)
		if err != nil {
			logger.Warning(err)
		}
		if outputInfo.Crtc != 0 {
			_, err = m.updateCrtcInfo(outputInfo.Crtc)
			if err != nil {
				logger.Warning(err)
			}
		}
		m.updateMonitor(output, outputInfo)
	}

	return nil
}

func (m *Manager) disableCrtc(crtc randr.Crtc, cfgTs x.Timestamp) error {
	setCfg, err := randr.SetCrtcConfig(m.xConn, crtc, 0, cfgTs,
		0, 0, 0, randr.RotationRotate0, nil).Reply(m.xConn)
	if err != nil {
		return err
	}
	if setCfg.Status != randr.SetConfigSuccess {
		return fmt.Errorf("failed to disable crtc %d: %v",
			crtc, getRandrStatusStr(setCfg.Status))
	}
	return nil
}

func (m *Manager) setOutputPrimary(output randr.Output) error {
	logger.Debug("set output primary", output)
	root := m.xConn.GetDefaultScreen().Root
	return randr.SetOutputPrimaryChecked(m.xConn, root, output).Check(m.xConn)
}

func (m *Manager) getOutputPrimary() (randr.Output, error) {
	root := m.xConn.GetDefaultScreen().Root
	reply, err := randr.GetOutputPrimary(m.xConn, root).Reply(m.xConn)
	if err != nil {
		return 0, err
	}
	return reply.Output, nil
}

// 更新属性 Primary 和 PrimaryRect
func (m *Manager) updateOutputPrimary() {
	pOutput, err := m.getOutputPrimary()
	if err != nil {
		logger.Warning(err)
		return
	}

	var newPrimary string
	var newRect x.Rectangle

	if pOutput != 0 {
		m.outputMapMu.Lock()

		for output, outputInfo := range m.outputMap {
			if pOutput != output {
				continue
			}

			newPrimary = outputInfo.Name

			if outputInfo.Crtc == 0 {
				logger.Warning("new primary output crtc is 0")
			} else {
				m.crtcMapMu.Lock()
				crtcInfo := m.crtcMap[outputInfo.Crtc]
				m.crtcMapMu.Unlock()
				if crtcInfo == nil {
					logger.Warning("crtcInfo is nil")
				} else {
					newRect = getCrtcRect(crtcInfo)
				}
			}
			break
		}

		m.outputMapMu.Unlock()
	}

	m.PropsMu.Lock()
	m.setPropPrimary(newPrimary)
	m.setPropPrimaryRect(newRect)
	m.PropsMu.Unlock()

	logger.Debugf("updateOutputPrimary name: %q, rect: %#v", newPrimary, newRect)
}

func (m *Manager) setPrimary(name string) error {
	switch m.DisplayMode {
	case DisplayModeMirror:
		return errors.New("not allow set primary in mirror mode")

	case DisplayModeOnlyOne:
		return m.switchModeOnlyOne(name)

	case DisplayModeExtend, DisplayModeCustom:
		screenCfg := m.getScreenConfig()
		configs := screenCfg.getMonitorConfigs(m.DisplayMode, m.CurrentCustomId)

		var monitor0 *Monitor
		for _, monitor := range m.monitorMap {
			if monitor.Name != name {
				continue
			}

			if !monitor.Connected {
				return errors.New("monitor is not connected")
			}

			monitor0 = monitor
			break
		}

		if monitor0 == nil {
			return errors.New("not found monitor")
		}

		if len(configs) == 0 {
			if m.DisplayMode == DisplayModeCustom {
				return errors.New("custom mode configs is empty")
			}
			configs = toMonitorConfigs(m.getConnectedMonitors(), monitor0.Name)
		} else {
			// modify configs
			updateMonitorConfigsName(configs, m.monitorMap)
			setMonitorConfigsPrimary(configs, monitor0.uuid)
		}

		err := m.setOutputPrimary(randr.Output(monitor0.ID))
		if err != nil {
			return err
		}

		screenCfg.setMonitorConfigs(m.DisplayMode, m.CurrentCustomId, configs)

		err = m.config.save(configFile)
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("invalid display mode %v", m.DisplayMode)
	}
	return nil
}

func (m *Manager) switchModeExtend() (err error) {
	logger.Debug("switch mode extend")
	var monitors []*Monitor
	for _, monitor := range m.monitorMap {
		monitors = append(monitors, monitor)
	}
	sortMonitorsByID(monitors)
	screenCfg := m.getScreenConfig()
	configs := screenCfg.getMonitorConfigs(DisplayModeExtend, "")

	var xOffset int
	var monitor0 *Monitor
	for _, monitor := range monitors {
		if monitor.Connected {
			monitor.enable(true)

			cfg := getMonitorConfigByUuid(configs, monitor.uuid)
			var mode ModeInfo
			if cfg != nil {
				mode = monitor.selectMode(cfg.Width, cfg.Height, cfg.RefreshRate)
				if monitor0 == nil && cfg.Primary {
					monitor0 = monitor
				}

			} else {
				mode = monitor.BestMode
			}

			monitor.setMode(mode)

			if xOffset > math.MaxInt16 {
				xOffset = math.MaxInt16
			}
			monitor.setPosition(int16(xOffset), 0)
			monitor.setRotation(randr.RotationRotate0)
			monitor.setReflect(0)

			xOffset += int(monitor.Width)
		} else {
			monitor.enable(false)
		}
	}

	if monitor0 == nil {
		monitor0 = getMinIDMonitor(m.getConnectedMonitors())
	}

	err = m.applyMonitorsConfig()
	if err != nil {
		return
	}

	if monitor0 != nil {
		err = m.setOutputPrimary(randr.Output(monitor0.ID))
		if err != nil {
			return
		}
	}

	return
}

func (m *Manager) getScreenConfig() *ScreenConfig {
	id := getMonitorsId(m.monitorMap)
	screenCfg := m.config[id]
	if screenCfg == nil {
		screenCfg = &ScreenConfig{}
		m.config[id] = screenCfg
	}
	return screenCfg
}

func (m *Manager) switchModeOnlyOne(name string) (err error) {
	logger.Debug("switch mode only one", name)

	screenCfg := m.getScreenConfig()
	configs := screenCfg.getMonitorConfigs(DisplayModeOnlyOne, "")

	var monitor0 *Monitor
	var needSaveCfg bool
	if name != "" {
		needSaveCfg = true
		for _, monitor := range m.monitorMap {
			if monitor.Name == name {
				monitor0 = monitor

				if !monitor.Connected {
					err = errors.New("monitor is not connected")
					return
				}

				break
			}
		}
		if monitor0 == nil {
			err = errors.New("not found monitor")
			return
		}
	} else {
		var enableUuid string
		for _, cfg := range configs {
			if cfg.Enabled {
				enableUuid = cfg.UUID
				break
			}
		}
		if enableUuid != "" {
			for _, monitor := range m.monitorMap {
				if monitor.uuid == enableUuid {
					monitor0 = monitor
					break
				}
			}
		}

		if monitor0 == nil {
			needSaveCfg = true
			monitor0 = getMinIDMonitor(m.getConnectedMonitors())
		}

	}
	if monitor0 == nil {
		err = errors.New("monitor0 is nil")
		return
	}

	for _, monitor := range m.monitorMap {
		if monitor.uuid == monitor0.uuid {
			monitor.enable(true)
			cfg := getMonitorConfigByUuid(configs, monitor.uuid)
			var mode ModeInfo
			var rotation uint16 = randr.RotationRotate0
			var reflect uint16
			if cfg != nil {
				mode = monitor.selectMode(cfg.Width, cfg.Height, cfg.RefreshRate)
				rotation = cfg.Rotation
				reflect = cfg.Reflect
			} else {
				mode = monitor.BestMode
			}

			monitor.setMode(mode)
			monitor.setPosition(0, 0)
			monitor.setRotation(rotation)
			monitor.setReflect(reflect)

		} else {
			monitor.enable(false)
		}
	}

	err = m.applyMonitorsConfig()
	if err != nil {
		return
	}

	// set primary output
	err = m.setOutputPrimary(randr.Output(monitor0.ID))
	if err != nil {
		return
	}

	if needSaveCfg {
		screenCfg.setMonitorConfigs(DisplayModeOnlyOne, "",
			toMonitorConfigs(m.getConnectedMonitors(), monitor0.Name))

		logger.Debug("call config.save")
		err = m.config.save(configFile)
		if err != nil {
			return
		}
	}

	return
}

func (m *Manager) switchModeCustom(name string) (err error) {
	logger.Debug("switch mode custom", name)
	if name == "" {
		err = errors.New("name is empty")
		return
	}

	screenCfg := m.getScreenConfig()
	configs := screenCfg.getMonitorConfigs(DisplayModeCustom, name)
	if len(configs) > 0 {
		err = m.applyMonitorConfigs(configs)
		return
	}

	err = m.switchModeExtend()
	if err != nil {
		return
	}

	screenCfg.setMonitorConfigs(DisplayModeCustom, name,
		toMonitorConfigs(m.getConnectedMonitors(), m.Primary))

	err = m.config.save(configFile)
	if err != nil {
		return
	}
	m.setPropCustomIdList(m.getCustomIdList())
	return
}

func (m *Manager) switchMode(mode byte, name string) (err error) {
	switch mode {
	case DisplayModeMirror:
		err = m.switchModeMirror()
	case DisplayModeExtend:
		err = m.switchModeExtend()
	case DisplayModeOnlyOne:
		err = m.switchModeOnlyOne(name)
	case DisplayModeCustom:
		err = m.switchModeCustom(name)
		if err == nil {
			m.setCurrentCustomId(name)
		}
	default:
		err = errors.New("invalid mode")
	}

	if err == nil {
		m.setDisplayMode(mode)
	} else {
		logger.Warningf("failed to switch mode %v %v: %v", mode, name, err)
	}
	return
}

func (m *Manager) setDisplayMode(mode byte) {
	m.setPropDisplayMode(mode)
	m.settings.SetEnum(gsKeyDisplayMode, int32(mode))
}

func (m *Manager) save() (err error) {
	logger.Debug("save")
	id := getMonitorsId(m.monitorMap)
	if id == "" {
		err = errors.New("no output connected")
		return
	}

	screenCfg := m.config[id]
	if screenCfg == nil {
		screenCfg = &ScreenConfig{}
		m.config[id] = screenCfg
	}
	monitors := m.getConnectedMonitors()

	if len(monitors) == 1 {
		screenCfg.Single = monitors[0].toConfig()
	} else {
		screenCfg.setMonitorConfigs(m.DisplayMode, m.CurrentCustomId,
			toMonitorConfigs(monitors, m.Primary))
	}

	err = m.config.save(configFile)
	if err != nil {
		logger.Warning(err)
		return
	}
	m.markClean()
	return nil
}

func (m *Manager) markClean() {
	m.monitorMapMu.Lock()
	for _, monitor := range m.monitorMap {
		monitor.backup = nil
	}
	m.monitorMapMu.Unlock()

	m.PropsMu.Lock()
	m.setPropHasChanged(false)
	m.PropsMu.Unlock()
}

func (m *Manager) getConnectedMonitors() []*Monitor {
	m.monitorMapMu.Lock()
	var monitors []*Monitor
	for _, monitor := range m.monitorMap {
		if monitor.Connected {
			monitors = append(monitors, monitor)
		}
	}
	m.monitorMapMu.Unlock()
	return monitors
}

func (m *Manager) setCurrentCustomId(name string) {
	m.setPropCurrentCustomId(name)
	m.settings.SetString(gsKeyCustomMode, name)
}

func (m *Manager) applyMonitorConfigs(configs []*MonitorConfig) error {
	var primaryOutput randr.Output
	for output, monitor := range m.monitorMap {
		monitorCfg := getMonitorConfigByUuid(configs, monitor.uuid)
		if monitorCfg == nil {
			monitor.enable(false)
		} else {
			if monitorCfg.Primary && monitorCfg.Enabled {
				primaryOutput = output
			}
			monitor.enable(monitorCfg.Enabled)
			monitor.setPosition(monitorCfg.X, monitorCfg.Y)
			monitor.setRotation(monitorCfg.Rotation)
			monitor.setReflect(monitorCfg.Reflect)
			mode := monitor.selectMode(monitorCfg.Width, monitorCfg.Height, monitorCfg.RefreshRate)
			monitor.setMode(mode)
		}
	}
	err := m.applyMonitorsConfig()
	if err != nil {
		return err
	}
	if primaryOutput == 0 {
		primaryOutput = randr.Output(getMinIDMonitor(m.getConnectedMonitors()).ID)
		// TODO get enabled monitor
	}
	err = m.setOutputPrimary(primaryOutput)
	if err != nil {
		return err
	}
	return nil
}

func (m *Manager) getCustomIdList() []string {
	id := getMonitorsId(m.monitorMap)

	screenCfg := m.config[id]
	if screenCfg == nil {
		return nil
	}

	result := make([]string, len(screenCfg.Custom))
	for idx, custom := range screenCfg.Custom {
		result[idx] = custom.Name
	}
	sort.Strings(result)
	return result
}

func getMonitorsId(monitorMap map[randr.Output]*Monitor) string {
	var ids []string
	for _, monitor := range monitorMap {
		if !monitor.Connected {
			continue
		}
		ids = append(ids, monitor.uuid)
	}
	if len(ids) == 0 {
		return ""
	}
	sort.Strings(ids)
	return strings.Join(ids, monitorsIdDelimiter)
}

func (m *Manager) updatePropMonitors() {
	monitors := m.getConnectedMonitors()
	sort.Slice(monitors, func(i, j int) bool {
		return monitors[i].ID < monitors[j].ID
	})
	paths := make([]dbus.ObjectPath, len(monitors))
	for i, monitor := range monitors {
		paths[i] = monitor.getPath()
	}
	m.setPropMonitors(paths)
}

func (m *Manager) modifyConfigName(name, newName string) (err error) {
	if name == "" || newName == "" {
		err = errors.New("name is empty")
		return
	}

	id := getMonitorsId(m.monitorMap)
	if id == "" {
		err = errors.New("no output connected")
		return
	}

	screenCfg := m.config[id]
	if screenCfg == nil {
		err = errors.New("not found screen config")
		return
	}

	var customConfig *CustomModeConfig
	for _, custom := range screenCfg.Custom {
		if custom.Name == name {
			customConfig = custom
			break
		}
	}
	if customConfig == nil {
		err = fmt.Errorf("not found custom mode config %q", name)
		return
	}
	if name == newName {
		return nil
	}

	for _, custom := range screenCfg.Custom {
		if custom.Name == newName {
			err = fmt.Errorf("same name config %q already exists", newName)
			return
		}
	}

	customConfig.Name = newName
	m.setPropCustomIdList(m.getCustomIdList())
	if name == m.CurrentCustomId {
		m.setCurrentCustomId(newName)
	}

	err = m.config.save(configFile)
	if err != nil {
		logger.Warning(err)
		return
	}

	return nil
}

func (m *Manager) deleteCustomMode(name string) (err error) {
	logger.Debugf("deleteCustomMode %q", name)
	if name == "" {
		err = errors.New("name is empty")
		return
	}

	id := getMonitorsId(m.monitorMap)
	if id == "" {
		err = errors.New("no output connected")
		return
	}

	if m.isCustomModeBeingUsed(name) {
		err = errors.New("the custom mode is being used")
		return
	}

	screenCfg := m.config[id]
	if screenCfg == nil {
		err = errors.New("not found screen config")
		return
	}

	var customConfigs []*CustomModeConfig
	foundName := false
	for _, custom := range screenCfg.Custom {
		if custom.Name == name {
			foundName = true
		} else {
			customConfigs = append(customConfigs, custom)
		}
	}

	if !foundName {
		logger.Warning("not found custom mode config:", name)
		// not found
		return nil
	}

	screenCfg.Custom = customConfigs

	if m.CurrentCustomId == name {
		m.setCurrentCustomId("")
	}

	m.setPropCustomIdList(m.getCustomIdList())
	err = m.config.save(configFile)
	if err != nil {
		return err
	}

	return nil
}

func (m *Manager) isCustomModeBeingUsed(name string) bool {
	return m.DisplayMode == DisplayModeCustom &&
		m.CurrentCustomId == name
}

func (m *Manager) getScreenSize() (sw, sh uint16) {
	var w, h int
	for _, monitor := range m.monitorMap {
		if !monitor.Connected || !monitor.Enabled {
			continue
		}

		width := monitor.CurrentMode.Width
		height := monitor.CurrentMode.Height

		if needSwapWidthHeight(monitor.Rotation) {
			width, height = height, width
		}

		w1 := int(monitor.X) + int(width)
		h1 := int(monitor.Y) + int(height)

		if w < w1 {
			w = w1
		}
		if h < h1 {
			h = h1
		}
	}
	if w > math.MaxUint16 {
		w = math.MaxUint16
	}
	if h > math.MaxUint16 {
		h = math.MaxUint16
	}
	sw = uint16(w)
	sh = uint16(h)
	return
}

func (m *Manager) initTouchMap() {
	value := m.settings.GetString(gsKeyMapOutput)
	if len(value) == 0 {
		m.TouchMap = make(map[string]string)
		m.setPropTouchMap(m.TouchMap)
		return
	}

	err := jsonUnmarshal(value, &m.TouchMap)
	if err != nil {
		logger.Warningf("[initTouchMap] unmarshal (%s) failed: %v",
			value, err)
		return
	}

	for touch, output := range m.TouchMap {
		m.doSetTouchMap(touch, output)
	}
	m.setPropTouchMap(m.TouchMap)
}

func (m *Manager) doSetTouchMap(output, touch string) error {
	// TODO
	monitors := m.getConnectedMonitors()
	found := false
	for _, monitor := range monitors {
		if monitor.Name == output {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("Invalid output name: %s", output)
	}

	// TODO: check touch validity
	return doAction(fmt.Sprintf("xinput --map-to-output %s %s", touch, output))
}

func (dpy *Manager) associateTouch(outputName, touch string) error {
	if dpy.TouchMap[touch] == outputName {
		return nil
	}

	err := dpy.doSetTouchMap(outputName, touch)
	if err != nil {
		logger.Warning("[AssociateTouch] set failed:", err)
		return err
	}

	dpy.TouchMap[touch] = outputName
	dpy.setPropTouchMap(dpy.TouchMap)
	dpy.settings.SetString(gsKeyMapOutput, jsonMarshal(dpy.TouchMap))
	return nil
}
