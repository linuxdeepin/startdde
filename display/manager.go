package display

import (
	"fmt"
	"gir/gio-2.0"
	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/randr"
	"github.com/BurntSushi/xgb/xproto"
	"os"
	"pkg.deepin.io/dde/api/drandr"
	"pkg.deepin.io/lib/dbus"
	"pkg.deepin.io/lib/log"
	"pkg.deepin.io/lib/utils"
	"regexp"
	"sort"
	"strings"
	"sync"
)

const (
	DisplayModeCustom uint8 = iota
	DisplayModeMirror
	DisplayModeExtend
	DisplayModeOnlyOne
	DisplayModeUnknow
)

const (
	displaySchemaId  = "com.deepin.dde.display"
	gsKeyDisplayMode = "display-mode"
	gsKeyBrightness  = "brightness"
	gsKeySetter      = "brightness-setter"
	gsKeyMapOutput   = "map-output"
	gsKeyPrimary     = "primary"
	gsKeyCustomMode  = "current-custom-mode"

	customModeDelim = "+"
)

type Manager struct {
	conn           *xgb.Conn
	outputInfos    drandr.OutputInfos
	modeInfos      drandr.ModeInfos
	config         *configManager
	lastConfigTime xproto.Timestamp

	HasChanged      bool
	DisplayMode     uint8
	ScreenWidth     uint16
	ScreenHeight    uint16
	Primary         string
	CurrentCustomId string
	CustomIdList    []string
	PrimaryRect     xproto.Rectangle
	Monitors        MonitorInfos
	// TODO rename Brightness to brightness, prefer to use GetBrightness() to get the brightness info
	Brightness      map[string]float64
	brightnessMutex sync.RWMutex
	TouchMap        map[string]string

	disableList []string

	// TODO: add mutex locker in used
	allMonitors MonitorInfos
	setting     *gio.Settings
	ifcLocker   sync.Mutex
	eventLocker sync.Mutex
}

var (
	_dpy           *Manager
	monitorsLocker sync.Mutex
	logger         = log.NewLogger(dbusDest)
	configFile     = os.Getenv("HOME") + "/.config/deepin/startdde/display.json"
)

func SetLogLevel(level log.Priority) {
	logger.SetLogLevel(level)
}

