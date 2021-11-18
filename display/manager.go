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
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/godbus/dbus"
	dgesture "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.gesture"
	sysdisplay "github.com/linuxdeepin/go-dbus-factory/com.deepin.system.display"
	inputdevices "github.com/linuxdeepin/go-dbus-factory/com.deepin.system.inputdevices"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.dbus"
	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
	"golang.org/x/xerrors"
	"pkg.deepin.io/dde/api/dxinput"
	dxutil "pkg.deepin.io/dde/api/dxinput/utils"
	"pkg.deepin.io/dde/startdde/display/brightness"
	gio "pkg.deepin.io/gir/gio-2.0"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/gsettings"
	"pkg.deepin.io/lib/log"
	"pkg.deepin.io/lib/xdg/basedir"
)

const (
	DisplayModeCustom uint8 = iota
	DisplayModeMirror
	DisplayModeExtend
	DisplayModeOnlyOne
	DisplayModeUnknown
)

// DisplayModeInvalid 无效的模式
const DisplayModeInvalid uint8 = 0

const (
	// 1：自动旋转；即未主动调用 SetRotation() 接口（由内部触发）发生的旋转操作，如根据重力传感器自动设定旋转方向
	RotationFinishModeAuto uint8 = iota + 1
	// 2：手动旋转；即主动调用 SetRotation() 接口完成旋转，如控制中心下拉框方式旋转屏幕
	RotationFinishModeManual
)

const (
	sensorProxyInterface       = "com.deepin.SensorProxy"
	sensorProxyPath            = "/com/deepin/SensorProxy"
	sensorProxySignalName      = "RotationValueChanged"
	sensorProxySignal          = "com.deepin.SensorProxy.RotationValueChanged"
	sensorProxyGetScreenStatus = "com.deepin.SensorProxy.GetScreenStatus"
)

const (
	gsSchemaDisplay  = "com.deepin.dde.display"
	gsKeyDisplayMode = "display-mode"
	gsKeyBrightness  = "brightness"
	gsKeySetter      = "brightness-setter"
	gsKeyMapOutput   = "map-output"
	gsKeyRateFilter  = "rate-filter"
	//gsKeyPrimary     = "primary"
	gsKeyCustomMode              = "current-custom-mode"
	gsKeyColorTemperatureMode    = "color-temperature-mode"
	gsKeyColorTemperatureManual  = "color-temperature-manual"
	gsKeyRotateScreenTimeDelay   = "rotate-screen-time-delay"
	customModeDelim              = "+"
	monitorsIdDelimiter          = ","
	defaultTemperatureMode       = ColorTemperatureModeNone
	defaultTemperatureManual     = 6500
	defaultRotateScreenTimeDelay = 500

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
	_monitorTypePriority = map[string]int{
		"edp":  priorityEDP,
		"dp":   priorityDP,
		"hdmi": priorityHDMI,
		"dvi":  priorityDVI,
		"vga":  priorityVGA,
	}
)

var (
	startBuildInScreenRotationMutex sync.Mutex
	rotationScreenValue             = map[string]uint16{
		"normal": randr.RotationRotate0,
		"left":   randr.RotationRotate270, // 屏幕重力旋转左转90
		"right":  randr.RotationRotate90,  // 屏幕重力旋转右转90
	}
)

type touchscreenMapValue struct {
	OutputName string
	Auto       bool
}

//go:generate dbusutil-gen -output display_dbusutil.go -import github.com/godbus/dbus,github.com/linuxdeepin/go-x11-client,pkg.deepin.io/lib/strv -type Manager,Monitor manager.go monitor.go
//go:generate dbusutil-gen em -type Manager,Monitor

type Manager struct {
	service        *dbusutil.Service
	sysBus         *dbus.Conn
	sysSigLoop     *dbusutil.SignalLoop
	sessionSigLoop *dbusutil.SignalLoop
	// 系统级 dbus-daemon 服务
	dbusDaemon   ofdbus.DBus
	sensorProxy  dbus.BusObject
	inputDevices inputdevices.InputDevices
	// 系统级 display 服务
	sysDisplay sysdisplay.Display
	xConn      *x.Conn
	PropsMu    sync.RWMutex
	sysConfig  SysRootConfig
	userConfig UserConfig
	userCfgMu  sync.Mutex

	recommendScaleFactor     float64
	builtinMonitor           *Monitor
	builtinMonitorMu         sync.Mutex
	candidateBuiltinMonitors []*Monitor // 候补的

	monitorMap   map[uint32]*Monitor
	monitorMapMu sync.Mutex
	mm           monitorManager
	debugOpts    debugOptions

	sessionActive bool
	newSysCfg     *SysRootConfig
	cursorShowed  bool

	// gsettings com.deepin.dde.display
	settings              *gio.Settings
	monitorsId            string
	monitorsIdMu          sync.Mutex
	hasBuiltinMonitor     bool
	rotateScreenTimeDelay int32
	setFillModeMu         sync.Mutex
	delayApplyTimer       *time.Timer
	prevNumMonitors       int
	applyMu               sync.Mutex
	inApply               bool

	// dbusutil-gen: equal=objPathsEqual
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
	// touch.uuid -> touchScreenDialog cmd
	touchScreenDialogMap   map[string]*exec.Cmd
	touchScreenDialogMutex sync.RWMutex

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
	// TODO 删除下面 2 个色温相关字段
	// 存在gsetting中的色温模式
	gsColorTemperatureMode int32
	// 存在gsetting中的色温值
	gsColorTemperatureManual int32
}

type monitorSizeInfo struct {
	width, height     uint16
	mmWidth, mmHeight uint32
}

var _ monitorManagerHooks = (*Manager)(nil)

func newManager(service *dbusutil.Service) *Manager {
	m := &Manager{
		service:    service,
		monitorMap: make(map[uint32]*Monitor),
		Brightness: make(map[string]float64),
	}

	chassis, err := getComputeChassis()
	if err != nil {
		logger.Warning(err)
	}
	if chassis == "laptop" || chassis == "all-in-one" {
		m.hasBuiltinMonitor = true
	}

	m.settings = gio.NewSettings(gsSchemaDisplay)
	m.CurrentCustomId = m.settings.GetString(gsKeyCustomMode)
	m.rotateScreenTimeDelay = m.settings.GetInt(gsKeyRotateScreenTimeDelay)
	m.ColorTemperatureManual = defaultTemperatureManual
	m.ColorTemperatureMode = defaultTemperatureMode

	m.xConn = _xConn

	screen := m.xConn.GetDefaultScreen()
	m.ScreenWidth = screen.WidthInPixels
	m.ScreenHeight = screen.HeightInPixels

	sessionSigLoop := dbusutil.NewSignalLoop(m.service.Conn(), 10)
	m.sessionSigLoop = sessionSigLoop
	sessionSigLoop.Start()

	if _useWayland {
		m.mm = newKMonitorManager(sessionSigLoop)
	} else {
		m.mm = newXMonitorManager(m.xConn, _hasRandr1d2)
	}

	m.mm.setHooks(m)

	m.setPropMaxBacklightBrightness(uint32(brightness.GetMaxBacklightBrightness()))

	m.sysBus, err = dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
	}

	sysSigLoop := dbusutil.NewSignalLoop(m.sysBus, 10)
	m.sysSigLoop = sysSigLoop
	sysSigLoop.Start()

	m.dbusDaemon = ofdbus.NewDBus(m.sysBus)
	m.dbusDaemon.InitSignalExt(sysSigLoop, true)

	m.inputDevices = inputdevices.NewInputDevices(m.sysBus)
	m.inputDevices.InitSignalExt(sysSigLoop, true)

	m.sysDisplay = sysdisplay.NewDisplay(m.sysBus)

	loginManager := login1.NewManager(m.sysBus)
	loginManager.InitSignalExt(sysSigLoop, true)
	/* 当系统从待机或者休眠状态唤醒时，需要重新获取当前屏幕的状态 */
	_, err = loginManager.ConnectPrepareForSleep(func(isSleep bool) {
		if !isSleep {
			logger.Info("system Wakeup, need reacquire screen status", isSleep)
			m.initScreenRotation()
		}
	})

	if err != nil {
		logger.Warning("failed to connect signal PrepareForSleep:", err)
	}

	selfSessionPath, err := loginManager.GetSessionByPID(0, uint32(os.Getpid()))
	if err != nil {
		logger.Warningf("get session path failed: %v", err)
		// 允许在不能获取 session path 时提早结束
		return m
	}
	logger.Debug("self session path:", selfSessionPath)
	selfSession, err := login1.NewSession(m.sysBus, selfSessionPath)
	if err != nil {
		logger.Warning(err)
		return m
	}

	selfSession.InitSignalExt(sysSigLoop, true)
	err = selfSession.Active().ConnectChanged(func(hasValue, active bool) {
		if !hasValue {
			return
		}
		logger.Debug("session active changed", active)

		m.sessionActive = active
		if !active {
			return
		}
		if m.newSysCfg != nil {
			m.handleSysConfigUpdated(m.newSysCfg)
			m.newSysCfg = nil
		}

		m.handleTouchscreenChanged()
		m.showTouchscreenDialogs()

		// 监听用户的session Active属性改变信号，当切换到当前已经登录的用户时
		// 需要从内核重新获取当前屏幕的状态，将锁屏界面旋转到对应方向
		if m.builtinMonitor != nil {
			m.initScreenRotation()
		}
	})
	if err != nil {
		logger.Warningf("prop active ConnectChanged failed! %v", err)
	}

	m.sessionActive, err = selfSession.Active().Get(0)
	if err != nil {
		logger.Warning(err)
	}

	return m
}

