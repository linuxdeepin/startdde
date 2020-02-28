package display

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"pkg.deepin.io/dde/startdde/wl_display/org_kde_kwin/output_management"

	"github.com/davecgh/go-spew/spew"
	"github.com/dkolbly/wl"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
	"pkg.deepin.io/dde/startdde/display/brightness"
	"pkg.deepin.io/gir/gio-2.0"
	dbus "pkg.deepin.io/lib/dbus1"
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
	service          *dbusutil.Service
	xConn            *x.Conn
	display          *wl.Display
	registry         *wl.Registry
	management       *output_management.Outputmanagement
	devicesWg        sync.WaitGroup
	devicesAllDone   bool
	devicesAllDoneMu sync.Mutex

	PropsMu              sync.RWMutex
	config               Config
	recommendScaleFactor float64
	//modes                []randr.ModeInfo
	//monitorMap           map[randr.Output]*Monitor
	monitorMap   map[uint32]*Monitor
	monitorMapMu sync.Mutex
	//crtcMap              map[randr.Crtc]*randr.GetCrtcInfoReply
	//crtcMapMu            sync.Mutex
	//outputMap            map[randr.Output]*randr.GetOutputInfoReply
	//outputMapMu          sync.Mutex
	//configTimestamp      x.Timestamp
	settings   *gio.Settings
	monitorsId string

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
	name   string
	Width  uint16
	Height uint16
	Rate   float64
}

func newManager(service *dbusutil.Service) *Manager {
	conn, err := x.NewConn()
	if err != nil {
		logger.Fatal(err)
	}
	m := &Manager{
		xConn:      conn,
		service:    service,
		monitorMap: make(map[uint32]*Monitor),
	}

	m.settings = gio.NewSettings(gsSchemaDisplay)
	m.DisplayMode = uint8(m.settings.GetEnum(gsKeyDisplayMode))
	if m.DisplayMode == DisplayModeUnknow {
		m.DisplayMode = DisplayModeExtend
	}
	m.CurrentCustomId = m.settings.GetString(gsKeyCustomMode)

	display, err := wl.Connect("")
	if err != nil {
		logger.Fatal(err)
	}
	m.display = display

	err = m.registerGlobals()
	if err != nil {
		logger.Warning(err)
	}

	if m.management == nil {
		logger.Warning("m.management is nil")
	}

	go m.loopDispatch()
	m.devicesWg.Wait()
	m.devicesAllDoneMu.Lock()
	m.devicesAllDone = true
	m.devicesAllDoneMu.Unlock()

	m.monitorsId = m.getMonitorsId()
	logger.Debug("after registerGlobals", m.monitorsId, m.monitorMap)
	m.recommendScaleFactor = m.calcRecommendedScaleFactor()
	m.updateScreenSize()

	m.config = loadConfig()
	m.CustomIdList = m.getCustomIdList()
	return m
}

func (m *Manager) applyDisplayMode() {
	logger.Debug("applyDisplayMode")
	monitors := m.getConnectedMonitors()
	var err error
	if len(monitors) == 1 {
		// 单屏
		screenCfg := m.getScreenConfig()
		var config *MonitorConfig
		if screenCfg.Single != nil {
			config = screenCfg.Single
		} else {
			config = monitors[0].toConfig()
			config.Enabled = true
			config.Primary = true
			mode := monitors[0].BestMode
			config.X = 0
			config.Y = 0
			config.Width = mode.Width
			config.Height = mode.Height
			config.RefreshRate = mode.Rate
			config.Rotation = randr.RotationRotate0
		}

		err = m.applyConfigs([]*MonitorConfig{config})
		if err != nil {
			logger.Warning("failed to apply configs:", err)
		}
		return
	}

	switch m.DisplayMode {
	case DisplayModeCustom:
		err = m.switchModeCustom(m.CurrentCustomId)
	case DisplayModeMirror:
		err = m.switchModeMirror()
	case DisplayModeExtend:
		err = m.switchModeExtend("")
	case DisplayModeOnlyOne:
		err = m.switchModeOnlyOne("")
	}

	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) init() {
	brightness.InitBacklightHelper()
	m.initBrightness()
	m.applyDisplayMode()
	m.initTouchMap()
}