func newManager() (*Manager, error) {
	conn, err := xgb.NewConn()
	if err != nil {
		return nil, err
	}

	screenInfo, err := drandr.GetScreenInfo(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	s, err := utils.CheckAndNewGSettings(displaySchemaId)
	if err != nil {
		conn.Close()
		return nil, err
	}

	config, err := newConfigManagerFromFile(configFile)
	if err != nil {
		config = &configManager{
			BaseGroup: make(map[string]*configMonitor),
			filename:  configFile,
		}
	}

	sw, sh := screenInfo.GetScreenSize()
	var m = Manager{
		conn:         conn,
		outputInfos:  screenInfo.Outputs.ListConnectionOutputs().ListValidOutputs(),
		modeInfos:    screenInfo.Modes,
		config:       config,
		ScreenWidth:  sw,
		ScreenHeight: sh,
		Brightness:   make(map[string]float64),
		TouchMap:     make(map[string]string),
	}
	m.setting = s
	m.DisplayMode = uint8(m.setting.GetEnum(gsKeyDisplayMode))
	if m.DisplayMode == DisplayModeUnknow {
		m.setPropDisplayMode(DisplayModeExtend)
		m.DisplayMode = DisplayModeExtend
	}
	m.Primary = m.setting.GetString(gsKeyPrimary)
	m.CurrentCustomId = m.setting.GetString(gsKeyCustomMode)
	m.outputInfos, m.disableList = m.filterOutputs(m.outputInfos)
	return &m, nil
}

func (dpy *Manager) init() {
	if len(dpy.outputInfos) == 0 {
		// TODO: wait for output connected
		logger.Warning("No output plugin")
		return
	}

	dpy.disableOutputs()
	dpy.updateMonitors()
	logger.Debug("-----After init:", dpy.Monitors.getMonitorsId())
	if len(dpy.Primary) == 0 || dpy.Monitors.getByName(dpy.Primary) == nil {
		dpy.Primary = dpy.Monitors[0].Name
	}
	dpy.doSetPrimary(dpy.Primary, true)

	// check config version
	dpy.checkConfigVersion()

	dpy.setPropCustomIdList(dpy.getCustomIdList())
	err := dpy.tryApplyConfig()
	if err != nil {
		logger.Error("Try apply settings failed for init:", err)
	}

	dpy.initBrightness()
	dpy.initTouchMap()
	dpy.updateScreenSize()
}

func (dpy *Manager) initTouchMap() {
	value := dpy.setting.GetString(gsKeyMapOutput)
	if len(value) == 0 {
		dpy.TouchMap = make(map[string]string)
		dpy.setPropTouchMap(dpy.TouchMap)
		return
	}

	err := jsonUnmarshal(value, &dpy.TouchMap)
	if err != nil {
		logger.Warningf("[initTouchMap] unmarshal (%s) failed: %v",
			value, err)
		return
	}

	for touch, output := range dpy.TouchMap {
		dpy.doSetTouchMap(touch, output)
	}
	dpy.setPropTouchMap(dpy.TouchMap)
}

func (dpy *Manager) doSetTouchMap(output, touch string) error {
	info := dpy.outputInfos.QueryByName(output)
	if len(info.Name) == 0 {
		return fmt.Errorf("Invalid output name: %s", output)
	}

	// TODO: check touch validity
	return doAction(fmt.Sprintf("xinput --map-to-output %s %s", touch, output))
}

func (dpy *Manager) switchToMirror() error {
	monitorsLocker.Lock()
	defer monitorsLocker.Unlock()
	connected, err := dpy.multiOutputCheck()
	if err != nil {
		return err
	}

	modes := connected.FoundCommonModes()
	if len(modes) == 0 {
		return fmt.Errorf("Not found common mode")
	}

	cmd := "xrandr "
	primary := connected[0].Name
	for _, m := range connected {
		m.cfg.Enabled = true
		m.doEnable(true)
		m.cfg.X = 0
		m.cfg.Y = 0
		m.doSetPosition(0, 0)
		m.cfg.Rotation = 1
		m.doSetRotation(1)
		m.cfg.Reflect = 0
		m.doSetReflect(0)
		mode := m.Modes.QueryBySize(modes[0].Width, modes[0].Height)
		m.cfg.Width = mode.Width
		m.cfg.Height = mode.Height
		m.doSetMode(mode.Id)
		cmd += m.generateCommandline(primary, false)
	}

	err = doAction(cmd)
	if err != nil {
		logger.Errorf("[switchToMirror] apply (%s) failed: %v", cmd, err)
		return err
	}
	return dpy.doSetPrimary(primary, true)
}

func (dpy *Manager) switchToExtend() error {
	monitorsLocker.Lock()
	defer monitorsLocker.Unlock()
	return dpy.doSwitchToExtend()
}

func (dpy *Manager) doSwitchToExtend() error {
	connected := dpy.Monitors.listConnected()
	if len(connected) == 0 {
		return nil
	}

	var (
		startx int16 = 0
		cmd          = "xrandr "
	)
	primary := connected[0].Name
	for _, m := range connected {
		m.cfg.Enabled = true
		m.doEnable(true)
		m.cfg.X = startx
		m.cfg.Y = 0
		m.doSetPosition(startx, 0)
		m.cfg.Rotation = 1
		m.doSetRotation(1)
		m.cfg.Reflect = 0
		m.doSetReflect(0)
		m.cfg.Width = m.BestMode.Width
		m.cfg.Height = m.BestMode.Height
		m.doSetMode(m.BestMode.Id)
		cmd += m.generateCommandline(primary, false)
		startx += int16(m.Width)
	}
	err := doAction(cmd)
	if err != nil {
		logger.Errorf("[switchToExtend] apply (%s) failed: %v", cmd, err)
		return err
	}
	return dpy.doSetPrimary(primary, true)
}

func (dpy *Manager) switchToOnlyOne(name string) error {
	monitorsLocker.Lock()
	defer monitorsLocker.Unlock()
	connected, err := dpy.multiOutputCheck()
	if err != nil {
		return nil
	}

	if m := connected.getByName(name); m == nil {
		return fmt.Errorf("Not found this output")
	}

	cmd := "xrandr "
	for _, m := range connected {
		if m.Name != name {
			m.cfg.Enabled = false
			m.doEnable(false)
		} else {
			m.cfg.Enabled = true
			m.doEnable(true)
			m.cfg.X = 0
			m.cfg.Y = 0
			m.doSetPosition(0, 0)
			m.cfg.Rotation = 1
			m.doSetRotation(1)
			m.cfg.Reflect = 0
			m.doSetReflect(0)
			m.cfg.Width = m.BestMode.Width
			m.cfg.Height = m.BestMode.Height
			m.doSetMode(m.BestMode.Id)
		}
		cmd += m.generateCommandline(name, false)
	}
	err = doAction(cmd)
	if err != nil {
		logger.Errorf("[switchToOnlyOne] apply (%s) failed: %v", cmd, err)
		return err
	}
	return dpy.doSetPrimary(name, true)
}

func (dpy *Manager) switchToCustom(name string) error {
	monitorsLocker.Lock()
	defer monitorsLocker.Unlock()
	// firstly find the matched config,
	// then update monitors from config, finally apply these config.
	id := dpy.Monitors.getMonitorsId()
	logger.Debug("---------[switchToCustom] now id:", id)
	if len(id) == 0 {
		return fmt.Errorf("No output connected")
	}
	id = name + customModeDelim + id
	cMonitor := dpy.config.get(id)
	logger.Debug("----------[switchToCustom] config manager:", dpy.config.String())
	if cMonitor == nil {
		if dpy.DisplayMode != DisplayModeMirror {
			dpy.doSwitchToExtend()
		}
		dpy.config.set(id, &configMonitor{
			Name:      name,
			Primary:   dpy.Primary,
			BaseInfos: dpy.Monitors.getBaseInfos(),
		})
		dpy.setPropCustomIdList(dpy.getCustomIdList())
		dpy.syncCurrentCustomId(name)
		return dpy.config.writeFile()
	}
	err := dpy.applyConfigSettings(cMonitor)
	if err == nil {
		dpy.syncCurrentCustomId(name)
	}
	return err
}

func (dpy *Manager) tryApplyConfig() error {
	outputLen := len(dpy.outputInfos)
	if outputLen != 1 && dpy.DisplayMode != DisplayModeCustom {
		// if multi-output and not custom mode, switch to the special mode
		primary := dpy.Primary
		if dpy.DisplayMode == DisplayModeOnlyOne {
			name := dpy.setting.GetString(gsKeyPrimary)
			if name != "" {
				primary = name
			}
		}
		logger.Info("-----------[tryApplyConfig] will switch to mode:", dpy.DisplayMode, primary)
		return dpy.SwitchMode(dpy.DisplayMode, primary)
	}

	id := dpy.Monitors.getMonitorsId()
	if outputLen != 1 && dpy.DisplayMode == DisplayModeCustom {
		id = dpy.CurrentCustomId + customModeDelim + id
	}
	cMonitor := dpy.config.get(id)
	if cMonitor == nil {
		// no config found, switch to extend mode
		if outputLen == 1 {
			return dpy.switchToExtend()
		}
		return dpy.SwitchMode(DisplayModeExtend, "")
	}
	monitorsLocker.Lock()
	defer monitorsLocker.Unlock()
	err := dpy.applyConfigSettings(cMonitor)
	if err == nil {
		dpy.setPropCustomIdList(dpy.getCustomIdList())
		if cMonitor.Name != "" {
			dpy.syncCurrentCustomId(cMonitor.Name)
		}
	}
	return err
}

func (dpy *Manager) applyConfigSettings(cMonitor *configMonitor) error {
	var corrected bool = false
	logger.Debugf("============[applyConfigSettings] config: %#v", cMonitor.String())
	logger.Debugf("============[applyConfigSettings] monitors: %#v", dpy.Monitors.getMonitorsId())
	for _, info := range cMonitor.BaseInfos {
		m := dpy.Monitors.get(info.UUID)
		if m == nil {
			logger.Errorf("Config has invalid info: %#v", info)
			continue
		}

		// Sometime output name changed after driver updated
		// so modified
		if m.Name != info.Name {
			corrected = true
			if cMonitor.Primary == info.Name {
				cMonitor.Primary = m.Name
			}
			info.Name = m.Name
		}
		dpy.updateMonitorFromBaseInfo(m, info)
	}
	if corrected {
		dpy.config.writeFile()
	}
	err := dpy.doApply(cMonitor.Primary, false)
	if err != nil {
		return err
	}
	dpy.doSetPrimary(cMonitor.Primary, true)
	dpy.rotateInputPointor()
	return nil
}

func (dpy *Manager) doSetPrimary(name string, effectRect bool) error {
	m := dpy.Monitors.canBePrimary(name)
	if m != nil {
		dpy.setPropPrimary(name)
		if effectRect {
			dpy.setPropPrimaryRect(xproto.Rectangle{
				X:      m.X,
				Y:      m.Y,
				Width:  m.Width,
				Height: m.Height,
			})
		}
		return nil
	}

	// try set a primary from monitors
	logger.Error("Invalid output name:", name)
	return fmt.Errorf("Not found the monitor for %s, maybe closed or disconnected",
		name)
}

func (dpy *Manager) trySetPrimary(effectRect bool) error {
	// check the current primary validity
	if m := dpy.Monitors.canBePrimary(dpy.Primary); m != nil {
		logger.Warningf("The current primary '%s' wouldn't be changed", dpy.Primary)
		return nil
	}

	for _, m := range dpy.Monitors {
		if !m.Connected || !m.Enabled {
			continue
		}
		dpy.setPropPrimary(m.Name)
		if effectRect {
			dpy.setPropPrimaryRect(xproto.Rectangle{
				X:      m.X,
				Y:      m.Y,
				Width:  m.Width,
				Height: m.Height,
			})
		}
	}
	logger.Error("Can't find any valid monitor")
	return fmt.Errorf("No valid monitor was found")
}

func (dpy *Manager) updateMonitors() {
	monitorsLocker.Lock()
	defer monitorsLocker.Unlock()
	// Not rebuild monitor object, just update it.
	// If disconnection, mark it.
	for _, m := range dpy.allMonitors {
		err := dpy.updateMonitor(m)
		if err != nil {
			m.setPropConnected(false)
		} else {
			m.setPropConnected(true)
		}
	}

	for _, oinfo := range dpy.outputInfos {
		m, err := dpy.outputToMonitorInfo(oinfo)
		if err != nil {
			logger.Debug("[updateMonitors] Error:", err)
			continue
		}

		err = dbus.InstallOnSession(m)
		if err != nil {
			logger.Errorf("Install dbus for '%#v' failed: %v",
				oinfo, err)
			continue
		}
		logger.Info("[updateMonitors] add monitor:", m.Name, m.X, m.Y, m.Width, m.Height)
		dpy.allMonitors = append(dpy.allMonitors, m)
	}
	dpy.sortMonitors()
	dpy.setPropMonitors(dpy.allMonitors.listConnected())
}

func (dpy *Manager) outputToMonitorInfo(output drandr.OutputInfo) (*MonitorInfo, error) {
	id := dpy.sumOutputUUID(output)
	// check output whether exists
	if m := dpy.allMonitors.get(id); m != nil {
		return nil, fmt.Errorf("Output '%s' has exist in monitors", output.Name)
	}

	base := toMonitorBaseInfo(output, id)
	modes := dpy.getModesByIds(output.Modes)
	var info = MonitorInfo{
		cfg:            &base,
		Name:           base.Name,
		Enabled:        base.Enabled,
		Connected:      true,
		X:              base.X,
		Y:              base.Y,
		Width:          base.Width,
		Height:         base.Height,
		Rotation:       base.Rotation,
		Reflect:        base.Reflect,
		RefreshRate:    base.RefreshRate,
		Rotations:      output.Crtc.Rotations,
		Reflects:       output.Crtc.Reflects,
		CurrentMode:    modes.QueryBySize(base.Width, base.Height),
		Modes:          modes,
		PreferredModes: dpy.getModesByIds(output.PreferredModes),
	}
	if len(info.PreferredModes) != 0 {
		info.BestMode = info.PreferredModes.Max()
	} else {
		info.BestMode = info.Modes.Max()
	}
	info.RefreshRate = info.CurrentMode.Rate
	info.Width, info.Height = parseModeByRotation(info.Width, info.Height, info.Rotation)

	// There should be no error occurs
	// if info.CurrentMode.Width == 0 || info.CurrentMode.Height == 0 {
	// 	info.CurrentMode = info.BestMode
	// }

	return &info, nil
}

func (dpy *Manager) updateMonitor(m *MonitorInfo) error {
	oinfo := dpy.outputInfos.QueryByName(m.Name)
	if oinfo.Id == 0 {
		logger.Warning("Not found output:", m.Name)
		return fmt.Errorf("Invalid output name: %s", m.Name)
	}

	m.locker.Lock()
	defer m.locker.Unlock()
	// EDID maybe changed because some reasons, but the display device not changed.
	id := dpy.sumOutputUUID(oinfo)
	if id != m.cfg.UUID {
		m.cfg.UUID = id
	}
	m.setPropModes(dpy.getModesByIds(oinfo.Modes))
	logger.Debugf("[updateMonitor] id: %s, crtc info: %#v", m.cfg.UUID, oinfo.Crtc)
	if oinfo.Crtc.Id == 0 {
		m.doEnable(false)
	} else {
		m.doEnable(true)
		m.setPropRotations(oinfo.Crtc.Rotations)
		m.setPropReflects(oinfo.Crtc.Reflects)
		m.setPropPreferredModes(dpy.getModesByIds(oinfo.PreferredModes))
		if len(m.PreferredModes) != 0 {
			m.setPropBestMode(m.PreferredModes.Max())
		} else {
			m.setPropBestMode(m.Modes.Max())
		}
		m.doSetRotation(oinfo.Crtc.Rotation)
		m.doSetReflect(oinfo.Crtc.Reflect)
		// change mode should after change rotation
		m.doSetPosition(oinfo.Crtc.X, oinfo.Crtc.Y)
		err := m.doSetMode(oinfo.Crtc.Mode)
		if err != nil {
			logger.Warningf("[updateMonitor] Set mode '%v' failed: %v", oinfo.Crtc.Mode, err)
			w, h := parseModeByRotation(oinfo.Crtc.Width, oinfo.Crtc.Height, m.Rotation)
			m.doSetModeBySize(w, h)
		}
	}
	return nil
}

func (dpy *Manager) updateMonitorFromBaseInfo(m *MonitorInfo, base *MonitorBaseInfo) error {
	m.locker.Lock()
	defer m.locker.Unlock()
	oinfo := dpy.outputInfos.QueryByName(m.Name)
	if oinfo.Id == 0 {
		logger.Warning("Not found output:", m.Name)
		return fmt.Errorf("Invalid output name: %s", m.Name)
	}

	logger.Debugf("Monitor: %s, base: %s", m.Name, base)
	m.cfg = base.Duplicate()
	return nil
}

var numReg = regexp.MustCompile(`-?[0-9]`)

func (dpy *Manager) sumOutputUUID(output drandr.OutputInfo) string {
	if len(output.EDID) < 128 {
		return output.Name
	}

	id, _ := utils.SumStrMd5(string(output.EDID[:128]))
	if id == "" {
		return output.Name
	}
	return numReg.ReplaceAllString(output.Name, "") + id
}

func (dpy *Manager) getModesByIds(ids []uint32) drandr.ModeInfos {
	var modes drandr.ModeInfos
	for _, v := range ids {
		mode := dpy.modeInfos.Query(v)
		if mode.Width == 0 || mode.Height == 0 {
			logger.Warning("[getModesByIds] Invalid mode id:", v)
			continue
		}
		// handle different rate but some width/height mode
		if t := modes.QueryBySize(mode.Width, mode.Height); t.Id != 0 {
			continue
		}
		modes = append(modes, mode)
	}
	return modes
}

func (dpy *Manager) detectHasChanged() {
	// Comment out the two following lines, because of deadlock. But why?
	// monitorsLocker.Lock()
	// defer monitorsLocker.Unlock()
	if len(dpy.outputInfos) != 1 && dpy.DisplayMode != DisplayModeCustom {
		// if multi-output and not custom mode, nothing
		dpy.setPropHasChanged(false)
		return
	}

	for _, m := range dpy.Monitors {
		if !dpy.isMonitorChanged(m) {
			continue
		}
		dpy.setPropHasChanged(true)
		break
	}
}

func (dpy *Manager) rotateInputPointor() {
	connected := dpy.Monitors
	if len(connected) == 0 {
		return
	}

	if !connected.isRotation() {
		return
	}
	rotateInputPointor(connected[0].Rotation)
}

func (dpy *Manager) multiOutputCheck() (MonitorInfos, error) {
	connected := dpy.Monitors
	if len(connected) == 0 {
		return nil, fmt.Errorf("No connected output found")
	}

	if len(connected) < 2 {
		return nil, fmt.Errorf("Only one output")
	}
	return connected, nil
}

func (dpy *Manager) isMonitorChanged(m *MonitorInfo) bool {
	// m.locker.Lock()
	// defer m.locker.Unlock()
	if !m.Connected {
		return false
	}

	oinfo := dpy.outputInfos.QueryByName(m.Name)
	return (oinfo.Connection != m.Enabled) || (oinfo.Crtc.Mode != m.CurrentMode.Id) ||
		(oinfo.Crtc.X != m.X) || (oinfo.Crtc.Y != m.Y) ||
		(oinfo.Crtc.Rotation != m.Rotation) || (oinfo.Crtc.Reflect != m.Reflect)
}

func (dpy *Manager) fixOutputNotClosed(outputId randr.Output) {
	if len(dpy.outputInfos) == 0 {
		return
	}
	for _, info := range dpy.outputInfos {
		if info.Id == uint32(outputId) {
			return
		}
	}
	dpy.doApply(dpy.Primary, true)
}

func (dpy *Manager) doApply(primary string, auto bool) error {
	return doAction(dpy.Monitors.genCommandline(primary, auto))
}

func (dpy *Manager) updateScreenSize() {
	monitorsLocker.Lock()
	defer monitorsLocker.Unlock()

	w, h := uint16(0), uint16(0)
	for _, monitor := range dpy.Monitors {
		if !monitor.Connected || !monitor.Enabled {
			continue
		}

		t1 := uint16(monitor.X) + monitor.Width
		t2 := uint16(monitor.Y) + monitor.Height
		if w < t1 {
			w = t1
		}
		if h < t2 {
			h = t2

		}
	}
	dpy.setPropScreenSize(w, h)
}

func (dpy *Manager) isIdDeletable(id string) bool {
	if dpy.DisplayMode != DisplayModeCustom {
		return true
	}
	return dpy.CurrentCustomId != id
}

func (dpy *Manager) syncCurrentCustomId(id string) {
	dpy.setPropCurrentCustomId(id)
	dpy.setting.SetString(gsKeyCustomMode, id)
}

func (dpy *Manager) getCustomIdList() []string {
	id := dpy.Monitors.getMonitorsId()
	set := dpy.config.getIdList()
	logger.Debug("~~~~~~~~~[getCustomIdList] id:", id)
	logger.Debug("---------[getCustomIdList] set:", set)
	var tmp []string
	for k, v := range set {
		if v == "" {
			continue
		}
		list := strings.Split(k, customModeDelim)
		if list[len(list)-1] == id {
			tmp = append(tmp, v)
		}
	}
	sort.Strings(tmp)
	return tmp
}