// 初始化系统级 display 服务的信号处理
func (m *Manager) initSysDisplay() {
	m.sysDisplay.InitSignalExt(m.sysSigLoop, true)
	_, err := m.sysDisplay.ConnectConfigUpdated(func(updateAt string) {
		logger.Debug("sys display ConfigUpdated", updateAt)
		if updateAt == m.sysConfig.UpdateAt {
			return
		}

		newSysConfig, err := m.getSysConfig()
		if err != nil {
			// 获取出错，忽略这一次更新信号
			logger.Warning("getSysConfig err:", err)
			return
		}
		if logger.GetLogLevel() == log.LevelDebug {
			logger.Debug("get new sysConfig:", spew.Sdump(newSysConfig))
		}

		if !m.sessionActive {
			m.newSysCfg = newSysConfig
			return
		}

		m.handleSysConfigUpdated(newSysConfig)
	})
	if err != nil {
		logger.Warning(err)
	}
}

// 处理系统级别的配置更新
func (m *Manager) handleSysConfigUpdated(newSysConfig *SysRootConfig) {
	logger.Debug("handleSysConfigUpdated")
	setCfg := func() {
		m.sysConfig.copyFrom(newSysConfig)
	}

	currentCfg := &m.sysConfig.Config
	newCfg := &newSysConfig.Config
	cfgEq := reflect.DeepEqual(currentCfg, newCfg)
	if cfgEq {
		// 具体配置没有任何改变
		logger.Debug("cfg eq")
		setCfg()
		return
	}

	fillModesEq := reflect.DeepEqual(currentCfg.FillModes, newCfg.FillModes)
	displayModeEq := currentCfg.DisplayMode == newCfg.DisplayMode
	scaleFactorsEq := reflect.DeepEqual(currentCfg.ScaleFactors, newCfg.ScaleFactors)
	monitorMap := m.cloneMonitorMap()
	monitors := getConnectedMonitors(monitorMap)
	single := len(monitors) == 1
	monitorsId := monitors.getMonitorsId()
	currentMonitorCfgs := currentCfg.getMonitorConfigs(monitorsId, currentCfg.DisplayMode, single)
	currentMonitorCfgs.sort()
	newMonitorCfgs := newCfg.getMonitorConfigs(monitorsId, currentCfg.DisplayMode, single)
	newMonitorCfgs.sort()
	monitorCfgsEq := reflect.DeepEqual(currentMonitorCfgs, newMonitorCfgs)
	logger.Debugf("fillModeEq: %v, displayModeEq: %v, scaleFactorsEq: %v, monitorCfgsEq: %v, monitorsId: %v, single: %v",
		fillModesEq, displayModeEq, scaleFactorsEq, monitorCfgsEq, monitorsId, single)
	if logger.GetLogLevel() == log.LevelDebug {
		logger.Debugf("currentMonitorCfgs: %s", spew.Sdump(currentMonitorCfgs))
		logger.Debugf("newMonitorCfgs: %s", spew.Sdump(newMonitorCfgs))
	}

	setCfg()

	if !scaleFactorsEq {
		// scale factors 改变了
		logger.Debug("scaleFactors changed")
		if ScaleFactorsHelper.changedCb != nil {
			go func() {
				err := ScaleFactorsHelper.changedCb(newCfg.ScaleFactors)
				if err != nil {
					logger.Warning("scale factors changed cb err:", err)
				}
			}()
		} else {
			logger.Warning("scale factors changed cb is nil")
		}
	}

	if !displayModeEq {
		// displayMode 改变了
		logger.Debug("displayMode changed")
		go func() {
			err := m.applyDisplayConfig(newCfg.DisplayMode, monitorsId, monitorMap, false, nil)
			if err != nil {
				logger.Warning(err)
				return
			}
			m.setPropDisplayMode(newCfg.DisplayMode)
		}()
		return
	}

	// 以下都是 displayMode 没变

	doApply := false
	if !monitorCfgsEq {
		if currentMonitorCfgs.onlyBrNotEq(newMonitorCfgs) {
			// 仅亮度改变
			logger.Debug("monitorCfgs not eq, but only brightness changed")
			go func() {
				for _, config := range newMonitorCfgs {
					if config.Enabled {
						err := m.setBrightness(config.Name, config.Brightness)
						if err != nil {
							logger.Warning(err)
						}
					}
				}
				m.syncPropBrightness()
			}()
		} else {
			// 不光是亮度改变，还有其他屏幕配置，比如位置pos，分辨率等改变。
			logger.Debug("monitor configs changed")
			doApply = true
			go func() {
				err := m.applySysMonitorConfigs(newCfg.DisplayMode, monitorsId, monitorMap, newMonitorCfgs, nil)
				if err != nil {
					logger.Warning(err)
					return
				}
			}()
		}
	}

	if !fillModesEq {
		// fillModes 改变了
		if !doApply {
			// applySysMonitorConfigs 会在内部设置 fillMode
			logger.Debug("fillModes changed, set fill mode for monitors")
			monitors := m.getConnectedMonitors()
			go func() {
				// 设置 fillModes
				for _, monitor := range monitors {
					err := m.mm.setMonitorFillMode(monitor, newCfg.FillModes[monitor.generateFillModeKey()])
					if err != nil {
						logger.Warning("set monitor fill mode failed:", monitor, err)
					}
				}
			}()
		}
	}
}

// initBuiltinMonitor 初始化内置显示器。
func (m *Manager) initBuiltinMonitor() {
	if !m.hasBuiltinMonitor {
		return
	}
	// 从系统级配置中获取内置显示器名称
	builtinMonitorName := m.sysConfig.Config.Cache.BuiltinMonitor

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
		m.builtinMonitor = getMinIdMonitor(rest)
		// 把剩余显示器列表 rest 设置到候选内置显示器列表。
		m.candidateBuiltinMonitors = rest
	}

	// 保存内置显示器配置文件
	err := m.saveBuiltinMonitorConfig(builtinMonitorName)
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
		err := m.saveBuiltinMonitorConfig(m.builtinMonitor.Name)
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

func (m *Manager) buildConfigForSingle(monitor *Monitor) SysMonitorConfigs {
	cfg := monitor.toBasicSysConfig()
	cfg.Enabled = true
	cfg.Primary = true
	mode := monitor.BestMode
	cfg.Width = mode.Width
	cfg.Height = mode.Height
	cfg.RefreshRate = mode.Rate
	// cfg.X = 0
	// cfg.Y = 0
	cfg.Brightness = 1
	cfg.Rotation = randr.RotationRotate0
	//cfg.Reflect = 0
	return SysMonitorConfigs{cfg}
}

func (m *Manager) applyDisplayConfig(mode byte, monitorsId string, monitorMap map[uint32]*Monitor, setColorTemp bool, options applyOptions) error {
	if monitorsId == "" {
		return errors.New("monitorsId is empty")
	}
	// X 环境下，如果 randr 版本低于 1.2 时，不做操作
	if !_useWayland && !_hasRandr1d2 {
		return nil
	}
	monitors := getConnectedMonitors(monitorMap)
	if len(monitors) == 0 {
		// 拔掉所有显示器
		logger.Debug("applyDisplayConfig not found any connected monitor，return")
		return nil
	}
	defer func() {
		if _useWayland {
			m.updateScreenSize()
		}

		// 拔插屏幕时需要根据配置文件重置色温
		if setColorTemp {
			m.applyColorTempConfig(mode)
		}
	}()
	var err error
	if len(monitors) == 1 {
		// 单屏情况
		screenCfg := m.getSysScreenConfig(monitorsId)
		needSaveCfg := false
		monitorConfigs := screenCfg.getSingleMonitorConfigs()
		if len(monitorConfigs) == 0 {
			// 没有单屏配置
			needSaveCfg = true
			monitorConfigs = m.buildConfigForSingle(monitors[0])
		}

		// 应用配置
		err = m.applySysMonitorConfigs(DisplayModeInvalid, monitorsId, monitorMap, monitorConfigs, options)
		if err != nil {
			logger.Warning("failed to apply configs:", err)
			return err
		}
		if needSaveCfg {
			screenCfg.setSingleMonitorConfigs(monitorConfigs)
			err = m.saveSysConfig("single")
			if err != nil {
				logger.Warning(err)
			}
		}
		return nil
	}
	// 多屏情况
	switch mode {
	case DisplayModeMirror:
		err = m.applyModeMirror(monitorsId, monitorMap, options)
	case DisplayModeExtend:
		err = m.applyModeExtend(monitorsId, monitorMap, options)
	case DisplayModeOnlyOne:
		err = m.applyModeOnlyOne(monitorsId, monitorMap, options)
	}

	if err != nil {
		logger.Warning(err)
		return err
	}
	return nil
}