func (m *Manager) calcRecommendedScaleFactor() float64 {
	minScaleFactor := 3.0
	monitors := m.getConnectedMonitors()
	if len(monitors) == 0 {
		return 1.0
	}
	for _, monitor := range monitors {
		scaleFactor := calcRecommendedScaleFactor(float64(monitor.Width), float64(monitor.Height),
			float64(monitor.MmWidth), float64(monitor.MmHeight))
		if minScaleFactor > scaleFactor {
			minScaleFactor = scaleFactor
		}
	}

	return minScaleFactor
}

func calcRecommendedScaleFactor(widthPx, heightPx, widthMm, heightMm float64) float64 {
	if widthMm == 0 || heightMm == 0 {
		return 1
	}

	lenPx := math.Hypot(widthPx, heightPx)
	lenMm := math.Hypot(widthMm, heightMm)

	lenPxStd := math.Hypot(1920, 1080)
	lenMmStd := math.Hypot(477, 268)

	const a = 0.00158
	fix := (lenMm - lenMmStd) * (lenPx / lenPxStd) * a
	scaleFactor := (lenPx/lenMm)/(lenPxStd/lenMmStd) + fix

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

//func (m *Manager) getModeInfo(mode randr.Mode) ModeInfo {
//	if mode == 0 {
//		return ModeInfo{}
//	}
//	for _, modeInfo := range m.modes {
//		if modeInfo.Id == uint32(mode) {
//			return toModeInfo(modeInfo)
//		}
//	}
//	return ModeInfo{}
//}
//
//func (m *Manager) getModeInfos(modes []randr.Mode) []ModeInfo {
//	var result []ModeInfo
//	for _, mode := range modes {
//		modeInfo := m.getModeInfo(mode)
//		if modeInfo.Id != 0 {
//			result = append(result, modeInfo)
//		}
//	}
//	result = filterModeInfos(result)
//	return result
//}

func (m *Manager) handleOutputDeviceDone(device *outputDeviceHandler) {
	logger.Debug("handleOutputDeviceDone", device.id)
	m.monitorMapMu.Lock()
	monitor, ok := m.monitorMap[device.id]
	m.monitorMapMu.Unlock()
	if ok {
		m.updateMonitor(monitor)
		m.updatePropMonitors()
		return
	}

	err := m.addMonitor(device)
	if err != nil {
		logger.Warning(err)
	} else {
		m.updatePropMonitors()
	}

	m.devicesAllDoneMu.Lock()
	if !m.devicesAllDone {
		m.devicesWg.Done()
	} else {
		m.updateMonitorsId()
		m.updateScreenSize()
	}
	m.devicesAllDoneMu.Unlock()
}

func (m *Manager) handleOutputDeviceEnabled(device *outputDeviceHandler) {
	logger.Debug("handleOutputDeviceEnabled", device.id)

	m.monitorMapMu.Lock()
	monitor, ok := m.monitorMap[device.id]
	m.monitorMapMu.Unlock()
	if ok {
		monitor.PropsMu.Lock()
		monitor.setPropEnabled(device.enabled)
		monitor.PropsMu.Unlock()
		return
	}
}

func (m *Manager) addMonitor(device *outputDeviceHandler) error {
	logger.Debugf("addMonitor %#v", device)
	monitor := &Monitor{
		m:         m,
		service:   m.service,
		ID:        device.id,
		device:    device,
		Connected: true,
	}
	monitor.uuid = device.uuid
	monitor.Enabled = device.enabled
	monitor.X = int16(device.x)
	monitor.Y = int16(device.y)
	monitor.MmWidth = uint32(device.physicalWidth)
	monitor.MmHeight = uint32(device.physicalHeight)
	monitor.Name = device.name()

	// mode info
	monitor.Modes = device.getModes()
	monitor.BestMode = device.getBestMode()
	monitor.PreferredModes = []ModeInfo{monitor.BestMode}
	monitor.CurrentMode = device.getCurrentMode()
	monitor.Width = monitor.CurrentMode.Width
	monitor.Height = monitor.CurrentMode.Height
	monitor.RefreshRate = monitor.CurrentMode.Rate

	monitor.Rotations = []uint16{randr.RotationRotate0, randr.RotationRotate90,
		randr.RotationRotate180, randr.RotationRotate270}
	monitor.Rotation = device.rotation()

	monitor.Reflect = 0 //TODO

	err := m.service.Export(monitor.getPath(), monitor)
	if err != nil {
		return err
	}
	m.monitorMapMu.Lock()
	m.monitorMap[device.id] = monitor
	m.monitorMapMu.Unlock()
	return nil
}

func (m *Manager) removeMonitor(id uint32) {
	m.monitorMapMu.Lock()

	monitor := m.monitorMap[id]
	delete(m.monitorMap, id)
	m.monitorMapMu.Unlock()

	err := m.service.StopExport(monitor)
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) updateMonitor(monitor *Monitor) {
	device := monitor.device
	logger.Debug("updateMonitor", monitor.ID)
	monitor.PropsMu.Lock()

	monitor.uuid = device.uuid
	monitor.setPropEnabled(device.enabled)
	monitor.setPropX(int16(device.x))
	monitor.setPropY(int16(device.y))
	monitor.setPropMmWidth(uint32(device.physicalWidth))
	monitor.setPropMmHeight(uint32(device.physicalHeight))
	monitor.setPropName(device.name())
	// mode info
	monitor.setPropModes(device.getModes())
	monitor.setPropBestMode(device.getBestMode())
	monitor.setPropPreferredModes([]ModeInfo{monitor.BestMode})
	monitor.setPropCurrentMode(device.getCurrentMode())
	monitor.setPropWidth(monitor.CurrentMode.Width)
	monitor.setPropHeight(monitor.CurrentMode.Height)
	monitor.setPropRefreshRate(monitor.CurrentMode.Rate)
	monitor.setPropRotation(device.rotation())
	//monitor.setPropReflect(0) //TODO

	logger.Debugf("updateMonitor id: %d, x:%d, y: %d, width: %d, height: %d",
		monitor.ID, monitor.X, monitor.Y, monitor.Width, monitor.Height)
	rect := monitor.getRect()
	monitor.PropsMu.Unlock()

	// if monitor is primary, update primary rect
	m.PropsMu.Lock()
	if monitor.Name == m.Primary {
		logger.Debugf("updateMonitor update primary rect: %+v", rect)
		m.setPropPrimaryRect(rect)
	}
	m.PropsMu.Unlock()

	m.updateScreenSize()
}

func (m *Manager) updateScreenSize() {
	var screenWidth uint16
	var screenHeight uint16

	m.monitorMapMu.Lock()
	for _, monitor := range m.monitorMap {
		if screenWidth < uint16(monitor.X)+monitor.Width {
			screenWidth = uint16(monitor.X) + monitor.Width
		}
		if screenHeight < uint16(monitor.Y)+monitor.Height {
			screenHeight = uint16(monitor.Y) + monitor.Height
		}
	}
	m.monitorMapMu.Unlock()

	m.setPropScreenWidth(screenWidth)
	m.setPropScreenHeight(screenHeight)
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
				mode, _ = getFirstModeBySize(monitor.Modes, maxSize.width, maxSize.height)
			}
			monitor.setMode(mode)
			monitor.setPosition(0, 0)
			monitor.setRotation(randr.RotationRotate0)
			monitor.setReflect(0)

		} else {
			monitor.enable(false)
		}
	}

	err = m.apply()
	if err != nil {
		return
	}

	monitor0 := getMinIDMonitor(m.getConnectedMonitors())
	if monitor0 != nil {
		err = m.setMonitorPrimary(monitor0)
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

//func (m *Manager) getScreenSize1() screenSize {
//	width, height := m.getScreenSize()
//	mmWidth := uint32(float64(width) / 3.792)
//	mmHeight := uint32(float64(height) / 3.792)
//	return screenSize{
//		width:    width,
//		height:   height,
//		mmWidth:  mmWidth,
//		mmHeight: mmHeight,
//	}
//}

//func (m *Manager) setScreenSize(ss screenSize) error {
//	root := m.xConn.GetDefaultScreen().Root
//	err := randr.SetScreenSizeChecked(m.xConn, root, ss.width, ss.height, ss.mmWidth,
//		ss.mmHeight).Check(m.xConn)
//	logger.Debugf("set screen size %dx%d, mm: %dx%d",
//		ss.width, ss.height, ss.mmWidth, ss.mmHeight)
//	return err
//}

//type crtcConfig struct {
//	crtc    randr.Crtc
//	outputs []randr.Output
//
//	x        int16
//	y        int16
//	rotation uint16
//	mode     randr.Mode
//}

func (m *Manager) apply() error {
	configuration, err := m.management.CreateConfiguration()
	if err != nil {
		return err
	}
	defer func() {
		err := configuration.Destroy()
		if err != nil {
			logger.Warning(err)
		}
	}()

	for _, monitor := range m.monitorMap {
		dev := monitor.device.core
		if monitor.Enabled {
			err = configuration.Enable(dev, 1)
			if err != nil {
				return err
			}

			err = configuration.Mode(dev, int32(monitor.CurrentMode.Id))
			if err != nil {
				return err
			}

			err = configuration.Position(dev, int32(monitor.X), int32(monitor.Y))
			if err != nil {
				return err
			}

			err = configuration.Transform(dev,
				toOutputDeviceTransform(monitor.Rotation, monitor.Reflect))
			if err != nil {
				return err
			}

		} else {
			err = configuration.Enable(dev, 0)
			if err != nil {
				return err
			}
		}
	}

	appliedChan := make(chan output_management.OutputconfigurationAppliedEvent)
	ah := outputCfgAppliedHandler{appliedChan}
	configuration.AddAppliedHandler(ah)

	failedChan := make(chan output_management.OutputconfigurationFailedEvent)
	fh := outputCfgFailedHandler{failedChan}
	configuration.AddFailedHandler(fh)

	defer func() {
		configuration.RemoveAppliedHandler(ah)
		configuration.RemoveFailedHandler(fh)
	}()

	err = configuration.Apply()
	if err != nil {
		return err
	}

loop:
	for {
		select {
		case <-appliedChan:
			logger.Debugf("applied success")
			break loop

		case m.display.Context().Dispatch() <- struct{}{}:
		case <-failedChan:
			logger.Warning("apply failed")
			err = errors.New("output configuration apply failed")
			break loop
		}
	}

	for _, monitor := range m.monitorMap {
		m.updateMonitor(monitor)
	}

	return err
}

//func (m *Manager) apply() error {
//	x.GrabServer(m.xConn)
//	defer func() {
//		err := x.UngrabServerChecked(m.xConn).Check(m.xConn)
//		if err != nil {
//			logger.Warning(err)
//		}
//	}()
//
//	monitorCrtcCfgMap := make(map[randr.Output]crtcConfig)
//	for output, monitor := range m.monitorMap {
//		if monitor.Enabled {
//			crtc := monitor.crtc
//			if crtc == 0 {
//				crtc = m.findFreeCrtc(output)
//				if crtc == 0 {
//					return errors.New("failed to find free crtc")
//				}
//			}
//			monitorCrtcCfgMap[output] = crtcConfig{
//				crtc:     crtc,
//				x:        monitor.X,
//				y:        monitor.Y,
//				mode:     randr.Mode(monitor.CurrentMode.Id),
//				rotation: monitor.Rotation | monitor.Reflect,
//				outputs:  []randr.Output{output},
//			}
//		} else {
//			if monitor.crtc != 0 {
//				monitorCrtcCfgMap[output] = crtcConfig{
//					crtc:     monitor.crtc,
//					rotation: randr.RotationRotate0,
//				}
//			}
//		}
//	}
//
//	m.PropsMu.RLock()
//	cfgTs := m.configTimestamp
//	m.PropsMu.RUnlock()
//
//	screenSize := m.getScreenSize1()
//
//	m.crtcMapMu.Lock()
//	for crtc, crtcInfo := range m.crtcMap {
//		rect := getCrtcRect(crtcInfo)
//		logger.Debugf("crtc %v, rect: %+v", crtc, rect)
//		if int(rect.X)+int(rect.Width) <= int(screenSize.width) &&
//			int(rect.Y)+int(rect.Height) <= int(screenSize.height) {
//			// 适合
//		} else {
//			// 不适合新的屏幕大小，如果已经启用，则需要禁用它
//			if len(crtcInfo.Outputs) == 0 {
//				continue
//			}
//			logger.Debugf("disable crtc %v, it's outputs: %v", crtc, crtcInfo.Outputs)
//			err := m.disableCrtc(crtc, cfgTs)
//			if err != nil {
//				return err
//			}
//		}
//	}
//	m.crtcMapMu.Unlock()
//
//	err := m.setScreenSize(screenSize)
//	if err != nil {
//		return err
//	}
//
//	for output, monitor := range m.monitorMap {
//		crtcCfg, ok := monitorCrtcCfgMap[output]
//		if !ok {
//			continue
//		}
//		err := monitor.applyConfig(crtcCfg)
//		if err != nil {
//			return err
//		}
//
//		outputInfo, err := m.updateOutputInfo(output)
//		if err != nil {
//			logger.Warning(err)
//		}
//		if outputInfo.Crtc != 0 {
//			_, err = m.updateCrtcInfo(outputInfo.Crtc)
//			if err != nil {
//				logger.Warning(err)
//			}
//		}
//		m.updateMonitor(output, outputInfo)
//
//		if monitor.Enabled {
//			m.PropsMu.Lock()
//			value, ok := m.Brightness[monitor.Name]
//			if !ok {
//				value = 1
//				m.Brightness[monitor.Name] = value
//			}
//			m.PropsMu.Unlock()
//
//			err = m.setMonitorBrightness(monitor, value)
//			if err != nil {
//				logger.Warningf("failed to set brightness for %s: %v", monitor.Name, err)
//			}
//		}
//	}
//
//	return nil
//}

//func (m *Manager) disableCrtc(crtc randr.Crtc, cfgTs x.Timestamp) error {
//	setCfg, err := randr.SetCrtcConfig(m.xConn, crtc, 0, cfgTs,
//		0, 0, 0, randr.RotationRotate0, nil).Reply(m.xConn)
//	if err != nil {
//		return err
//	}
//	if setCfg.Status != randr.SetConfigSuccess {
//		return fmt.Errorf("failed to disable crtc %d: %v",
//			crtc, getRandrStatusStr(setCfg.Status))
//	}
//	return nil
//}

func (m *Manager) setMonitorPrimary(monitor *Monitor) error {
	rect := monitor.getRect()
	m.PropsMu.Lock()
	m.setPropPrimary(monitor.Name)
	m.setPropPrimaryRect(rect)
	m.PropsMu.Unlock()
	return nil
}

//func (m *Manager) setOutputPrimary(output randr.Output) error {
//	logger.Debug("set output primary", output)
//	root := m.xConn.GetDefaultScreen().Root
//	return randr.SetOutputPrimaryChecked(m.xConn, root, output).Check(m.xConn)
//}
//
//func (m *Manager) getOutputPrimary() (randr.Output, error) {
//	root := m.xConn.GetDefaultScreen().Root
//	reply, err := randr.GetOutputPrimary(m.xConn, root).Reply(m.xConn)
//	if err != nil {
//		return 0, err
//	}
//	return reply.Output, nil
//}

// 更新属性 Primary 和 PrimaryRect
//func (m *Manager) updateOutputPrimary() {
//	pOutput, err := m.getOutputPrimary()
//	if err != nil {
//		logger.Warning(err)
//		return
//	}
//
//	var newPrimary string
//	var newRect x.Rectangle
//
//	if pOutput != 0 {
//		m.outputMapMu.Lock()
//
//		for output, outputInfo := range m.outputMap {
//			if pOutput != output {
//				continue
//			}
//
//			newPrimary = outputInfo.Name
//
//			if outputInfo.Crtc == 0 {
//				logger.Warning("new primary output crtc is 0")
//			} else {
//				m.crtcMapMu.Lock()
//				crtcInfo := m.crtcMap[outputInfo.Crtc]
//				m.crtcMapMu.Unlock()
//				if crtcInfo == nil {
//					logger.Warning("crtcInfo is nil")
//				} else {
//					newRect = getCrtcRect(crtcInfo)
//				}
//			}
//			break
//		}
//
//		m.outputMapMu.Unlock()
//	}
//
//	m.PropsMu.Lock()
//	m.setPropPrimary(newPrimary)
//	m.setPropPrimaryRect(newRect)
//	m.PropsMu.Unlock()
//
//	logger.Debugf("updateOutputPrimary name: %q, rect: %#v", newPrimary, newRect)
//}

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

		err := m.setMonitorPrimary(monitor0)
		if err != nil {
			return err
		}

		screenCfg.setMonitorConfigs(m.DisplayMode, m.CurrentCustomId, configs)

		err = m.saveConfig()
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("invalid display mode %v", m.DisplayMode)
	}
	return nil
}

