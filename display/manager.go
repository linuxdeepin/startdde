package display

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	dbus "github.com/godbus/dbus"
	dgesture "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.gesture"
	inputdevices "github.com/linuxdeepin/go-dbus-factory/com.deepin.system.inputdevices"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.dbus"
	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
	"golang.org/x/xerrors"
	"pkg.deepin.io/dde/api/dxinput"
	dxutil "pkg.deepin.io/dde/api/dxinput/utils"
	"pkg.deepin.io/dde/startdde/display/brightness"
	"pkg.deepin.io/dde/startdde/display/utils"
	gio "pkg.deepin.io/gir/gio-2.0"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/dbusutil/gsprop"
)

const (
	DisplayModeCustom uint8 = iota
	DisplayModeMirror
	DisplayModeExtend
	DisplayModeOnlyOne
	DisplayModeUnknow
)

const (
	// 不调整色温
	ColorTemperatureModeNormal int32 = iota
	// 自动调整色温
	ColorTemperatureModeAuto
	// 手动调整色温
	ColorTemperatureModeManual
)

const (
	gsSchemaDisplay  = "com.deepin.dde.display"
	gsKeyDisplayMode = "display-mode"
	gsKeyBrightness  = "brightness"
	gsKeySetter      = "brightness-setter"
	gsKeyMapOutput   = "map-output"
	gsKeyRateFilter  = "rate-filter"
	//gsKeyPrimary     = "primary"
	gsKeyCustomMode             = "current-custom-mode"
	gsKeyColorTemperatureMode   = "color-temperature-mode"
	gsKeyColorTemperatureManual = "color-temperature-manual"
	gsKeyRotateScreenTimeDelay  = "rotate-screen-time-delay"
	customModeDelim             = "+"
	monitorsIdDelimiter         = ","
	defaultTemperatureMode      = ColorTemperatureModeNormal
	defaultTemperatureManual    = 6500

	cmdTouchscreenDialogBin = "/usr/lib/deepin-daemon/dde-touchscreen-dialog"
)

const (
	priorityEDP = iota
	priorityDP
	priorityHDMI
	priorityDVI
	priorityVGA
	priorityOther
)

var (
	monitorPriority = map[string]int{
		"edp":  priorityEDP,
		"dp":   priorityDP,
		"hdmi": priorityHDMI,
		"dvi":  priorityDVI,
		"vga":  priorityVGA,
	}
)

var (
	rotationScreenValue = map[string]uint16{
		"normal": randr.RotationRotate0,
		"left":   randr.RotationRotate270, // 屏幕重力旋转左转90
		"right":  randr.RotationRotate90,  // 屏幕重力旋转右转90
	}
)

type touchscreenMapValue struct {
	OutputName string
	Auto       bool
}

// return pciId => (size => rates)
type RateFilterMap map[string]map[string][]float64

//go:generate dbusutil-gen -output display_dbusutil.go -import github.com/godbus/dbus,github.com/linuxdeepin/go-x11-client -type Manager,Monitor manager.go monitor.go
//go:generate dbusutil-gen em -type Manager,Monitor
type Manager struct {
	service                  *dbusutil.Service
	sysBus                   *dbus.Conn
	ofdbus                   ofdbus.DBus
	inputDevices             inputdevices.InputDevices
	xConn                    *x.Conn
	PropsMu                  sync.RWMutex
	config                   Config
	recommendScaleFactor     float64
	modes                    []randr.ModeInfo
	builtinMonitor           *Monitor
	builtinMonitorMu         sync.Mutex
	candidateBuiltinMonitors []*Monitor // 候补的
	monitorMap               map[randr.Output]*Monitor
	monitorMapMu             sync.Mutex
	crtcMap                  map[randr.Crtc]*randr.GetCrtcInfoReply
	crtcMapMu                sync.Mutex
	outputMap                map[randr.Output]*randr.GetOutputInfoReply
	outputMapMu              sync.Mutex
	configTimestamp          x.Timestamp
	settings                 *gio.Settings
	monitorsId               string
	modeChanged              bool
	info                     ConnectInfo
	rotationFinishChanged    bool
	rotationScreenTimer      *time.Timer
	hasBuiltinMonitor        bool

	// dbusutil-gen: equal=nil
	Monitors []dbus.ObjectPath
	// dbusutil-gen: equal=nil
	CustomIdList []string
	HasChanged   bool
	DisplayMode  byte
	// dbusutil-gen: equal=nil
	Brightness map[string]float64
	// dbusutil-gen: equal=nil
	Touchscreens dxTouchscreens
	// dbusutil-gen: equal=nil
	TouchMap       map[string]string
	touchscreenMap map[string]touchscreenMapValue

	CurrentCustomId        string
	Primary                string
	PrimaryRect            x.Rectangle
	ScreenWidth            uint16
	ScreenHeight           uint16
	MaxBacklightBrightness uint32

	// method of adjust color temperature according to time and location
	ColorTemperatureMode int32
	// adjust color temperature by manual adjustment
	ColorTemperatureManual int32
	// 存在gsetting中的色温模式
	gsColorTemperatureMode int32
	// 存在gsetting中的色温值
	gsColorTemperatureManual int32
	// 存在gsetting中的延时旋转屏幕值
	gsRotateScreenTimeDelay gsprop.Int
}

type ModeInfo struct {
	Id     uint32
	name   string
	Width  uint16
	Height uint16
	Rate   float64
}

var _xConn *x.Conn

var _hasRandr1d2 bool // 是否 randr 版本大于等于 1.2

func Init(xConn *x.Conn) {
	_xConn = xConn
	randrVersion, err := randr.QueryVersion(xConn, randr.MajorVersion, randr.MinorVersion).Reply(xConn)
	if err != nil {
		logger.Warning(err)
	} else {
		logger.Debugf("randr version %d.%d", randrVersion.ServerMajorVersion, randrVersion.ServerMinorVersion)
		if randrVersion.ServerMajorVersion > 1 ||
			(randrVersion.ServerMajorVersion == 1 && randrVersion.ServerMinorVersion >= 2) {
			_hasRandr1d2 = true
		}
		logger.Debug("has randr1.2:", _hasRandr1d2)
	}
}

type monitorSizeInfo struct {
	width, height     uint16
	mmWidth, mmHeight uint32
}

func GetRecommendedScaleFactor() float64 {
	if !_hasRandr1d2 {
		return 1
	}
	resources, err := getScreenResources(_xConn)
	if err != nil {
		logger.Warning(err)
		return 1
	}
	cfgTs := resources.ConfigTimestamp

	var monitors []*monitorSizeInfo
	for _, output := range resources.Outputs {
		outputInfo, err := randr.GetOutputInfo(_xConn, output, cfgTs).Reply(_xConn)
		if err != nil {
			logger.Warningf("get output %v info failed: %v", output, err)
			return 1.0
		}
		if outputInfo.Connection != randr.ConnectionConnected {
			continue
		}

		crtcInfo, err := randr.GetCrtcInfo(_xConn, outputInfo.Crtc, cfgTs).Reply(_xConn)
		if err != nil {
			logger.Warningf("get crtc %v info failed: %v", outputInfo.Crtc, err)
			return 1.0
		}
		monitors = append(monitors, &monitorSizeInfo{
			mmWidth:  outputInfo.MmWidth,
			mmHeight: outputInfo.MmHeight,
			width:    crtcInfo.Width,
			height:   crtcInfo.Height,
		})
	}

	if len(monitors) == 0 {
		return 1.0
	}

	minScaleFactor := 3.0
	for _, monitor := range monitors {
		scaleFactor := calcRecommendedScaleFactor(float64(monitor.width), float64(monitor.height),
			float64(monitor.mmWidth), float64(monitor.mmHeight))
		if minScaleFactor > scaleFactor {
			minScaleFactor = scaleFactor
		}
	}
	return minScaleFactor
}