func (m *Manager) migrateOldConfig() {
	if _greeterMode {
		return
	}
	logger.Debug("migrateOldConfig")

	// NOTE: 在设置 m.DisplayMode, m.Brightness, m.gsColorTemperatureMode, m.gsColorTemperatureManual 之后
	// 再加载配置文件并迁移，主要原因是 loadOldConfig 中的 ConfigV3D3.toConfig 和 ConfigV4.toConfig 需要。
	m.DisplayMode = byte(m.settings.GetEnum(gsKeyDisplayMode))
	m.gsColorTemperatureMode = m.settings.GetInt(gsKeyColorTemperatureMode)
	m.gsColorTemperatureManual = m.settings.GetInt(gsKeyColorTemperatureManual)
	m.initBrightness()
	configV6, err := loadOldConfig(m)
	if err != nil {
		// 旧配置加载失败
		if !os.IsNotExist(err) {
			logger.Warning(err)
		}
	} else {
		// 旧配置加载成功
		if logger.GetLogLevel() == log.LevelDebug {
			logger.Debug("migrateOldConfig configV6:", spew.Sdump(configV6))
		}
		sysCfg := configV6.toSysConfigV1()
		m.sysConfig.Config = sysCfg

		m.userConfig = configV6.toUserConfigV1()
		m.userConfig.fix()
		if err := m.saveUserConfig(); err != nil {
			logger.Warning(err)
		}
	}

	cfgDir := getCfgDir()
	// 内置显示器配置文件 ~/.config/deepin/startdde/builtin-monitor
	builtinMonitorConfigFile := filepath.Join(cfgDir, "builtin-monitor")
	builtinMonitor, err := loadBuiltinMonitorConfig(builtinMonitorConfigFile)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warning(err)
		}
	} else {
		m.sysConfig.Config.Cache.BuiltinMonitor = builtinMonitor
	}

	// NOTE: 这里的 cache 文件路径就是错的 ~/.cache/deepin/startdded/connectifno.cache
	connectCacheFile := filepath.Join(basedir.GetUserCacheDir(),
		"deepin/startdded/connectifno.cache")
	connectInfo, err := readConnectInfoCache(connectCacheFile)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warning(err)
		}
	} else {
		connectTime := make(map[string]time.Time)
		for name, connected := range connectInfo.Connects {
			if connected {
				if t, ok := connectInfo.LastConnectedTimes[name]; ok {
					connectTime[name] = t
				}
			}
		}
		m.sysConfig.Config.Cache.ConnectTime = connectTime
	}

	m.sysConfig.fix()
	if err := m.saveSysConfigNoLock("migrate old config"); err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) init() {
	brightness.InitBacklightHelper()
	m.initDebugOptions()
	m.loadSysConfig()
	if m.sysConfig.Version == "" {
		// 系统配置为空，需要迁移旧配置
		m.migrateOldConfig()
	}

	if _scaleFactors != nil {
		err := m.setScaleFactors(_scaleFactors)
		if err != nil {
			logger.Warning(err)
		}
		_scaleFactors = nil
	}

	if _hasRandr1d2 || _useWayland {
		monitors := m.mm.getMonitors()
		logger.Debug("len monitors", len(monitors))
		err := m.recordMonitorsConnected(monitors)
		if err != nil {
			logger.Warning(err)
		}

		for _, monitor := range monitors {
			err := m.addMonitor(monitor)
			if err != nil {
				logger.Warning(err)
			}
		}

		m.initBuiltinMonitor()
		m.monitorsId = m.getMonitorsId()
		m.updatePropMonitors()

	} else {
		// randr 版本低于 1.2
		screen := m.xConn.GetDefaultScreen()
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

	m.DisplayMode = m.sysConfig.Config.DisplayMode

	err := m.loadUserConfig()
	if err != nil {
		logger.Warning("loadUserConfig err:", err)
	}

	// NOTE: m.listenXEvents 应该在 m.applyDisplayConfig 之前，否则会造成它里面的 m.apply 函数的等待超时。
	m.listenXEvents()
	// 此时不需要设置色温，在 StartPart2 中做。为性能考虑。
	monitorMap := m.cloneMonitorMap()
	monitorsId := getConnectedMonitors(monitorMap).getMonitorsId()
	err = m.applyDisplayConfig(m.DisplayMode, monitorsId, monitorMap, false, nil)
	if err != nil {
		logger.Warning(err)
	}
	if m.builtinMonitor != nil {
		m.listenSettingsChanged() // 监听旋转屏幕延时值
		m.initScreenRotation()    // 获取初始屏幕的状态（屏幕方向）
		m.listenRotateSignal()    // 监听屏幕旋转信号
	} else {
		// 没有内建屏,不监听内核信号
		logger.Info("built-in screen does not exist")
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

// 过滤掉部分模式，尽量不过滤掉 saveMode。
func (m *Manager) filterModeInfos(modeInfos []ModeInfo, saveMode ModeInfo) []ModeInfo {
	result := filterModeInfosByRefreshRate(filterModeInfos(modeInfos, saveMode), m.getRateFilter())
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
		service:       m.service,
		m:             m,
		ID:            uint32(output),
		Name:          "Default",
		Connected:     true,
		realConnected: true,
		MmWidth:       uint32(size.MWidth),
		MmHeight:      uint32(size.MHeight),
		Enabled:       true,
		Width:         size.Width,
		Height:        size.Height,
	}

	err = m.service.Export(monitor.getPath(), monitor)
	if err != nil {
		return nil, err
	}
	m.monitorMapMu.Lock()
	m.monitorMap[uint32(output)] = monitor
	m.monitorMapMu.Unlock()
	return monitor, nil
}

func (m *Manager) updateMonitorFallback(screenInfo *randr.GetScreenInfoReply) *Monitor {
	output := randr.Output(1)
	m.monitorMapMu.Lock()
	monitor, ok := m.monitorMap[uint32(output)]
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

func (m *Manager) recordMonitorsConnected(monitors []*MonitorInfo) (err error) {
	t := time.Now()
	needSave := false
	for _, monitor := range monitors {
		ns := m.recordMonitorConnectedAux(monitor.Name, monitor.Connected, t)
		needSave = needSave || ns
	}
	if needSave {
		err = m.saveSysConfig("monitors connected changed")
	}
	return err
}

func (m *Manager) recordMonitorConnected(name string, connected bool, t time.Time) (err error) {
	logger.Debug("recordMonitorConnected", name, connected, t)
	needSave := m.recordMonitorConnectedAux(name, connected, t)
	if needSave {
		err = m.saveSysConfig("monitor connected changed")
	}
	return err
}

func (m *Manager) recordMonitorConnectedAux(name string, connected bool, t time.Time) (needSave bool) {
	m.sysConfig.mu.Lock()
	connectTime := m.sysConfig.Config.Cache.ConnectTime
	if connected {
		// 连接
		if _, ok := connectTime[name]; !ok {
			if connectTime == nil {
				connectTime = make(map[string]time.Time)
				m.sysConfig.Config.Cache.ConnectTime = connectTime
			}
			connectTime[name] = t
			needSave = true
		}
	} else {
		// 断开
		if _, ok := connectTime[name]; ok {
			delete(connectTime, name)
			needSave = true
		}
	}
	m.sysConfig.mu.Unlock()
	return needSave
}

// addMonitor 在 Manager.monitorMap 增加显示器，在 dbus 上导出显示器对象
func (m *Manager) addMonitor(monitorInfo *MonitorInfo) error {
	m.monitorMapMu.Lock()
	_, ok := m.monitorMap[monitorInfo.ID]
	m.monitorMapMu.Unlock()
	if ok {
		return nil
	}

	logger.Debug("addMonitor", monitorInfo.Name)

	monitor := &Monitor{
		service:            m.service,
		m:                  m,
		ID:                 monitorInfo.ID,
		Name:               monitorInfo.Name,
		Connected:          monitorInfo.VirtualConnected,
		realConnected:      monitorInfo.Connected,
		MmWidth:            monitorInfo.MmWidth,
		MmHeight:           monitorInfo.MmHeight,
		Enabled:            monitorInfo.Enabled,
		uuid:               monitorInfo.UUID,
		Manufacturer:       monitorInfo.Manufacturer,
		Model:              monitorInfo.Model,
		AvailableFillModes: monitorInfo.AvailableFillModes,
	}

	monitor.Modes = m.filterModeInfos(monitorInfo.Modes, monitorInfo.PreferredMode)
	monitor.BestMode = getBestMode(monitor.Modes, monitorInfo.PreferredMode)
	if !monitor.BestMode.isZero() {
		monitor.PreferredModes = []ModeInfo{monitor.BestMode}
	}
	monitor.X = monitorInfo.X
	monitor.Y = monitorInfo.Y
	monitor.Width = monitorInfo.Width
	monitor.Height = monitorInfo.Height

	monitor.Reflects = getReflects(monitorInfo.Rotations)
	monitor.Rotations = getRotations(monitorInfo.Rotations)
	monitor.Rotation, monitor.Reflect = parseCrtcRotation(monitorInfo.Rotation)
	monitor.CurrentMode = monitorInfo.CurrentMode
	monitor.RefreshRate = monitorInfo.CurrentMode.Rate

	monitor.oldRotation = monitor.Rotation

	m.handleMonitorConnectedChanged(monitor, monitorInfo.Connected)

	err := m.service.Export(monitor.getPath(), monitor)
	if err != nil {
		return err
	}
	m.monitorMapMu.Lock()
	m.monitorMap[monitorInfo.ID] = monitor
	m.monitorMapMu.Unlock()

	monitorObj := m.service.GetServerObject(monitor)
	err = monitorObj.SetWriteCallback(monitor, "CurrentFillMode",
		monitor.setCurrentFillMode)
	if err != nil {
		logger.Warning("call SetWriteCallback err:", err)
		return err
	}

	return nil
}

// updateMonitor 根据 outputInfo 中的信息更新 dbus 上的 Monitor 对象的属性
func (m *Manager) updateMonitor(monitorInfo *MonitorInfo) {
	m.monitorMapMu.Lock()
	monitor, ok := m.monitorMap[monitorInfo.ID]
	m.monitorMapMu.Unlock()
	if !ok {
		err := m.addMonitor(monitorInfo)
		if err != nil {
			logger.Warning(err)
			return
		}

		return
	}
	logger.Debugf("updateMonitor %v", monitorInfo.Name)
	monitorInfo.dumpForDebug()

	m.handleMonitorConnectedChanged(monitor, monitorInfo.Connected)
	monitor.PropsMu.Lock()

	if monitor.uuid != monitorInfo.UUID {
		logger.Debugf("%v uuid changed, old:%q, new %q", monitor, monitor.uuid, monitorInfo.UUID)
	}
	monitor.uuid = monitorInfo.UUID
	monitor.realConnected = monitorInfo.Connected
	monitor.setPropAvailableFillModes(monitorInfo.AvailableFillModes)
	monitor.setPropManufacturer(monitorInfo.Manufacturer)
	monitor.setPropModel(monitorInfo.Model)
	monitor.setPropModes(m.filterModeInfos(monitorInfo.Modes, monitorInfo.PreferredMode))
	bestMode := getBestMode(monitor.Modes, monitorInfo.PreferredMode)
	monitor.setPropBestMode(bestMode)
	var preferredModes []ModeInfo
	if !bestMode.isZero() {
		preferredModes = []ModeInfo{bestMode}
	}
	monitor.setPropPreferredModes(preferredModes)
	monitor.setPropMmWidth(monitorInfo.MmWidth)
	monitor.setPropMmHeight(monitorInfo.MmHeight)
	monitor.setPropX(monitorInfo.X)
	monitor.setPropY(monitorInfo.Y)
	monitor.setPropWidth(monitorInfo.Width)
	monitor.setPropHeight(monitorInfo.Height)
	// NOTE: 前端 dcc 要求在设置了 Width 和 Height 之后，再设置 Connected 和 Enabled。
	monitor.setPropConnected(monitorInfo.VirtualConnected)
	monitor.setPropEnabled(monitorInfo.Enabled)

	monitor.setPropReflects(getReflects(monitorInfo.Rotations))
	monitor.setPropRotations(getRotations(monitorInfo.Rotations))
	rotation, reflectProp := parseCrtcRotation(monitorInfo.Rotation)
	monitor.setPropRotation(rotation)
	monitor.setPropReflect(reflectProp)

	monitor.setPropCurrentMode(monitorInfo.CurrentMode)
	monitor.setPropRefreshRate(monitorInfo.CurrentMode.Rate)

	monitor.PropsMu.Unlock()
}

func (m *Manager) handleMonitorConnectedChanged(monitor *Monitor, connected bool) {
	err := m.recordMonitorConnected(monitor.Name, connected, time.Now())
	if err != nil {
		logger.Warning(err)
	}
	if connected {
		// 连接
	} else {
		// 断开
		m.updateBuiltinMonitorOnDisconnected(monitor.ID)
	}
}

func (m *Manager) buildConfigForModeMirror(monitors Monitors) (monitorCfgs SysMonitorConfigs, err error) {
	logger.Debug("switch mode mirror")
	commonSizes := getMonitorsCommonSizes(monitors)
	if len(commonSizes) == 0 {
		err = errors.New("not found common size")
		return
	}
	maxSize := getMaxAreaSize(commonSizes)
	primaryMonitor := m.getDefaultPrimaryMonitor(monitors)
	for _, monitor := range monitors {
		cfg := monitor.toBasicSysConfig()
		cfg.Enabled = true
		if monitor.ID == primaryMonitor.ID {
			cfg.Primary = true
		}
		mode := getFirstModeBySize(monitor.Modes, maxSize.width, maxSize.height)
		cfg.Width = mode.Width
		cfg.Height = mode.Height
		cfg.RefreshRate = mode.Rate
		cfg.X = 0
		cfg.Y = 0
		cfg.Rotation = randr.RotationRotate0
		cfg.Reflect = 0
		cfg.Brightness = 1
		monitorCfgs = append(monitorCfgs, cfg)
	}
	return
}

func (m *Manager) applyModeMirror(monitorsId string, monitorMap map[uint32]*Monitor, options applyOptions) (err error) {
	logger.Debug("apply mode mirror")
	screenCfg := m.getSysScreenConfig(monitorsId)
	configs := screenCfg.getMonitorConfigs(DisplayModeMirror, "")
	monitors := getConnectedMonitors(monitorMap)

	needSaveCfg := false
	if len(configs) == 0 {
		needSaveCfg = true
		configs, err = m.buildConfigForModeMirror(monitors)
		if err != nil {
			return
		}
	}

	err = m.applySysMonitorConfigs(DisplayModeMirror, monitorsId, monitorMap, configs, options)
	if err != nil {
		return
	}

	if needSaveCfg {
		screenCfg.setMonitorConfigs(DisplayModeMirror, "", configs)
		return m.saveSysConfig("mode mirror")
	}

	return
}

func (m *Manager) getSuitableSysMonitorConfigs(displayMode byte) SysMonitorConfigs {
	monitors := m.getConnectedMonitors()
	screenCfg := m.getSysScreenConfig(monitors.getMonitorsId())
	if len(monitors) == 0 {
		return nil
	} else if len(monitors) == 1 {
		return screenCfg.getSingleMonitorConfigs()
	}
	uuid := getOnlyOneMonitorUuid(displayMode, monitors)
	return screenCfg.getMonitorConfigs(displayMode, uuid)
}

func (m *Manager) getSuitableUserMonitorModeConfig(displayMode byte) *UserMonitorModeConfig {
	monitors := m.getConnectedMonitors()
	screenCfg := m.getUserScreenConfig()
	if len(monitors) == 0 {
		return nil
	} else if len(monitors) == 1 {
		return screenCfg[KeySingle]
	}
	uuid := getOnlyOneMonitorUuid(displayMode, monitors)
	return screenCfg.getMonitorModeConfig(displayMode, uuid)
}

func (m *Manager) modifySuitableSysMonitorConfigs(fn func(configs SysMonitorConfigs) SysMonitorConfigs) {
	monitors := m.getConnectedMonitors()
	screenCfg := m.getSysScreenConfig(monitors.getMonitorsId())
	if len(monitors) == 0 {
		return
	} else if len(monitors) == 1 {
		configs := screenCfg.getSingleMonitorConfigs()
		configs = fn(configs)
		screenCfg.setSingleMonitorConfigs(configs)
		return
	}
	displayMode := m.DisplayMode
	uuid := getOnlyOneMonitorUuid(displayMode, monitors)
	configs := screenCfg.getMonitorConfigs(displayMode, uuid)
	configs = fn(configs)
	screenCfg.setMonitorConfigs(displayMode, uuid, configs)
}

// 获取 OnlyOne 显示模式下启用显示器的 UUID，其他显示模式下返回空。
func getOnlyOneMonitorUuid(displayMode byte, monitors Monitors) (uuid string) {
	if displayMode == DisplayModeOnlyOne {
		for _, monitor := range monitors {
			if monitor.Enabled {
				uuid = monitor.uuid
				break
			}
		}
	}
	return
}

func (m *Manager) modifySuitableUserMonitorModeConfig(fn func(cfg *UserMonitorModeConfig)) {
	monitors := m.getConnectedMonitors()
	screenCfg := m.getUserScreenConfig()
	if len(monitors) == 0 {
		return
	} else if len(monitors) == 1 {
		cfg := screenCfg[KeySingle]
		if cfg == nil {
			cfg = getDefaultUserMonitorModeConfig()
		}
		fn(cfg)
		screenCfg[KeySingle] = cfg
		return
	}
	displayMode := m.DisplayMode
	uuid := getOnlyOneMonitorUuid(displayMode, monitors)
	cfg := screenCfg.getMonitorModeConfig(displayMode, uuid)
	if cfg == nil {
		cfg = getDefaultUserMonitorModeConfig()
	}
	fn(cfg)
	screenCfg.setMonitorModeConfig(displayMode, uuid, cfg)
}

type screenSize struct {
	width    uint16
	height   uint16
	mmWidth  uint32
	mmHeight uint32
}

func (m *Manager) apply(monitorsId string, monitorMap map[uint32]*Monitor, options applyOptions) error {
	// 当前的屏幕大小
	m.PropsMu.RLock()
	prevScreenSize := screenSize{width: m.ScreenWidth, height: m.ScreenHeight}
	m.PropsMu.RUnlock()
	m.setInApply(true)

	// NOTE: 应该限制只有 Manager.apply 才能调用 mm.apply
	m.applyMu.Lock()
	err := m.mm.apply(monitorsId, monitorMap, prevScreenSize, options, m.sysConfig.Config.FillModes)
	m.applyMu.Unlock()

	m.setInApply(false)
	return err
}

func (m *Manager) getInApply() bool {
	m.PropsMu.Lock()
	defer m.PropsMu.Unlock()
	return m.inApply
}

func (m *Manager) setInApply(value bool) {
	m.PropsMu.Lock()
	m.inApply = value
	m.PropsMu.Unlock()
}

func (m *Manager) handlePrimaryRectChanged(pmi primaryMonitorInfo) {
	logger.Debug("handlePrimaryRectChanged", pmi)
	m.PropsMu.Lock()
	defer m.PropsMu.Unlock()

	m.setPropPrimary(pmi.Name)
	if !pmi.IsRectEmpty() {
		m.setPropPrimaryRect(pmi.Rect)
	}
}

func (m *Manager) setPrimary(name string) error {
	switch m.DisplayMode {
	case DisplayModeMirror:
		return errors.New("not allow set primary in mirror mode")

	case DisplayModeOnlyOne:
		options := applyOptions{
			optionOnlyOne: name,
		}
		monitorMap := m.cloneMonitorMap()
		monitorsId := getConnectedMonitors(monitorMap).getMonitorsId()
		return m.applyModeOnlyOne(monitorsId, monitorMap, options)

	case DisplayModeExtend:
		monitorMap := m.cloneMonitorMap()
		monitors := getConnectedMonitors(monitorMap)
		monitorsId := monitors.getMonitorsId()
		screenCfg := m.getSysScreenConfig(monitorsId)
		configs := screenCfg.getMonitorConfigs(DisplayModeExtend, "")

		var primaryMonitor *Monitor
		for _, monitor := range monitorMap {
			if monitor.Name != name {
				continue
			}

			if !monitor.realConnected {
				return errors.New("monitor is not connected")
			}

			primaryMonitor = monitor
			break
		}

		if primaryMonitor == nil {
			return errors.New("not found primary monitor")
		}

		if len(configs) == 0 {
			configs = toSysMonitorConfigs(monitors, primaryMonitor.Name)
		} else {
			// modify configs
			// TODO 这里为什么需要更新 Name？
			updateSysMonitorConfigsName(configs, m.monitorMap)
			configs.setPrimary(primaryMonitor.uuid)
		}

		err := m.mm.setMonitorPrimary(primaryMonitor.ID)
		if err != nil {
			return err
		}

		screenCfg.setMonitorConfigs(DisplayModeExtend, "", configs)

		err = m.saveSysConfig("primary changed")
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("invalid display mode %v", m.DisplayMode)
	}
	return nil
}

func (m *Manager) buildConfigForModeExtend(monitors Monitors) (monitorCfgs SysMonitorConfigs, err error) {
	// 先获取主屏
	var primaryMonitor *Monitor
	primaryMonitor = m.getDefaultPrimaryMonitor(monitors)

	sortMonitorsByPrimaryAndId(monitors, primaryMonitor)
	var xOffset int

	for _, monitor := range monitors {
		cfg := monitor.toBasicSysConfig()
		cfg.Enabled = true
		if monitor.ID == primaryMonitor.ID {
			cfg.Primary = true
		}
		mode := monitor.BestMode
		// 不用考虑旋转，默认不旋转
		cfg.Width = mode.Width
		cfg.Height = mode.Height
		cfg.RefreshRate = mode.Rate

		if xOffset > math.MaxInt16 {
			xOffset = math.MaxInt16
		}
		cfg.X = int16(xOffset)
		//cfg.Y = 0
		cfg.Rotation = randr.RotationRotate0
		//cfg.Reflect = 0
		cfg.Brightness = 1
		xOffset += int(cfg.Width)
		monitorCfgs = append(monitorCfgs, cfg)
	}
	return
}

func (m *Manager) applyModeExtend(monitorsId string, monitorMap map[uint32]*Monitor, options applyOptions) (err error) {
	logger.Debug("apply mode extend")
	screenCfg := m.getSysScreenConfig(monitorsId)
	configs := screenCfg.getMonitorConfigs(DisplayModeExtend, "")
	monitors := getConnectedMonitors(monitorMap)

	needSaveCfg := false
	if len(configs) == 0 {
		needSaveCfg = true
		configs, err = m.buildConfigForModeExtend(monitors)
		if err != nil {
			return
		}
	}

	err = m.applySysMonitorConfigs(DisplayModeExtend, monitorsId, monitorMap, configs, options)
	if err != nil {
		return
	}

	if needSaveCfg {
		screenCfg.setMonitorConfigs(DisplayModeExtend, "", configs)
		return m.saveSysConfig("mode extend")
	}
	return
}

// getSysScreenConfig 根据 monitorsId 参数返回不同的屏幕配置，不同 monitorsId 则屏幕配置不同。
// monitorsId 代表了已连接了哪些显示器。
func (m *Manager) getSysScreenConfig(monitorsId string) *SysScreenConfig {
	// TODO: 这个方法其实不安全，因为返回了 SysScreenConfig 指针。
	m.sysConfig.mu.Lock()
	defer m.sysConfig.mu.Unlock()

	screenCfg := m.sysConfig.Config.Screens[monitorsId]
	if screenCfg == nil {
		if m.sysConfig.Config.Screens == nil {
			m.sysConfig.Config.Screens = make(map[string]*SysScreenConfig)
		}
		screenCfg = &SysScreenConfig{}
		m.sysConfig.Config.Screens[monitorsId] = screenCfg
	}
	return screenCfg
}

func (cfg *SysConfig) getMonitorConfigs(monitorId string, displayMode byte, single bool) SysMonitorConfigs {
	sc := cfg.Screens[monitorId]
	if sc == nil {
		return nil
	}
	if single {
		return sc.getSingleMonitorConfigs()
	}
	return sc.getMonitorConfigs(displayMode, sc.OnlyOneUuid)
}

func (m *Manager) buildConfigForModeOnlyOne(monitors Monitors, uuid string) (monitorCfgs SysMonitorConfigs, err error) {
	for _, monitor := range monitors {
		mode := monitor.BestMode
		cfg := monitor.toBasicSysConfig()
		if monitor.uuid == uuid {
			cfg.Enabled = true
			cfg.Primary = true
			cfg.Width = mode.Width
			cfg.Height = mode.Height
			cfg.RefreshRate = mode.Rate
			//cfg.X = 0
			//cfg.Y = 0
			cfg.Rotation = randr.RotationRotate0
			//cfg.Reflect = 0
			cfg.Brightness = 1
			monitorCfgs = append(monitorCfgs, cfg)
			return
		}
	}
	return
}

func (m *Manager) applyModeOnlyOne(monitorsId string, monitorMap map[uint32]*Monitor, options applyOptions) (err error) {
	name, _ := options[optionOnlyOne].(string)
	logger.Debug("apply mode only one", name)

	monitors := getConnectedMonitors(monitorMap)
	screenCfg := m.getSysScreenConfig(monitorsId)
	uuid := ""
	needSaveCfg := false
	if name == "" {
		// 未指定名称
		uuid = screenCfg.OnlyOneUuid
		monitor := monitors.GetByUuid(uuid)
		if monitor == nil {
			// uuid 无效
			uuid = ""
		}
	} else {
		// 指定了名称
		monitor := monitors.GetByName(name)
		if monitor != nil {
			uuid = monitor.uuid
			screenCfg.OnlyOneUuid = uuid
			needSaveCfg = true
		} else {
			// 名称指定错误
			return InvalidOutputNameError{Name: name}
		}
	}

	if uuid == "" {
		primaryMonitor := m.getDefaultPrimaryMonitor(monitors)
		if primaryMonitor == nil {
			return errors.New("not found primary monitor")
		}
		uuid = primaryMonitor.uuid
		screenCfg.OnlyOneUuid = uuid
		needSaveCfg = true
	}
	// 必须要有 uuid
	if uuid == "" {
		return errors.New("uuid is empty")
	}

	configs := screenCfg.getMonitorConfigs(DisplayModeOnlyOne, uuid)

	if len(configs) == 0 {
		needSaveCfg = true
		configs, err = m.buildConfigForModeOnlyOne(monitors, uuid)
		if err != nil {
			return
		}
	}

	err = m.applySysMonitorConfigs(DisplayModeOnlyOne, monitorsId, monitorMap, configs, options)
	if err != nil {
		return
	}

	if needSaveCfg {
		screenCfg.setMonitorConfigs(DisplayModeOnlyOne, uuid, configs)
		return m.saveSysConfig("mode only one")
	}

	return
}

func (m *Manager) switchMode(mode byte, name string) (err error) {
	options := applyOptions{
		optionOnlyOne: name,
		// 替代之前的 modeChanged
		optionDisableCrtc: true,
	}
	oldMode := m.DisplayMode
	monitorMap := m.cloneMonitorMap()
	monitorsId := getConnectedMonitors(monitorMap).getMonitorsId()
	err = m.applyDisplayConfig(mode, monitorsId, monitorMap, true, options)
	if err != nil {
		logger.Warning(err)

		// 模式切换失败，回退到之前的模式
		monitorMap := m.cloneMonitorMap()
		monitorsId := getConnectedMonitors(monitorMap).getMonitorsId()
		err1 := m.applyDisplayConfig(oldMode, monitorsId, monitorMap, true, options)
		if err1 != nil {
			logger.Warning(err1)
		}

		return err
	}
	if oldMode != mode {
		// 保存设置
		m.sysConfig.mu.Lock()
		m.sysConfig.Config.DisplayMode = mode
		err = m.saveSysConfigNoLock("switch mode")
		m.sysConfig.mu.Unlock()

		if err != nil {
			logger.Warning(err)
			return err
		}
	}

	return nil
}

func (m *Manager) setDisplayMode(mode byte) {
	m.setPropDisplayMode(mode)
	m.sysConfig.Config.DisplayMode = mode
}

func (m *Manager) save() (err error) {
	m.PropsMu.RLock()
	if !m.HasChanged {
		m.PropsMu.RUnlock()
		logger.Debug("no save, no changed")
		return nil
	}
	m.PropsMu.RUnlock()

	logger.Debug("save")
	monitorMap := m.cloneMonitorMap()
	monitors := getConnectedMonitors(monitorMap)
	if len(monitors) == 0 {
		err = errors.New("no monitor connected")
		return
	}

	screenCfg := m.getSysScreenConfig(monitors.getMonitorsId())

	if len(monitors) == 1 {
		screenCfg.setSingleMonitorConfigs(SysMonitorConfigs{monitors[0].toSysConfig()})
	} else {
		// TODO 为什么需要这里的代码？
		var primaryName string
		//当为扩展屏幕的时候，设置默认屏为配置文件中默认屏幕
		if m.DisplayMode == DisplayModeExtend && screenCfg.Extend != nil {
			for _, monitor := range screenCfg.Extend.Monitors {
				if monitor.Primary {
					primaryName = monitor.Name
				}
			}
		}
		//没找到主屏或者模式非扩展模式，则取默认值
		if primaryName == "" {
			primaryName = m.Primary
		}

		logger.Debugf("display mode: %d, primary name: %s", m.DisplayMode, primaryName)
		uuid := getOnlyOneMonitorUuid(m.DisplayMode, monitors)
		screenCfg.setMonitorConfigs(m.DisplayMode, uuid, toSysMonitorConfigs(monitors, primaryName))
	}

	err = m.saveSysConfig("save")
	if err != nil {
		return err
	}
	m.markClean()
	return nil
}

func (m *Manager) markClean() {
	logger.Debug("markClean")
	m.monitorMapMu.Lock()
	for _, monitor := range m.monitorMap {
		monitor.PropsMu.Lock()
		monitor.backup = nil
		monitor.PropsMu.Unlock()
	}
	m.monitorMapMu.Unlock()

	m.PropsMu.Lock()
	m.setPropHasChanged(false)
	m.PropsMu.Unlock()
}

func (m *Manager) applyChanges() error {
	if m.getInApply() {
		logger.Debug("no apply changes, in apply")
		return nil
	}

	m.PropsMu.RLock()
	if !m.HasChanged {
		m.PropsMu.RUnlock()
		logger.Debug("no apply changes, no changed")
		return nil
	}
	m.PropsMu.RUnlock()

	monitorMap := m.cloneMonitorMap()
	monitors := getConnectedMonitors(monitorMap)
	monitorsId := monitors.getMonitorsId()

	// 检查 monitors 的配置
	err := checkMonitors(monitors, m.DisplayMode)
	if err != nil {
		logger.Warning("checkMonitors failed:", err)
		m.resetChangesWithoutApply()
		return err
	}

	err = m.apply(monitorsId, monitorMap, nil)
	return err
}

func (m *Manager) resetChangesWithoutApply() {
	m.PropsMu.Lock()
	if !m.HasChanged {
		m.PropsMu.Unlock()
		return
	}
	m.setPropHasChanged(false)
	m.PropsMu.Unlock()

	m.monitorMapMu.Lock()
	for _, monitor := range m.monitorMap {
		monitor.resetChanges()
	}
	m.monitorMapMu.Unlock()
}

func checkMonitors(monitors Monitors, mode byte) error {
	logger.Debug("checkMonitors mode:", mode)
	configs := toSysMonitorConfigs(monitors, "")
	if logger.GetLogLevel() == log.LevelDebug {
		logger.Debug("checkMonitors configs:", spew.Sdump(configs))
	}
	if len(configs) > 1 {
		if mode == DisplayModeExtend {
			zeroPosCount := 0
			for _, config := range configs {
				if config.X == 0 && config.Y == 0 {
					zeroPosCount++
				}
			}
			logger.Debug("zeroPosCount:", zeroPosCount)
			if zeroPosCount > 1 {
				return errors.New("display mode is extend, but the position of more than one monitor is at (0,0)")
			}
		}
	}
	return nil
}

func (m *Manager) getConnectedMonitors() Monitors {
	m.monitorMapMu.Lock()
	defer m.monitorMapMu.Unlock()
	return getConnectedMonitors(m.monitorMap)
}

func (m *Manager) getVirtualConnectedMonitors() Monitors {
	m.monitorMapMu.Lock()
	defer m.monitorMapMu.Unlock()
	return getVirtualConnectedMonitors(m.monitorMap)
}

func (m *Manager) cloneMonitorMap() map[uint32]*Monitor {
	m.monitorMapMu.Lock()
	defer m.monitorMapMu.Unlock()

	return m.cloneMonitorMapNoLock()
}

func (m *Manager) cloneMonitorMapNoLock() map[uint32]*Monitor {
	result := make(map[uint32]*Monitor, len(m.monitorMap))
	for id, monitor := range m.monitorMap {
		result[id] = monitor.clone()
	}
	return result
}

func (m *Manager) applySysMonitorConfigs(mode byte, monitorsId string, monitorMap map[uint32]*Monitor, configs SysMonitorConfigs, options applyOptions) error {
	logger.Debug("applySysMonitorConfigs", spew.Sdump(configs), options)

	// 验证配置
	enabledCount := 0
	for _, config := range configs {
		if config.Enabled {
			enabledCount++
		}
	}
	if enabledCount == 0 {
		return errors.New("invalid configs: no enabled monitor")
	}

	var primaryMonitorID uint32
	var enabledMonitors []*Monitor
	for _, monitor := range monitorMap {
		monitorCfg := configs.getByUuid(monitor.uuid)
		if monitorCfg == nil {
			logger.Debug("disable monitor", monitor)
			monitor.Enabled = false
		} else {
			if monitorCfg.Enabled {
				logger.Debug("enable monitor", monitor)
				if monitorCfg.Primary {
					primaryMonitorID = monitor.ID
				}
				enabledMonitors = append(enabledMonitors, monitor)
				//所有可设置的值都设置为配置文件中的值
				monitor.X = monitorCfg.X
				monitor.Y = monitorCfg.Y
				monitor.Rotation = monitorCfg.Rotation
				monitor.Reflect = monitorCfg.Reflect

				// monitorCfg 中的宽和高是经过 rotation 调整的
				width := monitorCfg.Width
				height := monitorCfg.Height
				swapWidthHeightWithRotation(monitorCfg.Rotation, &width, &height)
				mode := monitor.selectMode(width, height, monitorCfg.RefreshRate)
				monitor.setModeNoEmitChanged(mode)
				monitor.Enabled = true
			} else {
				logger.Debug("disable monitor", monitor)
				monitor.Enabled = false
			}
		}
	}

	if primaryMonitorID == 0 {
		primaryMonitorID = m.getDefaultPrimaryMonitor(enabledMonitors).ID
	}

	// 对于 X 来说，这里是处理 crtc 设置
	err := m.apply(monitorsId, monitorMap, options)
	if err != nil {
		return err
	}

	// NOTE: DisplayMode 属性改变信号应该在设置各个 Monitor 的属性之后，否则会引发前端 dcc 的 bug。
	if mode != DisplayModeInvalid {
		m.setPropDisplayMode(mode)
	}

	// 异步处理亮度设置
	go func() {
		for _, config := range configs {
			if config.Enabled {
				err := m.setBrightness(config.Name, config.Brightness)
				if err != nil {
					logger.Warning(err)
				}
			}
		}
		m.syncPropBrightness()
	}()

	// NOTE: Primary 和 PrimaryRect 属性改变信号应该在 DisplayMode 属性改变之后，否则会引发前端 dcc 的 bug。
	err = m.mm.setMonitorPrimary(primaryMonitorID)
	if err != nil {
		logger.Warning(err)
	}

	return nil
}

func (m *Manager) getDefaultPrimaryMonitor(monitors []*Monitor) *Monitor {
	if len(monitors) == 0 {
		return nil
	}
	builtinMonitor := m.getBuiltinMonitor()
	if builtinMonitor != nil && Monitors(monitors).GetById(builtinMonitor.ID) != nil {
		return builtinMonitor
	}

	monitor := m.getPriorMonitor(monitors)
	return monitor
}

func (m *Manager) getMonitorConnectTime(name string) time.Time {
	m.sysConfig.mu.Lock()
	defer m.sysConfig.mu.Unlock()
	return m.sysConfig.Config.Cache.ConnectTime[name]
}

// getPriorMonitor 获取优先级最高的显示器，用于作为主屏。
func (m *Manager) getPriorMonitor(monitors []*Monitor) *Monitor {
	if len(monitors) == 0 {
		return nil
	}
	sort.Slice(monitors, func(i, j int) bool {
		mi := monitors[i]
		mj := monitors[j]

		pi := getPortPriority(mi.Name)
		pj := getPortPriority(mj.Name)

		// 多字段排序
		// 按优先级从小到大排序，如果优先级相同，按最后连接时间从早到晚排序。
		if pi == pj {
			ti := m.getMonitorConnectTime(mi.Name)
			tj := m.getMonitorConnectTime(mj.Name)
			return ti.Before(tj)
		}
		return pi < pj
	})
	return monitors[0]
}

// getPortType 根据显示器名称判断出端口类型，比如 vga，hdmi，edp 等。
func getPortType(name string) string {
	i := strings.IndexRune(name, '-')
	if i != -1 {
		name = name[0:i]
	}
	return strings.ToLower(name)
}

func getPortPriority(name string) int {
	portType := getPortType(name)
	p, ok := _monitorTypePriority[portType]
	if ok {
		return p
	}
	return priorityOther
}

func (m *Manager) getMonitorsId() string {
	return getConnectedMonitors(m.cloneMonitorMap()).getMonitorsId()
}

// updatePropMonitors 把所有已连接显示器的对象路径设置到 Manager 的 Monitors 属性。
func (m *Manager) updatePropMonitors() {
	monitors := m.getConnectedMonitors()
	paths := monitors.getPaths()
	logger.Debug("update prop Monitors:", paths)
	m.PropsMu.Lock()
	m.setPropMonitors(paths)
	m.PropsMu.Unlock()
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
		return nil, xerrors.New("no matched touchscreen ID")
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
	touchScreenUUID := ""
	i := -1
	for index, v := range m.Touchscreens {
		if v.path == path {
			i = index
			touchScreenUUID = v.uuid
		}
	}

	if i == -1 {
		return
	}

	if touchScreenUUID != "" {
		m.touchScreenDialogMutex.RLock()
		existCmd, ok := m.touchScreenDialogMap[touchScreenUUID]
		m.touchScreenDialogMutex.RUnlock()
		if ok && existCmd != nil && existCmd.Process != nil {
			if existCmd.ProcessState == nil {
				logger.Debug("to kill process of touchScreenDialog.")
				err := existCmd.Process.Kill()
				if err != nil {
					logger.Warning("failed to kill process of touchScreenDialog, error:", err)
				}
			}
		}
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
	_, err := m.dbusDaemon.ConnectNameOwnerChanged(func(name, oldOwner, newOwner string) {
		if name == m.inputDevices.ServiceName_() && newOwner == "" {
			m.setPropTouchscreens(nil)
		}
	})
	if err != nil {
		logger.Warning(err)
	}

	_, err = m.inputDevices.ConnectTouchscreenAdded(func(path dbus.ObjectPath) {
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
	m.touchScreenDialogMap = make(map[string]*exec.Cmd)

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
		matrix := genTransformationMatrix(monitor0.X, monitor0.Y, monitor0.Width, monitor0.Height, monitor0.Rotation|monitor0.Reflect)
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

func (m *Manager) loadUserConfig() error {
	content, err := ioutil.ReadFile(userConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var cfg UserConfig
	err = json.Unmarshal(content, &cfg)
	if err != nil {
		return err
	}
	cfg.fix()
	m.userConfig = cfg
	return nil
}

func (m *Manager) saveUserConfig() error {
	if _greeterMode {
		return nil
	}
	m.userCfgMu.Lock()
	defer m.userCfgMu.Unlock()

	m.userConfig.Version = userConfigVersion
	if logger.GetLogLevel() == log.LevelDebug {
		if m.debugOpts.printSaveCfgDetail {
			logger.Debug("saveUserConfig", spew.Sdump(m.userConfig))
		} else {
			logger.Debug("saveUserConfig")
		}
	}
	content, err := json.Marshal(m.userConfig)
	if err != nil {
		return err
	}
	dir := filepath.Dir(userConfigFile)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	filename := userConfigFile + ".new"
	err = ioutil.WriteFile(filename, content, 0644)
	if err != nil {
		return err
	}
	err = os.Rename(filename, userConfigFile)
	return err
}

func (m *Manager) loadSysConfig() {
	cfg, err := m.getSysConfig()
	if err != nil {
		logger.Warning(err)
		// 修正一下空配置
		m.sysConfig.fix()
	} else {
		m.sysConfig.copyFrom(cfg)
	}
}

func (c *SysRootConfig) fix() {
	cfg := &c.Config
	// 默认显示模式为复制模式
	if cfg.DisplayMode == DisplayModeUnknown || cfg.DisplayMode == DisplayModeCustom {
		cfg.DisplayMode = DisplayModeMirror
	}
	for _, screenConfig := range cfg.Screens {
		screenConfig.fix()
	}
}

// 无需对结果再次地调用 fix 方法
func (m *Manager) getSysConfig() (*SysRootConfig, error) {
	cfgJson, err := m.sysDisplay.GetConfig(0)
	if err != nil {
		return nil, err
	}
	var rootCfg SysRootConfig
	err = jsonUnmarshal(cfgJson, &rootCfg)
	if err != nil {
		return nil, err
	}
	rootCfg.fix()
	return &rootCfg, nil
}

// saveSysConfig 保存系统级配置
func (m *Manager) saveSysConfig(reason string) error {
	m.sysConfig.mu.Lock()
	defer m.sysConfig.mu.Unlock()

	err := m.saveSysConfigNoLock(reason)
	return err
}

type debugOptions struct {
	printSaveCfgDetail bool
}

func (m *Manager) initDebugOptions() {
	m.debugOpts.printSaveCfgDetail = os.Getenv("DISPLAY_PRINT_SAVE_CFG_DETAIL") == "1"
}

func (m *Manager) saveSysConfigNoLock(reason string) error {
	if _greeterMode {
		return nil
	}
	m.sysConfig.UpdateAt = time.Now().Format(time.RFC3339Nano)
	m.sysConfig.Version = sysConfigVersion

	if logger.GetLogLevel() == log.LevelDebug {
		if m.debugOpts.printSaveCfgDetail {
			logger.Debugf("saveSysConfig reason: %s, sysConfig: %s", reason, spew.Sdump(&m.sysConfig))
		} else {
			logger.Debugf("saveSysConfig reason: %s", reason)
		}
	}

	cfgJson := jsonMarshal(&m.sysConfig)
	err := m.sysDisplay.SetConfig(0, cfgJson)
	return err
}

func (m *Manager) setMonitorFillMode(monitor *Monitor, fillMode string) error {
	m.setFillModeMu.Lock()
	defer m.setFillModeMu.Unlock()

	if len(monitor.AvailableFillModes) == 0 {
		return errors.New("monitor do not support set fill mode")
	}

	m.sysConfig.mu.Lock()
	cfg := &m.sysConfig.Config
	fillModeKey := monitor.generateFillModeKey()
	if fillMode == "" {
		fillMode = cfg.FillModes[fillModeKey]
	}
	m.sysConfig.mu.Unlock()
	if fillMode == "" {
		fillMode = fillModeDefault
	}

	logger.Debugf("%v set fill mode %v", monitor, fillMode)

	err := m.mm.setMonitorFillMode(monitor, fillMode)
	if err != nil {
		return err
	}

	m.sysConfig.mu.Lock()
	if cfg.FillModes == nil {
		cfg.FillModes = make(map[string]string)
	}
	cfg.FillModes[fillModeKey] = fillMode
	err = m.saveSysConfigNoLock("fill mode changed")
	m.sysConfig.mu.Unlock()

	return err
}

func (m *Manager) showTouchscreenDialog(touchScreenUUID, touchscreenSerial string) error {
	m.touchScreenDialogMutex.RLock()
	existCmd, ok := m.touchScreenDialogMap[touchScreenUUID]
	m.touchScreenDialogMutex.RUnlock()
	if ok && existCmd != nil {
		// 已经存在dialog，不重复打开dialog
		logger.Debug("showTouchscreenDialog failed, touchScreen is existed:", touchScreenUUID)
		return nil
	}

	cmd := exec.Command(cmdTouchscreenDialogBin, touchscreenSerial)

	err := cmd.Start()
	if err != nil {
		return err
	}

	m.touchScreenDialogMutex.Lock()
	m.touchScreenDialogMap[touchScreenUUID] = cmd
	m.touchScreenDialogMutex.Unlock()

	go func() {
		err = cmd.Wait()
		if err != nil {
			logger.Debug(err)
		}
		m.touchScreenDialogMutex.Lock()
		if _, ok := m.touchScreenDialogMap[touchScreenUUID]; ok {
			delete(m.touchScreenDialogMap, touchScreenUUID)
		}
		m.touchScreenDialogMutex.Unlock()
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

/* 根据从内核获取的屏幕的初始状态(屏幕的方向)，旋转桌面到对应的方向 */
func (m *Manager) initScreenRotation() {
	if m.sensorProxy == nil {
		m.sensorProxy = m.sysBus.Object(sensorProxyInterface, sensorProxyPath)
	}

	screenRatationStatus := "normal"
	if m.sensorProxy != nil {
		err := m.sensorProxy.Call(sensorProxyGetScreenStatus, 0).Store(&screenRatationStatus)
		if err != nil {
			logger.Warning("failed to get screen rotation status", err)
			return
		}

		startBuildInScreenRotationMutex.Lock()
		defer startBuildInScreenRotationMutex.Unlock()
		rotationRotate, ok := rotationScreenValue[strings.TrimSpace(screenRatationStatus)]
		if ok {
			m.startBuildInScreenRotation(rotationRotate)
		}
	}
}

// 检查当前连接的所有触控面板, 如果没有映射配置, 那么调用 OSD 弹窗.
func (m *Manager) showTouchscreenDialogs() {
	for _, touch := range m.Touchscreens {
		if _, ok := m.touchscreenMap[touch.uuid]; !ok {
			logger.Debug("cannot find touchscreen", touch.uuid, "'s configure, show OSD")
			err := m.showTouchscreenDialog(touch.uuid, touch.Serial)
			if err != nil {
				logger.Warning("shotTouchscreenOSD", err)
			}
		}
	}
}

// syncPropBrightness 将亮度从每个显示器 monitor.Brightness 同步到 Manager 的属性 Brightness 中。
func (m *Manager) syncPropBrightness() {
	monitors := m.getConnectedMonitors()
	newMap := make(map[string]float64)
	for _, monitor := range monitors {
		newMap[monitor.Name] = monitor.Brightness
	}
	m.PropsMu.Lock()
	m.setPropBrightness(newMap)
	m.PropsMu.Unlock()
}

func (m *Manager) getRateFilter() RateFilterMap {
	data := make(RateFilterMap)
	jsonStr := m.settings.GetString(gsKeyRateFilter)
	err := json.Unmarshal([]byte(jsonStr), &data)
	if err != nil {
		logger.Warning(err)
		return data
	}

	return data
}

func (m *Manager) listenRotateSignal() {
	systemBus, err := dbus.SystemBus()
	if err != nil {
		logger.Fatal(err)
	}

	err = systemBus.BusObject().AddMatchSignal(sensorProxyInterface, sensorProxySignalName,
		dbus.WithMatchObjectPath(sensorProxyPath), dbus.WithMatchSender(sensorProxyInterface)).Err
	if err != nil {
		logger.Fatal(err)
	}

	signalCh := make(chan *dbus.Signal, 10)
	m.sysBus.Signal(signalCh)
	go func() {
		var rotationScreenTimer *time.Timer
		rotateScreenValue := "normal"

		for sig := range signalCh {
			if sig.Path != sensorProxyPath || sig.Name != sensorProxySignal {
				continue
			}

			err = dbus.Store(sig.Body, &rotateScreenValue)
			if err != nil {
				logger.Warning("call dbus.Store err:", err)
				continue
			}

			if rotationScreenTimer == nil {
				rotationScreenTimer = time.AfterFunc(time.Millisecond*time.Duration(m.rotateScreenTimeDelay), func() {
					startBuildInScreenRotationMutex.Lock()
					defer startBuildInScreenRotationMutex.Unlock()
					rotationRotate, ok := rotationScreenValue[strings.TrimSpace(rotateScreenValue)]
					if ok {
						m.startBuildInScreenRotation(rotationRotate)
					}
				})
			} else {
				rotationScreenTimer.Reset(time.Millisecond * time.Duration(m.rotateScreenTimeDelay))
			}
		}
	}()
}

func (m *Manager) startBuildInScreenRotation(latestRotationValue uint16) {
	// 判断旋转信号值是否符合要求
	if latestRotationValue != randr.RotationRotate0 &&
		latestRotationValue != randr.RotationRotate90 &&
		latestRotationValue != randr.RotationRotate270 {
		logger.Warningf("get Rotation screen value failed: %d", latestRotationValue)
		return
	}

	if m.builtinMonitor != nil {
		err := m.builtinMonitor.SetRotation(latestRotationValue)
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

		m.builtinMonitor.setPropCurrentRotateMode(RotationFinishModeAuto)
	}
}

func (m *Manager) listenSettingsChanged() {
	if m.settings == nil {
		m.rotateScreenTimeDelay = defaultRotateScreenTimeDelay
		return
	}

	gsettings.ConnectChanged(gsSchemaDisplay, "*", func(key string) {
		switch key {
		case gsKeyRotateScreenTimeDelay:
			m.rotateScreenTimeDelay = m.settings.GetInt(key)
			return
		default:
			return
		}
	})
}

// wayland 下专用，更新屏幕宽高属性
func (m *Manager) updateScreenSize() {
	var screenWidth uint16
	var screenHeight uint16

	m.monitorMapMu.Lock()
	for _, monitor := range m.monitorMap {
		if !monitor.Enabled {
			continue
		}
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