func (m *Manager) switchModeExtend(primary string) (err error) {
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

	err = m.apply()
	if err != nil {
		return
	}

	if primary != "" {
		for _, m := range monitors {
			if m.Enabled && m.Name == primary {
				monitor0 = m
			}
		}
	}

	if monitor0 != nil {
		err = m.setMonitorPrimary(monitor0)
		if err != nil {
			return
		}
	}

	return
}

func (m *Manager) getScreenConfig() *ScreenConfig {
	id := m.getMonitorsId()
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

	err = m.apply()
	if err != nil {
		return
	}

	// set primary output
	err = m.setMonitorPrimary(monitor0)
	if err != nil {
		return
	}

	if needSaveCfg {
		screenCfg.setMonitorConfigs(DisplayModeOnlyOne, "",
			toMonitorConfigs(m.getConnectedMonitors(), monitor0.Name))

		err = m.saveConfig()
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
		err = m.applyConfigs(configs)
		return
	}

	// 自定义配置不存在时，尽可能使用当前的显示配置。
	// hasDisabled 表示是否有连接了但是未启用的屏幕，如果有，为了开启显示器，
	// 切换到扩展模式，以扩展模式初始化自定义配置。
	hasDisabled := false
	monitors := m.getConnectedMonitors()
	for _, m := range monitors {
		if !m.Enabled {
			hasDisabled = true
			break
		}
	}

	if hasDisabled {
		err = m.switchModeExtend(m.Primary)
		if err != nil {
			return
		}
	}

	screenCfg.setMonitorConfigs(DisplayModeCustom, name,
		toMonitorConfigs(m.getConnectedMonitors(), m.Primary))

	err = m.saveConfig()
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
		err = m.switchModeExtend("")
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
	id := m.getMonitorsId()
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

	err = m.saveConfig()
	if err != nil {
		return err
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

func (m *Manager) getConnectedMonitors() Monitors {
	m.monitorMapMu.Lock()
	var monitors Monitors
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

func (m *Manager) applyConfigs(configs []*MonitorConfig) error {
	logger.Debug("applyConfigs", spew.Sdump(configs))
	var primaryMonitor *Monitor
	for _, monitor := range m.monitorMap {
		monitorCfg := getMonitorConfigByUuid(configs, monitor.uuid)
		if monitorCfg == nil {
			monitor.enable(false)
		} else {
			if monitorCfg.Primary && monitorCfg.Enabled {
				primaryMonitor = monitor
			}
			monitor.enable(monitorCfg.Enabled)
			monitor.setPosition(monitorCfg.X, monitorCfg.Y)
			monitor.setRotation(monitorCfg.Rotation)
			monitor.setReflect(monitorCfg.Reflect)

			width := monitorCfg.Width
			height := monitorCfg.Height
			if needSwapWidthHeight(monitorCfg.Rotation) {
				width, height = height, width
			}
			mode := monitor.selectMode(width, height, monitorCfg.RefreshRate)
			monitor.setMode(mode)
		}
	}
	err := m.apply()
	if err != nil {
		return err
	}
	if primaryMonitor == nil {
		primaryMonitor = getMinIDMonitor(m.getConnectedMonitors())
	}
	err = m.setMonitorPrimary(primaryMonitor)
	if err != nil {
		return err
	}
	return nil
}

func (m *Manager) getCustomIdList() []string {
	id := m.getMonitorsId()

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

func (m *Manager) getMonitorsId() string {
	var ids []string
	m.monitorMapMu.Lock()
	for _, monitor := range m.monitorMap {
		if !monitor.Connected {
			continue
		}
		ids = append(ids, monitor.uuid)
	}
	m.monitorMapMu.Unlock()
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

	id := m.getMonitorsId()
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

	err = m.saveConfig()
	if err != nil {
		return err
	}

	return nil
}

func (m *Manager) deleteCustomMode(name string) (err error) {
	logger.Debugf("deleteCustomMode %q", name)
	if name == "" {
		err = errors.New("name is empty")
		return
	}

	id := m.getMonitorsId()
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
	err = m.saveConfig()
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

func (m *Manager) associateTouch(outputName, touch string) error {
	if m.TouchMap[touch] == outputName {
		return nil
	}

	err := m.doSetTouchMap(outputName, touch)
	if err != nil {
		logger.Warning("[AssociateTouch] set failed:", err)
		return err
	}

	m.TouchMap[touch] = outputName
	m.setPropTouchMap(m.TouchMap)
	m.settings.SetString(gsKeyMapOutput, jsonMarshal(m.TouchMap))
	return nil
}

func (m *Manager) saveConfig() error {
	logger.Debug("save config")
	dir := filepath.Dir(configFile)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(configVersionFile, []byte(configVersion), 0644)
	if err != nil {
		return err
	}

	err = m.config.save(configFile)
	if err != nil {
		return err
	}
	return nil
}