func newManager(service *dbusutil.Service) *Manager {
	m := &Manager{
		service:    service,
		monitorMap: make(map[randr.Output]*Monitor),
	}

	chassis, err := getComputeChassis()
	if err != nil {
		logger.Warning(err)
	}
	if chassis == "laptop" || chassis == "all-in-one" {
		m.hasBuiltinMonitor = true
	}

	m.settings = gio.NewSettings(gsSchemaDisplay)
	m.DisplayMode = uint8(m.settings.GetEnum(gsKeyDisplayMode))
	if m.DisplayMode == DisplayModeUnknow {
		m.DisplayMode = DisplayModeExtend
	}
	m.CurrentCustomId = m.settings.GetString(gsKeyCustomMode)
	m.gsColorTemperatureManual = m.settings.GetInt(gsKeyColorTemperatureManual)
	m.gsColorTemperatureMode = m.settings.GetEnum(gsKeyColorTemperatureMode)
	m.gsRotateScreenTimeDelay.Bind(m.settings, gsKeyRotateScreenTimeDelay)
	m.ColorTemperatureManual = defaultTemperatureManual
	m.ColorTemperatureMode = defaultTemperatureMode

	m.xConn = _xConn

	screen := m.xConn.GetDefaultScreen()
	m.ScreenWidth = screen.WidthInPixels
	m.ScreenHeight = screen.HeightInPixels

	if _hasRandr1d2 {
		resources, err := getScreenResources(m.xConn)
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

		m.initBuiltinMonitor()
		m.monitorsId = m.getMonitorsId()
		m.updatePropMonitors()
		m.updateOutputPrimary()

	} else {
		// randr 版本低于 1.2
		screenInfo, err := randr.GetScreenInfo(m.xConn, screen.Root).Reply(m.xConn)
		if err == nil {
			monitor, err := m.addMonitorFallback(screenInfo)
			if err == nil {
				m.updatePropMonitors()
				m.setPropPrimary("Default")
				m.setPropPrimaryRect(x.Rectangle{
					X:      monitor.X,
					Y:      monitor.Y,
					Width:  monitor.Width,
					Height: monitor.Height,
				})
			} else {
				logger.Warning(err)
			}
		} else {
			logger.Warning(err)
		}
	}

	m.setPropMaxBacklightBrightness(uint32(brightness.GetMaxBacklightBrightness()))

	m.sysBus, err = dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
	}

	sigLoop := dbusutil.NewSignalLoop(m.sysBus, 10)
	sigLoop.Start()

	m.ofdbus = ofdbus.NewDBus(m.sysBus)
	m.ofdbus.InitSignalExt(sigLoop, true)

	m.inputDevices = inputdevices.NewInputDevices(m.sysBus)
	m.inputDevices.InitSignalExt(sigLoop, true)

	selfObj, err := login1.NewSession(m.sysBus, "/org/freedesktop/login1/session/self")
	if err != nil {
		logger.Warningf("connect login1 self sesion failed! %v", err)
		return m
	}
	id, err := selfObj.Id().Get(0)
	if err != nil {
		logger.Warningf("get self session id failed! %v", err)
		return m
	}
	managerObj := login1.NewManager(m.sysBus)
	path, err := managerObj.GetSession(0, id)
	if err != nil {
		logger.Warningf("get session path %s failed! %v", id, err)
		return m
	}
	sessionObj, err := login1.NewSession(m.sysBus, path)
	if err != nil {
		logger.Warningf("connect login1 sesion %s failed! %v", path, err)
		return m
	}

	sessionObj.InitSignalExt(sigLoop, true)
	err = sessionObj.Active().ConnectChanged(func(hasValue, value bool) {
		if !hasValue || !value {
			return
		}
		m.handleTouchscreenChanged()
		m.showTouchscreenDialogs()
	})
	if err != nil {
		logger.Warningf("prop active ConnectChanged failed! %v", err)
	}

	return m
}

// initBuiltinMonitor 初始化内置显示器。
func (m *Manager) initBuiltinMonitor() {
	if !m.hasBuiltinMonitor {
		return
	}
	// 从配置文件获取内置显示器名称
	builtinMonitorName, err := loadBuiltinMonitorConfig(builtinMonitorConfigFile)
	if err != nil {
		logger.Warning(err)
	}
	monitors := m.getConnectedMonitors()
	if builtinMonitorName != "" {
		for _, monitor := range monitors {
			if monitor.Name == builtinMonitorName {
				m.builtinMonitor = monitor
			}
		}
	}

	// 从配置文件获取的内置显示器还存在，信任配置文件，可以返回了
	if m.builtinMonitor != nil {
		return
	}
	builtinMonitorName = ""

	var rest []*Monitor
	for _, monitor := range monitors {
		name := strings.ToLower(monitor.Name)
		if strings.HasPrefix(name, "vga") {
			// 忽略 vga 开头的
		} else if strings.HasPrefix(name, "edp") {
			// 如果是 edp 开头，直接成为 builtinMonitor
			rest = []*Monitor{monitor}
			break
		} else {
			rest = append(rest, monitor)
		}
	}

	if len(rest) == 1 {
		m.builtinMonitor = rest[0]
		builtinMonitorName = m.builtinMonitor.Name
	} else if len(rest) > 1 {
		// 选择 id 最小的显示器作为内置显示器，这个结果不太准确，但却无可奈何。
		// 不保存 builtinMonitor 到配置文件中，由于 builtinMonitorName 为空，就会清空配置文件。
		m.builtinMonitor = getMinIDMonitor(rest)
		// 把剩余显示器列表 rest 设置到候选内置显示器列表。
		m.candidateBuiltinMonitors = rest
	}

	// 保存内置显示器配置文件
	err = saveBuiltinMonitorConfig(builtinMonitorConfigFile, builtinMonitorName)
	if err != nil {
		logger.Warning("failed to save builtin monitor config:", err)
	}
}

// updateBuiltinMonitorOnDisconnected 在发现显示器断开连接时，更新内置显示器，因为断开的不可能是内置显示器。
// 参数 id 是断开的显示器的 id。
func (m *Manager) updateBuiltinMonitorOnDisconnected(id uint32) {
	m.builtinMonitorMu.Lock()
	defer m.builtinMonitorMu.Unlock()

	if len(m.candidateBuiltinMonitors) < 2 {
		return
	}
	m.candidateBuiltinMonitors = monitorsRemove(m.candidateBuiltinMonitors, id)
	if len(m.candidateBuiltinMonitors) == 1 {
		// 当只剩下一个候补时能自动成为真的 builtin monitor
		m.builtinMonitor = m.candidateBuiltinMonitors[0]
		m.candidateBuiltinMonitors = nil
		// 保存内置显示器配置文件
		err := saveBuiltinMonitorConfig(builtinMonitorConfigFile, m.builtinMonitor.Name)
		if err != nil {
			logger.Warning("failed to save builtin monitor config:", err)
		}
	}
}

// monitorsRemove 删除 monitors 列表中显示器 id 为参数 id 的显示器，返回新列表
func monitorsRemove(monitors []*Monitor, id uint32) []*Monitor {
	var result []*Monitor
	for _, m := range monitors {
		if m.ID != id {
			result = append(result, m)
		}
	}
	return result
}

func (m *Manager) applyDisplayMode(needInitColorTemperature bool, options applyOptions) {
	// 对于 randr 版本低于 1.2 时，不做操作
	if !_hasRandr1d2 {
		return
	}
	monitors := m.getConnectedMonitors()
	if len(monitors) == 0 {
		// 拔掉所有显示器
		logger.Debug("applyDisplayMode without any monitor，return")
		return
	}
	var err error
	if len(monitors) == 1 {
		// 单屏情况
		screenCfg := m.getScreenConfig()
		config := new(SingleModeConfig)
		if screenCfg.Single != nil && screenCfg.Single.Monitor != nil {
			// 已有单屏配置
			config = screenCfg.Single
		} else {
			// 没有单屏配置
			config.Monitor = monitors[0].toConfig()
			config.Monitor.Enabled = true
			config.Monitor.Primary = true
			mode := monitors[0].BestMode
			config.Monitor.X = 0
			config.Monitor.Y = 0
			config.Monitor.Width = mode.Width
			config.Monitor.Height = mode.Height
			config.Monitor.RefreshRate = mode.Rate
			config.Monitor.Brightness = 1
			config.ColorTemperatureMode = defaultTemperatureMode
			config.ColorTemperatureManual = defaultTemperatureManual
			config.Monitor.Rotation = randr.RotationRotate0
			screenCfg.Single = config
		}

		// 应用单屏配置
		err = m.applySingleConfigs(config, options)
		if err != nil {
			logger.Warning("failed to apply configs:", err)
		}
		// 拔插屏幕时需要根据配置文件重置色温
		if needInitColorTemperature {
			m.initColorTemperature()
		}

		err = m.saveConfig()
		if err != nil {
			logger.Warning(err)
		}
		m.syncBrightness()

		return
	}
	// 多屏情况
	switch m.DisplayMode {
	case DisplayModeMirror:
		err = m.switchModeMirror(options)
	case DisplayModeExtend:
		err = m.switchModeExtend(options)
	case DisplayModeOnlyOne:
		err = m.switchModeOnlyOne("", options)
	}

	if err != nil {
		logger.Warning(err)
	}
	// 拔插屏幕时需要根据配置文件重置色温
	if needInitColorTemperature {
		m.initColorTemperature()
	}
	m.syncBrightness()
}

func (m *Manager) init() {
	brightness.InitBacklightHelper()
	m.initBrightness()
	// 在获取屏幕亮度之后再加载配置文件，版本迭代时把上次的亮度值写入配置文件。
	m.config = loadConfig(m)
	// 重启 startdde 读取上一次设置，不需要重置色温。
	m.applyDisplayMode(false, nil)
	m.listenEvent() // 等待 applyDisplayMode 执行完成再开启监听 X 事件
	if m.builtinMonitor != nil {
		m.builtinMonitor.latestRotationValue = m.builtinMonitor.Rotation
		m.listenRotateSignal() // 监听屏幕旋转信号
	} else {
		// 没有内建屏,不监听内核信号
		logger.Info("built-in screen does not exist")
	}
}

// initColorTemperature 初始化色温设置，名字不太好，不在初始化时，也调用了。
func (m *Manager) initColorTemperature() {
	method := m.ColorTemperatureMode
	err := m.setMethodAdjustCCT(method)
	if err != nil {
		logger.Error(err)
		return
	}
}

// calcRecommendedScaleFactor 计算推荐的缩放比
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

func getScreenResources(xConn *x.Conn) (*randr.GetScreenResourcesReply, error) {
	root := xConn.GetDefaultScreen().Root
	resources, err := randr.GetScreenResources(xConn, root).Reply(xConn)
	return resources, err
}

func (m *Manager) getScreenResourcesCurrent() (*randr.GetScreenResourcesCurrentReply, error) {
	root := m.xConn.GetDefaultScreen().Root
	resources, err := randr.GetScreenResourcesCurrent(m.xConn, root).Reply(m.xConn)
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

// getModeInfo 从 Manager.modes 模式列表中找 id 和参数 mode 相同的模式，然后转换类型为 ModeInfo。
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
	result = filterModeInfosByRefreshRate(filterModeInfos(result), m.getRateFilter())
	return result
}

func getScreenInfoSize(screenInfo *randr.GetScreenInfoReply) (size randr.ScreenSize, err error) {
	sizeId := screenInfo.SizeID
	if int(sizeId) < len(screenInfo.Sizes) {
		size = screenInfo.Sizes[sizeId]
	} else {
		err = fmt.Errorf("size id out of range: %d %d", sizeId, len(screenInfo.Sizes))
	}
	return
}

func (m *Manager) addMonitorFallback(screenInfo *randr.GetScreenInfoReply) (*Monitor, error) {
	output := randr.Output(1)

	size, err := getScreenInfoSize(screenInfo)
	if err != nil {
		return nil, err
	}

	monitor := &Monitor{
		service:   m.service,
		m:         m,
		ID:        uint32(output),
		Name:      "Default",
		Connected: true,
		MmWidth:   uint32(size.MWidth),
		MmHeight:  uint32(size.MHeight),
		Enabled:   true,
		Width:     size.Width,
		Height:    size.Height,
	}

	err = m.service.Export(monitor.getPath(), monitor)
	if err != nil {
		return nil, err
	}
	m.monitorMapMu.Lock()
	m.monitorMap[output] = monitor
	m.monitorMapMu.Unlock()
	return monitor, nil
}

func (m *Manager) updateMonitorFallback(screenInfo *randr.GetScreenInfoReply) *Monitor {
	output := randr.Output(1)
	m.monitorMapMu.Lock()
	monitor, ok := m.monitorMap[output]
	m.monitorMapMu.Unlock()
	if !ok {
		return nil
	}

	size, err := getScreenInfoSize(screenInfo)
	if err != nil {
		logger.Warning(err)
		return nil
	}

	monitor.setPropWidth(size.Width)
	monitor.setPropHeight(size.Height)
	monitor.setPropMmWidth(uint32(size.MWidth))
	monitor.setPropMmHeight(uint32(size.MHeight))
	return monitor
}

// addMonitor 在 Manager.monitorMap 增加显示器，在 dbus 上导出显示器对象
func (m *Manager) addMonitor(output randr.Output, outputInfo *randr.GetOutputInfoReply) error {
	m.monitorMapMu.Lock()
	_, ok := m.monitorMap[output]
	m.monitorMapMu.Unlock()
	if ok {
		return nil
	}

	m.initConnectInfo()

	connected := outputInfo.Connection == randr.ConnectionConnected
	m.setAndSaveConnectInfo(connected, outputInfo)
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

	edid, err := utils.GetOutputEDID(m.xConn, output)
	if err != nil {
		logger.Warning(err)
	}
	manufacturer, model := parseEDID(edid)
	monitor := &Monitor{
		service:      m.service,
		m:            m,
		ID:           uint32(output),
		Name:         outputInfo.Name,
		Connected:    connected,
		MmWidth:      outputInfo.MmWidth,
		MmHeight:     outputInfo.MmHeight,
		Enabled:      enabled,
		crtc:         outputInfo.Crtc,
		uuid:         getOutputUUID(outputInfo.Name, edid),
		Manufacturer: manufacturer,
		Model:        model,
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
	monitor.oldRotation = monitor.Rotation

	err = m.service.Export(monitor.getPath(), monitor)
	if err != nil {
		return err
	}
	m.monitorMapMu.Lock()
	m.monitorMap[output] = monitor
	m.monitorMapMu.Unlock()
	return nil
}

// updateMonitor 根据 outputInfo 中的信息更新 dbus 上的 Monitor 对象的属性
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

	var edid []byte
	if connected {
		edid, err = utils.GetOutputEDID(m.xConn, output)
		if err != nil {
			logger.Warning(err)
		}

		m.initConnectInfo()
		m.setAndSaveConnectInfo(connected, outputInfo)
	} else {
		m.info.Connects[outputInfo.Name] = connected
		err := doSaveCache(&m.info, cacheFile)
		if err != nil {
			logger.Warning("doSaveCache failed", err)
		}
		m.updateBuiltinMonitorOnDisconnected(monitor.ID)
	}
	manufacturer, model := parseEDID(edid)
	uuid := getOutputUUID(outputInfo.Name, edid)
	monitor.PropsMu.Lock()
	monitor.uuid = uuid
	monitor.crtc = outputInfo.Crtc
	monitor.setPropManufacturer(manufacturer)
	monitor.setPropModel(model)
	monitor.setPropConnected(connected)
	monitor.setPropEnabled(enabled)
	monitor.setPropModes(m.getModeInfos(outputInfo.Modes))
	monitor.setPropBestMode(monitor.getBestMode(m, outputInfo))
	monitor.setPropPreferredModes([]ModeInfo{monitor.BestMode})
	monitor.setPropMmWidth(outputInfo.MmWidth)
	monitor.setPropMmHeight(outputInfo.MmHeight)
	monitor.PropsMu.Unlock()
	m.updateMonitorCrtcInfo(monitor, crtcInfo)
}

func (m *Manager) updateMonitorCrtcInfo(monitor *Monitor, crtcInfo *randr.GetCrtcInfoReply) {
	if crtcInfo == nil {
		return
	}
	if crtcInfo.Mode != 0 {
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

func (m *Manager) switchModeMirrorAux() (err error, monitor0 *Monitor) {
	logger.Debug("switch mode mirror")
	monitors := m.getConnectedMonitors()
	commonSizes := getMonitorsCommonSizes(monitors)
	if len(commonSizes) == 0 {
		err = errors.New("not found common size")
		return
	}
	maxSize := getMaxAreaSize(commonSizes)
	for _, monitor := range m.monitorMap {
		if monitor.Connected {
			monitor.enable(true)
			var mode ModeInfo
			mode = getFirstModeBySize(monitor.Modes, maxSize.width, maxSize.height)
			monitor.setMode(mode)
			monitor.setPosition(0, 0)
			monitor.setRotation(randr.RotationRotate0)
			monitor.setReflect(0)
			monitor.setBrightness(1)

		} else {
			monitor.enable(false)
		}
	}

	m.ColorTemperatureMode = defaultTemperatureMode
	m.ColorTemperatureManual = defaultTemperatureManual

	err = m.apply()
	if err != nil {
		return
	}

	monitor0 = m.getDefaultPrimaryMonitor(m.getConnectedMonitors())
	if monitor0 != nil {
		err = m.setOutputPrimary(randr.Output(monitor0.ID))
		if err != nil {
			return
		}
	}
	return
}

func (m *Manager) switchModeMirror(options applyOptions) (err error) {
	screenCfg := m.getScreenConfig()
	configs := screenCfg.getModeConfigs(DisplayModeMirror)
	logger.Debug("switchModeMirror")
	if len(configs.Monitors) > 0 {
		err = m.applyConfigs(configs, options)
		return
	}

	err, monitor0 := m.switchModeMirrorAux()
	if err != nil {
		return
	}

	screenCfg.setModeConfigs(DisplayModeMirror, m.ColorTemperatureMode, m.ColorTemperatureManual, toMonitorConfigs(m.getConnectedMonitors(), monitor0.Name))

	return m.saveConfig()
}

type screenSize struct {
	width    uint16
	height   uint16
	mmWidth  uint32
	mmHeight uint32
}

// getScreenSize1 计算出需要的屏幕尺寸
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

type applyOptions map[string]interface{}

const (
	optionDisableCrtc = "disableCrtc"
)

func (m *Manager) apply(optionsSlice ...applyOptions) error {
	logger.Debug("call apply")

	optDisableCrtc := false
	if len(optionsSlice) > 0 {
		// optionsSlice 应该最多只有一个元素
		options := optionsSlice[0]
		optDisableCrtc, _ = options[optionDisableCrtc].(bool)
	}

	x.GrabServer(m.xConn)
	defer func() {
		err := x.UngrabServerChecked(m.xConn).Check(m.xConn)
		if err != nil {
			logger.Warning(err)
		}
		logger.Debug("apply return")
	}()

	monitorCrtcCfgMap := make(map[randr.Output]crtcConfig)
	// 根据 monitor 的配置，准备 crtc 配置到 monitorCrtcCfgMap
	for output, monitor := range m.monitorMap {
		monitor.dumpInfoForDebug()
		if monitor.Enabled {
			// 启用显示器
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
			// 禁用显示器
			if monitor.crtc != 0 {
				// 禁用此 crtc，把它的 outputs 设置为空。
				monitorCrtcCfgMap[output] = crtcConfig{
					crtc:     monitor.crtc,
					rotation: randr.RotationRotate0,
				}
			}
		}
	}

	m.PropsMu.RLock()
	cfgTs := m.configTimestamp
	// 当前的屏幕大小
	prevScreenSize := screenSize{width: m.ScreenWidth, height: m.ScreenHeight}
	m.PropsMu.RUnlock()

	// 未来的，apply 之后的屏幕所需尺寸
	screenSize := m.getScreenSize1()
	logger.Debugf("screen size after apply: %+v", screenSize)

	monitors := m.getConnectedMonitors()
	m.crtcMapMu.Lock()
	for crtc, crtcInfo := range m.crtcMap {
		rect := getCrtcRect(crtcInfo)
		logger.Debugf("crtc %v, crtcInfo: %+v", crtc, crtcInfo)

		// 是否考虑临时禁用 crtc
		shouldDisable := false

		if m.modeChanged {
			// 显示模式切换了
			logger.Debugf("should disable crtc %v because of mode changed", crtc)
			shouldDisable = true
		} else if optDisableCrtc {
			// NOTE: 如果接入了双屏，断开一个屏幕，让另外的屏幕都暂时禁用，来避免桌面壁纸的闪烁问题（突然黑一下，然后很快恢复），
			// 这么做是为了兼顾修复 pms bug 83875 和 94116。
			// 但是对于 bug 94116，依然保留问题：先断开再连接显示器，桌面壁纸依然有闪烁问题。
			logger.Debugf("should disable crtc %v because of optDisableCrtc is true", crtc)
			shouldDisable = true
		} else if int(rect.X)+int(rect.Width) > int(screenSize.width) ||
			int(rect.Y)+int(rect.Height) > int(screenSize.height) {
			// 当前 crtc 的尺寸超过了未来的屏幕尺寸，必须禁用
			logger.Debugf("should disable crtc %v because of the size of crtc exceeds the size of future screen", crtc)
			shouldDisable = true
		} else {
			monitor := monitors.GetByCrtc(crtc)
			if monitor != nil && monitor.Enabled {
				if rect.X != monitor.X || rect.Y != monitor.Y ||
					rect.Width != monitor.Width || rect.Height != monitor.Height ||
					crtcInfo.Rotation != monitor.Rotation|monitor.Reflect {
					// crtc 的参数将发生改变, 这里的 monitor 包含了 crtc 未来的状态。
					logger.Debugf("should disable crtc %v because of the parameters of crtc changed", crtc)
					shouldDisable = true
				}
			}
		}

		if shouldDisable && len(crtcInfo.Outputs) > 0 {
			logger.Debugf("disable crtc %v, it's outputs: %v", crtc, crtcInfo.Outputs)
			err := m.disableCrtc(crtc, cfgTs)
			if err != nil {
				return err
			}
		}
	}
	m.crtcMapMu.Unlock()
	m.modeChanged = false

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

		if monitor.Enabled {
			m.PropsMu.Lock()
			value := monitor.Brightness
			if value == 0 {
				value = 1
			}
			m.PropsMu.Unlock()

			go func(mon *Monitor) {
				err = m.setMonitorBrightness(mon, value)
				if err != nil {
					logger.Warningf("failed to set brightness for %s: %v", mon.Name, err)
				}
			}(monitor) // 用局部变量作闭包上值
		}
		err = monitor.emitPropChangedBrightness(monitor.Brightness)
		if err != nil {
			logger.Error("emitPropChangedBrightness failed")
		}
	}

	// NOTE: 为配合文件管理器修一个 bug：
	// 双屏左右摆放，两屏幕有相同最大分辨率，设置左屏为主屏，自定义模式下两屏合并、拆分循环切换，此时如果不发送 PrimaryRect 属性
	// 改变信号，将在从合并切换到拆分时，右屏的桌面壁纸没有绘制，是全黑的。可能是所有显示器的分辨率都没有改变，桌面 dde-desktop
	// 程序收不到相关信号。
	// 此时屏幕尺寸被改变是很好的特征，发送一个 PrimaryRect 属性改变通知桌面 dde-desktop 程序让它重新绘制桌面壁纸，以消除 bug。
	// TODO: 这不是一个很好的方案，后续可与桌面程序方面沟通改善方案。
	if prevScreenSize.width != screenSize.width || prevScreenSize.height != screenSize.height {
		// screen size changed
		// NOTE: 不能直接用 prevScreenSize != screenSize 进行比较，因为 screenSize 类型不止 width 和 height 字段。
		logger.Debug("[apply] screen size changed, force emit prop changed for PrimaryRect")
		m.PropsMu.RLock()
		rect := m.PrimaryRect
		m.PropsMu.RUnlock()
		err := m.emitPropChangedPrimaryRect(rect)
		if err != nil {
			logger.Warning(err)
		}
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

	logger.Debugf("updateOutputPrimary name: %q, rect: %+v", newPrimary, newRect)
}

func (m *Manager) setPrimary(name string) error {
	switch m.DisplayMode {
	case DisplayModeMirror:
		return errors.New("not allow set primary in mirror mode")

	case DisplayModeOnlyOne:
		return m.switchModeOnlyOne(name, nil)

	case DisplayModeExtend:
		screenCfg := m.getScreenConfig()
		configs := screenCfg.getMonitorConfigs(m.DisplayMode)

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

		screenCfg.setMonitorConfigs(m.DisplayMode, configs)

		err = m.saveConfig()
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("invalid display mode %v", m.DisplayMode)
	}
	return nil
}

func (m *Manager) switchModeExtend(options applyOptions) (err error) {
	logger.Debug("switch mode extend")
	screenCfg := m.getScreenConfig()
	modeConfigs := screenCfg.getModeConfigs(DisplayModeExtend)

	if len(modeConfigs.Monitors) > 0 {
		err = m.applyConfigs(modeConfigs, options)
		return
	}

	var monitors []*Monitor
	for _, monitor := range m.monitorMap {
		monitors = append(monitors, monitor)
	}

	var xOffset int
	var monitor0 *Monitor
	//先获取主屏
	monitor0 = m.getDefaultPrimaryMonitor(m.getConnectedMonitors())

	sortMonitorsByPrimaryAndID(monitors, monitor0)

	for _, monitor := range monitors {
		if monitor.Connected {
			monitor.enable(true)
			var mode ModeInfo
			mode = monitor.BestMode
			monitor.setMode(mode)

			if xOffset > math.MaxInt16 {
				xOffset = math.MaxInt16
			}
			monitor.setPosition(int16(xOffset), 0)
			monitor.setRotation(randr.RotationRotate0)
			monitor.setReflect(0)
			monitor.setBrightness(1)

			xOffset += int(monitor.Width)
		} else {
			monitor.enable(false)
		}
	}
	m.ColorTemperatureMode = defaultTemperatureMode
	m.ColorTemperatureManual = defaultTemperatureManual

	err = m.apply(options)
	if err != nil {
		return
	}

	if monitor0 != nil {
		err = m.setOutputPrimary(randr.Output(monitor0.ID))
		if err != nil {
			logger.Warning("failed to set primary output:", err)
			return
		}

		screenCfg.setModeConfigs(DisplayModeExtend, m.ColorTemperatureMode, m.ColorTemperatureManual, toMonitorConfigs(m.getConnectedMonitors(), monitor0.Name))

		err = m.saveConfig()
		if err != nil {
			logger.Warning("failed to save config:", err)
			return
		}
	}

	return
}

// getScreenConfig 根据当前的 MonitorsId 返回不同的屏幕配置，不同 MonitorsId 则屏幕配置不同。
// MonitorsId 代表了已连接了哪些显示器。
func (m *Manager) getScreenConfig() *ScreenConfig {
	id := m.getMonitorsId()
	screenCfg := m.config[id]
	if screenCfg == nil {
		screenCfg = &ScreenConfig{}
		m.config[id] = screenCfg
	}
	return screenCfg
}

func (m *Manager) switchModeOnlyOne(name string, options applyOptions) (err error) {
	logger.Debug("switch mode only one", name)

	screenCfg := m.getScreenConfig()
	configs := screenCfg.getModeConfigs(DisplayModeOnlyOne)

	if name == "" && configs == nil {
		return errors.New("")
	}

	var monitor0 *Monitor
	if name != "" {
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
		for _, cfg := range configs.Monitors {
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
			var mode ModeInfo
			var rotation uint16 = randr.RotationRotate0
			var reflect uint16
			if len(configs.Monitors) > 0 {
				cfg := getMonitorConfigByUuid(configs.Monitors, monitor.uuid)
				if cfg != nil {
					mode = monitor.selectMode(cfg.Width, cfg.Height, cfg.RefreshRate)
					rotation = cfg.Rotation
					reflect = cfg.Reflect

					if cfg.Brightness == 0 {
						cfg.Brightness = 1
					}
					monitor.setBrightness(cfg.Brightness)
					m.ColorTemperatureMode = cfg.ColorTemperatureMode
					m.ColorTemperatureManual = cfg.ColorTemperatureManual
					monitor.colorTemperatureManual = cfg.ColorTemperatureManual
					monitor.colorTemperatureMode = cfg.ColorTemperatureMode
				} else {
					mode = monitor.BestMode
					monitor.setBrightness(1)
					m.ColorTemperatureMode = defaultTemperatureMode
					m.ColorTemperatureManual = defaultTemperatureManual
					monitor.colorTemperatureManual = defaultTemperatureManual
					monitor.colorTemperatureMode = defaultTemperatureMode
				}

				//启动时从配置文件读取
			} else {
				mode = monitor.BestMode
				m.ColorTemperatureMode = defaultTemperatureMode
				m.ColorTemperatureManual = defaultTemperatureManual
				monitor.colorTemperatureManual = defaultTemperatureManual
				monitor.colorTemperatureMode = defaultTemperatureMode
				monitor.setBrightness(1)
			}

			monitor.setMode(mode)
			monitor.setPosition(0, 0)
			monitor.setRotation(rotation)
			monitor.setReflect(reflect)

		} else {
			monitor.enable(false)
			//配置文件中有数据使用配置文件中的数据,否则设置亮度为1
			if len(configs.Monitors) > 0 {
				cfg := getMonitorConfigByUuid(configs.Monitors, monitor.uuid)
				if cfg != nil {
					monitor.setPropBrightness(cfg.Brightness)
					monitor.colorTemperatureManual = cfg.ColorTemperatureManual
					monitor.colorTemperatureMode = cfg.ColorTemperatureMode
				}
			} else {
				monitor.setPropBrightness(1)
				monitor.colorTemperatureManual = defaultTemperatureManual
				monitor.colorTemperatureMode = defaultTemperatureMode
			}
		}
	}

	err = m.apply(options)
	if err != nil {
		return
	}
	// set primary output
	err = m.setOutputPrimary(randr.Output(monitor0.ID))
	if err != nil {
		return
	}

	screenCfg.setModeConfigs(DisplayModeOnlyOne, m.ColorTemperatureMode, m.ColorTemperatureManual,
		toMonitorConfigs(m.getConnectedMonitors(), monitor0.Name))
	err = m.saveConfig()
	if err != nil {
		return
	}

	return
}

func (m *Manager) switchMode(mode byte, name string) (err error) {
	switch mode {
	case DisplayModeMirror:
		err = m.switchModeMirror(nil)
	case DisplayModeExtend:
		err = m.switchModeExtend(nil)
	case DisplayModeOnlyOne:
		err = m.switchModeOnlyOne(name, nil)
	default:
		err = errors.New("invalid mode")
	}
	if err == nil {
		m.setDisplayMode(mode)
		m.syncBrightness()
		err = m.setMethodAdjustCCT(m.ColorTemperatureMode)
		if err != nil {
			logger.Debug("SetMethodAdjustCCT", err)
			return err
		}
		m.modeChanged = true
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
		screenCfg.Single.Monitor = monitors[0].toConfig()
		screenCfg.Single.Monitor.ColorTemperatureMode = defaultTemperatureMode
		screenCfg.Single.Monitor.ColorTemperatureManual = defaultTemperatureManual
		screenCfg.Single.ColorTemperatureMode = m.ColorTemperatureMode
		screenCfg.Single.ColorTemperatureManual = m.ColorTemperatureManual
	} else {
		var primaryName string
		//当为扩展屏幕的时候，设置默认屏为配置文件中默认屏幕
		if DisplayModeExtend == m.DisplayMode {
			for _, i := range screenCfg.Extend.Monitors {
				if i.Primary {
					primaryName = i.Name
				}
			}
		}
		//没找到主屏或者模式非扩展模式，则取默认值
		if len(primaryName) == 0 {
			primaryName = m.Primary
		}

		logger.Debugf("DisplayMode:<%d>,Primary Name<%s>", m.DisplayMode, primaryName)
		screenCfg.setMonitorConfigs(m.DisplayMode, toMonitorConfigs(monitors, primaryName))
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

// 复制和扩展时触发
func (m *Manager) applyConfigs(configs *ModeConfigs, options applyOptions) error {
	logger.Debug("applyConfigs", spew.Sdump(configs), options)
	var primaryOutput randr.Output
	for output, monitor := range m.monitorMap {
		monitorCfg := getMonitorConfigByUuid(configs.Monitors, monitor.uuid)
		if monitorCfg == nil {
			monitor.enable(false)
		} else {
			if monitorCfg.Primary && monitorCfg.Enabled {
				primaryOutput = output
			}
			//所有可设置的值都设置为配置文件中的值
			monitor.setPosition(monitorCfg.X, monitorCfg.Y)
			monitor.setRotation(monitorCfg.Rotation)
			monitor.setReflect(monitorCfg.Reflect)
			//切换到新模式,用之前的亮度值
			logger.Debug("monitorCfg.Brightness[monitorCfg.name]", monitorCfg.Name, monitorCfg.Brightness)
			monitor.setBrightness(monitorCfg.Brightness)

			width := monitorCfg.Width
			height := monitorCfg.Height
			if needSwapWidthHeight(monitorCfg.Rotation) {
				width, height = height, width
			}
			mode := monitor.selectMode(width, height, monitorCfg.RefreshRate)
			monitor.setMode(mode)
			monitor.enable(true)
			m.ColorTemperatureMode = monitorCfg.ColorTemperatureMode
			m.ColorTemperatureManual = monitorCfg.ColorTemperatureManual
			monitor.colorTemperatureMode = monitorCfg.ColorTemperatureMode
			monitor.colorTemperatureManual = monitorCfg.ColorTemperatureManual
		}
	}

	logger.Debug("ColorTemperatureMode = ,ColorTemperatureManual = ", m.ColorTemperatureMode, m.ColorTemperatureManual)

	err := m.apply(options)
	if err != nil {
		return err
	}
	if primaryOutput == 0 {
		primaryOutput = randr.Output(m.getDefaultPrimaryMonitor(m.getConnectedMonitors()).ID)
	}
	err = m.setOutputPrimary(primaryOutput)
	if err != nil {
		return err
	}

	return nil
}

func (m *Manager) applySingleConfigs(config *SingleModeConfig, options applyOptions) error {
	logger.Debug("applyConfigs", spew.Sdump(config), options)
	var primaryOutput randr.Output
	for output, monitor := range m.monitorMap {
		monitorCfg := getMonitorConfigByUuid([]*MonitorConfig{config.Monitor}, monitor.uuid)
		if monitorCfg == nil {
			monitor.enable(false)
		} else {
			if monitorCfg.Primary && monitorCfg.Enabled {
				primaryOutput = output
			}
			//所有可设置的值都设置为配置文件中的值
			monitor.setPosition(monitorCfg.X, monitorCfg.Y)
			monitor.setRotation(monitorCfg.Rotation)
			monitor.setReflect(monitorCfg.Reflect)
			//切换到新模式,用之前的亮度值
			logger.Debug("monitorCfg.Brightness[monitorCfg.name]", monitorCfg.Name, monitorCfg.Brightness)
			monitor.setBrightness(monitorCfg.Brightness)

			width := monitorCfg.Width
			height := monitorCfg.Height
			if needSwapWidthHeight(monitorCfg.Rotation) {
				width, height = height, width
			}
			mode := monitor.selectMode(width, height, monitorCfg.RefreshRate)
			monitor.setMode(mode)
			monitor.enable(true)
		}
	}

	logger.Debug("ColorTemperatureMode = ,ColorTemperatureManual = ", m.ColorTemperatureMode, m.ColorTemperatureManual)
	m.ColorTemperatureMode = config.ColorTemperatureMode
	m.ColorTemperatureManual = config.ColorTemperatureManual

	err := m.apply(options)
	if err != nil {
		return err
	}
	if primaryOutput == 0 {
		primaryOutput = randr.Output(m.getDefaultPrimaryMonitor(m.getConnectedMonitors()).ID)
	}
	err = m.setOutputPrimary(primaryOutput)
	if err != nil {
		return err
	}

	return nil
}

func (m *Manager) getDefaultPrimaryMonitor(monitors []*Monitor) *Monitor {
	if len(monitors) == 0 {
		return nil
	}
	builtinMonitor := m.getBuiltinMonitor()
	if builtinMonitor != nil {
		return builtinMonitor
	}

	monitor := m.getPriorMonitor(monitors)
	if monitor != nil {
		return monitor
	}

	return m.getMinLastConnectedTimeMonitor(monitors)
}

// getPriorMonitor 获取优先级最高的显示器，用于作为主屏。
func (m *Manager) getPriorMonitor(monitors []*Monitor) *Monitor {
	var monitor *Monitor
	priority := priorityOther
	for _, v := range monitors {
		name := m.getPortType(v.Name)
		p, ok := monitorPriority[name]

		// 不在列表中的话，留空，以便最后通过连接时间来设置主屏幕
		if !ok {
			continue
		}

		// 优先级的数值越小，级别越高。
		if p < priority {
			monitor = v
			priority = p
			continue
		}

		// 当接口类型相同，若一个是默认显示器，继续让它作为默认显示器
		if p == priority {
			if m.info.LastConnectedTimes == nil {
				if m.Primary == v.Name {
					monitor = v
					priority = p
				}
			} else {
				return nil
			}
		}
	}

	return monitor
}

// getPortType 根据显示器名称判断出端口类型，比如 vga，hdmi，edp 等。
func (m *Manager) getPortType(name string) string {
	i := strings.IndexRune(name, '-')
	if i != -1 {
		name = name[0:i]
	}
	return strings.ToLower(name)
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

// updatePropMonitors 把所有已连接显示器的对象路径设置到 Manager 的 Monitors 属性。
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

	if name == newName {
		return nil
	}

	err = m.saveConfig()
	if err != nil {
		return err
	}

	return nil
}

// getScreenSize 根据 Manager.monitorMap 中显示器的设置，计算出需要的屏幕尺寸。
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

func (m *Manager) newTouchscreen(path dbus.ObjectPath) (*Touchscreen, error) {
	t, err := inputdevices.NewTouchscreen(m.sysBus, path)
	if err != nil {
		return nil, err
	}

	touchscreen := &Touchscreen{
		path: path,
	}
	touchscreen.Name, _ = t.Name().Get(0)
	touchscreen.DeviceNode, _ = t.DevNode().Get(0)
	touchscreen.Serial, _ = t.Serial().Get(0)
	touchscreen.uuid, _ = t.UUID().Get(0)
	touchscreen.outputName, _ = t.OutputName().Get(0)
	touchscreen.width, _ = t.Width().Get(0)
	touchscreen.height, _ = t.Height().Get(0)

	touchscreen.busType = BusTypeUnknown
	busType, _ := t.BusType().Get(0)
	if strings.ToLower(busType) == "usb" {
		touchscreen.busType = BusTypeUSB
	}

	getXTouchscreenInfo(touchscreen)
	if touchscreen.Id == 0 {
		return nil, xerrors.New("no mathced touchscreen ID")
	}

	return touchscreen, nil
}

func (m *Manager) removeTouchscreenByIdx(i int) {
	// see https://github.com/golang/go/wiki/SliceTricks
	m.Touchscreens[i] = m.Touchscreens[len(m.Touchscreens)-1]
	m.Touchscreens[len(m.Touchscreens)-1] = nil
	m.Touchscreens = m.Touchscreens[:len(m.Touchscreens)-1]
}

func (m *Manager) removeTouchscreenByPath(path dbus.ObjectPath) {
	i := -1
	for index, v := range m.Touchscreens {
		if v.path == path {
			i = index
		}
	}

	if i == -1 {
		return
	}

	m.removeTouchscreenByIdx(i)
}

func (m *Manager) removeTouchscreenByDeviceNode(deviceNode string) {
	i := -1
	for idx, v := range m.Touchscreens {
		if v.DeviceNode == deviceNode {
			i = idx
			break
		}
	}

	if i == -1 {
		return
	}

	m.removeTouchscreenByIdx(i)
}

func (m *Manager) initTouchscreens() {
	m.ofdbus.ConnectNameOwnerChanged(func(name, oldOwner, newOwner string) {
		if name == m.inputDevices.ServiceName_() && newOwner == "" {
			m.setPropTouchscreens(nil)
		}
	})

	_, err := m.inputDevices.ConnectTouchscreenAdded(func(path dbus.ObjectPath) {
		getDeviceInfos(true)

		// 通过 path 删除重复设备
		m.removeTouchscreenByPath(path)

		touchscreen, err := m.newTouchscreen(path)
		if err != nil {
			logger.Warning(err)
			return
		}

		// 若设备已存在，删除并重新添加
		m.removeTouchscreenByDeviceNode(touchscreen.DeviceNode)

		m.Touchscreens = append(m.Touchscreens, touchscreen)
		m.emitPropChangedTouchscreens(m.Touchscreens)

		m.handleTouchscreenChanged()
		m.showTouchscreenDialogs()
	})
	if err != nil {
		logger.Warning(err)
	}

	_, err = m.inputDevices.ConnectTouchscreenRemoved(func(path dbus.ObjectPath) {
		m.removeTouchscreenByPath(path)
		m.emitPropChangedTouchscreens(m.Touchscreens)
		m.handleTouchscreenChanged()
		m.showTouchscreenDialogs()
	})
	if err != nil {
		logger.Warning(err)
	}

	touchscreens, err := m.inputDevices.Touchscreens().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}

	getDeviceInfos(true)
	for _, p := range touchscreens {
		touchscreen, err := m.newTouchscreen(p)
		if err != nil {
			logger.Warning(err)
			continue
		}

		m.Touchscreens = append(m.Touchscreens, touchscreen)
	}
	m.emitPropChangedTouchscreens(m.Touchscreens)

	m.initTouchMap()
	m.handleTouchscreenChanged()
	m.showTouchscreenDialogs()
}

func (m *Manager) initTouchMap() {
	m.touchscreenMap = make(map[string]touchscreenMapValue)
	m.TouchMap = make(map[string]string)

	value := m.settings.GetString(gsKeyMapOutput)
	if len(value) == 0 {
		return
	}

	err := jsonUnmarshal(value, &m.touchscreenMap)
	if err != nil {
		logger.Warningf("[initTouchMap] unmarshal (%s) failed: %v",
			value, err)
		return
	}

	for touchUUID, v := range m.touchscreenMap {
		for _, t := range m.Touchscreens {
			if t.uuid == touchUUID {
				m.TouchMap[t.Serial] = v.OutputName
				break
			}
		}
	}
}

func (m *Manager) doSetTouchMap(monitor0 *Monitor, touchUUID string) error {
	var touchId int32 = -1
	for _, touchscreen := range m.Touchscreens {
		if touchscreen.uuid != touchUUID {
			continue
		}

		touchId = touchscreen.Id
	}
	if touchId == -1 {
		return fmt.Errorf("invalid touchscreen: %s", touchUUID)
	}

	dxTouchscreen, err := dxinput.NewTouchscreen(touchId)
	if err != nil {
		return err
	}

	ignoreGestureFunc := func(id int32, ignore bool) {
		hasNode := dxutil.IsPropertyExist(id, "Device Node")
		if hasNode {
			data, item := dxutil.GetProperty(id, "Device Node")
			node := string(data[:item])

			gestureObj := dgesture.NewGesture(m.sysBus)
			gestureObj.SetInputIgnore(0, node, ignore)
		}
	}

	if monitor0.Enabled {
		matrix := m.genTransformationMatrix(monitor0.X, monitor0.Y, monitor0.Width, monitor0.Height, monitor0.Rotation|monitor0.Reflect)
		logger.Debugf("matrix: %v", matrix)

		err = dxTouchscreen.Enable(true)
		if err != nil {
			return err
		}
		ignoreGestureFunc(dxTouchscreen.Id, false)

		return dxTouchscreen.SetTransformationMatrix(matrix)
	} else {
		logger.Debugf("touchscreen %s disabled", touchUUID)
		ignoreGestureFunc(dxTouchscreen.Id, true)
		return dxTouchscreen.Enable(false)
	}
}

func (m *Manager) updateTouchscreenMap(outputName string, touchUUID string, auto bool) {
	var err error

	m.touchscreenMap[touchUUID] = touchscreenMapValue{
		OutputName: outputName,
		Auto:       auto,
	}
	m.settings.SetString(gsKeyMapOutput, jsonMarshal(m.touchscreenMap))

	var touchSerial string
	for _, v := range m.Touchscreens {
		if v.uuid == touchUUID {
			touchSerial = v.Serial
		}
	}

	m.TouchMap[touchSerial] = outputName

	err = m.emitPropChangedTouchMap(m.TouchMap)
	if err != nil {
		logger.Warning("failed to emit TouchMap PropChanged:", err)
	}
}

func (m *Manager) removeTouchscreenMap(touchUUID string) {
	delete(m.touchscreenMap, touchUUID)
	m.settings.SetString(gsKeyMapOutput, jsonMarshal(m.touchscreenMap))

	var touchSerial string
	for _, v := range m.Touchscreens {
		if v.uuid == touchUUID {
			touchSerial = v.Serial
		}
	}

	delete(m.TouchMap, touchSerial)

	err := m.emitPropChangedTouchMap(m.TouchMap)
	if err != nil {
		logger.Warning("failed to emit TouchMap PropChanged:", err)
	}
}

func (m *Manager) associateTouch(monitor *Monitor, touchUUID string, auto bool) error {
	m.PropsMu.Lock()
	defer m.PropsMu.Unlock()

	if v, ok := m.touchscreenMap[touchUUID]; ok && v.OutputName == monitor.Name {
		return nil
	}

	err := m.doSetTouchMap(monitor, touchUUID)
	if err != nil {
		logger.Warning("[AssociateTouch] set failed:", err)
		return err
	}

	m.updateTouchscreenMap(monitor.Name, touchUUID, auto)

	return nil
}

// saveConfig 保存配置到文件，把 Manager.config 内容写到文件。
func (m *Manager) saveConfig() error {
	dir := filepath.Dir(configFile_v5)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(configVersionFile, []byte(configVersion), 0644)
	if err != nil {
		return err
	}

	err = m.config.save(configFile_v5)
	if err != nil {
		return err
	}
	return nil
}

func (m *Manager) showTouchscreenDialog(touchscreenSerial string) error {
	cmd := exec.Command(cmdTouchscreenDialogBin, touchscreenSerial)

	err := cmd.Start()
	if err != nil {
		return err
	}

	go func() {
		err = cmd.Wait()
		if err != nil {
			logger.Debug(err)
		}
	}()
	return nil
}

func (m *Manager) handleTouchscreenChanged() {
	logger.Debugf("touchscreens changed %#v", m.Touchscreens)

	monitors := m.getConnectedMonitors()

	// 清除已拔下触摸屏的配置
	for uuid := range m.touchscreenMap {
		found := false
		for _, touch := range m.Touchscreens {
			if touch.uuid == uuid {
				found = true
				break
			}
		}
		if !found {
			m.removeTouchscreenMap(uuid)
		}
	}

	if len(m.Touchscreens) == 1 && len(monitors) == 1 {
		m.associateTouch(monitors[0], m.Touchscreens[0].uuid, true)
	}

	for _, touch := range m.Touchscreens {
		// 有配置，直接使配置生效
		if v, ok := m.touchscreenMap[touch.uuid]; ok {
			monitor := monitors.GetByName(v.OutputName)
			if monitor != nil {
				logger.Debugf("assigned %s to %s, cfg", touch.uuid, v.OutputName)
				err := m.doSetTouchMap(monitor, touch.uuid)
				if err != nil {
					logger.Warning("failed to map touchscreen:", err)
				}
				continue
			}

			// else 配置中的显示器不存在，忽略配置并删除
			m.removeTouchscreenMap(touch.uuid)
		}

		if touch.outputName != "" {
			logger.Debugf("assigned %s to %s, WL_OUTPUT", touch.uuid, touch.outputName)
			monitor := monitors.GetByName(touch.outputName)
			if monitor == nil {
				logger.Warning("WL_OUTPUT not found")
				continue
			}
			err := m.associateTouch(monitor, touch.uuid, true)
			if err != nil {
				logger.Warning(err)
			}
			continue
		}

		// 物理大小匹配
		assigned := false
		for _, monitor := range monitors {
			logger.Debugf("monitor %s w %d h %d, touch %s w %d h %d",
				monitor.Name, monitor.MmWidth, monitor.MmHeight,
				touch.uuid, uint32(math.Round(touch.width)), uint32(math.Round(touch.height)))

			if monitor.MmWidth == uint32(math.Round(touch.width)) && monitor.MmHeight == uint32(math.Round(touch.height)) {
				logger.Debugf("assigned %s to %s, phy size", touch.uuid, monitor.Name)
				err := m.associateTouch(monitor, touch.uuid, true)
				if err != nil {
					logger.Warning(err)
				}
				assigned = true
				break
			}
		}
		if assigned {
			continue
		}

		// 有内置显示器，且触摸屏不是通过 USB 连接，关联内置显示器
		if m.builtinMonitor != nil {
			if touch.busType != BusTypeUSB {
				logger.Debugf("assigned %s to %s, builtin", touch.uuid, m.builtinMonitor.Name)
				err := m.associateTouch(m.builtinMonitor, touch.uuid, true)
				if err != nil {
					logger.Warning(err)
				}
				continue
			}
		}

		// 关联主显示器，不保存主显示器不保存配置，并显示配置 Dialog
		monitor := monitors.GetByName(m.Primary)
		if monitor == nil {
			logger.Warningf("primary output %s not found", m.Primary)
		} else {
			err := m.doSetTouchMap(monitor, touch.uuid)
			if err != nil {
				logger.Warning("failed to map touchscreen:", err)
			}
		}
	}
}

// 检查当前连接的所有触控面板, 如果没有映射配置, 那么调用 OSD 弹窗.
func (m *Manager) showTouchscreenDialogs() {
	for _, touch := range m.Touchscreens {
		if _, ok := m.touchscreenMap[touch.uuid]; !ok {
			logger.Debug("cannot find touchscreen", touch.uuid, "'s configure, show OSD")
			err := m.showTouchscreenDialog(touch.Serial)
			if err != nil {
				logger.Warning("shotTouchscreenOSD", err)
			}
		}
	}
}

// setMethodAdjustCCT 调用 redshift 程序设置色温的模式。
// Normal 模式不调节色温，Auto 模式是自动，Manual 模式是手动，根据数值调整。
func (m *Manager) setMethodAdjustCCT(adjustMethod int32) error {
	if adjustMethod > ColorTemperatureModeManual || adjustMethod < ColorTemperatureModeNormal {
		return errors.New("adjustMethod type out of range, not 0 or 1 or 2")
	}
	_ = m.emitPropChangedColorTemperatureMode(adjustMethod)
	m.saveColorTemperatureModeToConfigs(adjustMethod)
	switch adjustMethod {
	case ColorTemperatureModeNormal: // 不调节色温，关闭redshift服务
		controlRedshift("stop") // 停止服务
		resetColorTemp()        // 色温重置
	case ColorTemperatureModeAuto: // 自动模式调节色温 启动服务
		resetColorTemp()
		controlRedshift("start") // 开启服务
	case ColorTemperatureModeManual: // 手动调节色温 关闭服务 调节色温(调用存在之前保存的手动色温值)
		controlRedshift("stop") // 停止服务
		lastManualCCT := m.ColorTemperatureManual
		err := m.setColorTemperature(lastManualCCT)
		if err != nil {
			return err
		}
	}
	return nil
}

// setColorTemperature 指定色温值为参数 value。
func (m *Manager) setColorTemperature(value int32) error {
	if m.ColorTemperatureMode != ColorTemperatureModeManual {
		return errors.New("current not manual mode, can not adjust CCT by manual")
	}
	if value < 1000 || value > 25000 {
		return errors.New("value out of range")
	}
	setColorTempOneShot(strconv.Itoa(int(value))) // 手动设置色温
	_ = m.emitPropChangedColorTemperatureManual(value)
	m.saveColorTemperatureToConfigs(value)
	return nil
}

// syncBrightness 将亮度从每个显示器 monitor.Brightness 同步到 Manager 的属性 Brightness 中。
func (m *Manager) syncBrightness() {
	for _, monitor := range m.monitorMap {
		if monitor.Connected {
			logger.Debug("monitor.Brightness = ", monitor.Brightness)
			m.Brightness[monitor.Name] = monitor.Brightness
		}
	}
	m.setPropBrightness(m.Brightness)
}

func (m *Manager) initConnectInfo() {
	tmp, _ := doReadCache(cacheFile)
	if tmp != nil {
		m.info = *tmp
	} else {
		m.info.Connects = make(map[string]bool)
		m.info.LastConnectedTimes = make(map[string]time.Time)
	}
}

func (m *Manager) setAndSaveConnectInfo(connected bool, outputInfo *randr.GetOutputInfoReply) {
	if connected && !m.info.Connects[outputInfo.Name] {
		m.info.Connects[outputInfo.Name] = connected
		m.info.LastConnectedTimes[outputInfo.Name] = time.Now()
		err := doSaveCache(&m.info, cacheFile)
		if err != nil {
			logger.Warning("doSaveCache failed", err)
		}
	}
}

func (m *Manager) getRateFilter() RateFilterMap {
	var data RateFilterMap = make(RateFilterMap)
	jsonStr := m.settings.GetString(gsKeyRateFilter)
	err := json.Unmarshal([]byte(jsonStr), &data)
	if err != nil {
		logger.Warning(err)
		return data
	}

	return data
}

func (m *Manager) listenRotateSignal() {
	const (
		strFace       = "com.deepin.SensorProxy"
		strPath       = "/com/deepin/SensorProxy"
		strSignalName = "RotationValueChanged"
	)

	systemBus, err := dbus.SystemBus()
	if err != nil {
		logger.Fatal(err)
	}

	err = systemBus.BusObject().AddMatchSignal(strFace, strSignalName,
		dbus.WithMatchObjectPath(dbus.ObjectPath(strPath)), dbus.WithMatchSender(strFace)).Err
	if err != nil {
		logger.Fatal(err)
	}

	m.rotationFinishChanged = true
	signalCh := make(chan *dbus.Signal, 10)
	systemBus.Signal(signalCh)
	go func() {
		for {
			select {
			case sig := <-signalCh:
				var rotateScreenValue string
				err = dbus.Store(sig.Body, &rotateScreenValue)
				if err == nil {
					if m.builtinMonitor.latestRotationValue != rotationScreenValue[rotateScreenValue] {
						m.builtinMonitor.latestRotationValue = rotationScreenValue[rotateScreenValue]
						if m.rotationFinishChanged {
							m.rotationFinishChanged = false
							m.rotationScreenTimer = time.AfterFunc(time.Millisecond*time.Duration(
								m.gsRotateScreenTimeDelay.Get()), func() {
								m.startRotateScreen()
								m.rotationFinishChanged = true
							})
						} else {
							m.rotationScreenTimer.Reset(time.Millisecond * time.Duration(
								m.gsRotateScreenTimeDelay.Get()))
						}
					}
				}
			}
		}
	}()
}

func (m *Manager) startRotateScreen() {
	// 判断旋转信号值是否符合要求
	if m.builtinMonitor.latestRotationValue != randr.RotationRotate0 &&
		m.builtinMonitor.latestRotationValue != randr.RotationRotate90 &&
		m.builtinMonitor.latestRotationValue != randr.RotationRotate270 {
		logger.Warningf("get Rotation screen value failed: %d", m.builtinMonitor.latestRotationValue)
		return
	}

	if m.builtinMonitor != nil {
		err := m.builtinMonitor.SetRotation(m.builtinMonitor.latestRotationValue)
		if err != nil {
			logger.Warning("call SetRotation failed:", err)
			return
		}

		// 使旋转后配置生效
		err = m.ApplyChanges()
		if err != nil {
			logger.Warning("call ApplyChanges failed:", err)
			return
		}

		err = m.Save()
		if err != nil {
			logger.Warning("call Save failed:", err)
			return
		}

		err1 := m.builtinMonitor.service.Emit(m.builtinMonitor, "RotateFinish")
		if err1 != nil {
			logger.Warning("emit RotateFinish failed:", err1)
			return
		}
	}
}
