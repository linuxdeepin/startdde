/**
 * Copyright (C) 2014 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package display

import (
	"fmt"
	"strings"
	"sync"

	"dbus/com/deepin/daemon/helper/backlight"
	"gir/gio-2.0"
	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/randr"
	"github.com/BurntSushi/xgb/xproto"
	"pkg.deepin.io/lib/dbus"
	"pkg.deepin.io/lib/log"
)

const (
	displaySchema         = "com.deepin.dde.display"
	gsKeyBrightnessSetter = "brightness-setter"
	gsKeyDisplayMode      = "display-mode"
	gsKeyBrightness       = "brightness"

	brightnessSetterAuto              = "auto"
	brightnessSetterGamma             = "gamma"
	brightnessSetterBacklight         = "backlight"
	brightnessSetterBacklightRaw      = "backlight-raw"
	brightnessSetterBacklightPlatform = "backlight-platform"
	brightnessSetterBacklightFirmware = "backlight-firmware"
)

var (
	xcon, _ = xgb.NewConn()
	_       = initX11()

	Root           xproto.Window
	ScreenWidthMm  uint16
	ScreenHeightMm uint16

	LastConfigTimeStamp xproto.Timestamp

	MinWidth, MinHeight, MaxWidth, MaxHeight uint16

	logger = log.NewLogger("daemon/display")
)

func initX11() bool {
	randr.Init(xcon)
	sinfo := xproto.Setup(xcon).DefaultScreen(xcon)
	Root = sinfo.Root
	ScreenWidthMm = sinfo.WidthInMillimeters
	ScreenHeightMm = sinfo.HeightInMillimeters
	LastConfigTimeStamp = xproto.Timestamp(0)

	ver, err := randr.QueryVersion(xcon, 1, 3).Reply()
	if err != nil {
		logger.Error("randr.QueryVersion error:", err)
		return false
	}
	if ver.MajorVersion != 1 || ver.MinorVersion != 3 {
		logger.Error("randr version is too low:", ver.MajorVersion, ver.MinorVersion, "this program require at least randr 1.3")
		return false
	}
	if err != nil {
		logger.Error("randr.GetSceenSizeRange failed :", err)
		return false
	}
	return true
}

var GetDisplay = func() func() *Display {
	dpy := &Display{}

	sinfo := xproto.Setup(xcon).DefaultScreen(xcon)
	dpy.setPropScreenWidth(sinfo.WidthInPixels)
	dpy.setPropScreenHeight(sinfo.HeightInPixels)
	dpy.setting = gio.NewSettings(displaySchema)
	dpy.setPropDisplayMode(int16(dpy.setting.GetEnum(gsKeyDisplayMode)))
	dpy.initBrightnessManager()
	GetDisplayInfo().update()
	dpy.setPropHasChanged(false)
	dpy.cfg = LoadConfigDisplay(dpy)

	randr.SelectInputChecked(xcon, Root, randr.NotifyMaskOutputChange|randr.NotifyMaskOutputProperty|randr.NotifyMaskCrtcChange|randr.NotifyMaskScreenChange)

	var err error
	dpy.blHelper, err = backlight.NewBacklight(backlightDest,
		backlightPath)
	if err != nil {
		logger.Warning("New backlight failed:", err)
		dpy.blHelper = nil
	}

	return func() *Display {
		return dpy
	}
}()

type Display struct {
	Monitors    []*Monitor
	monitorLock sync.RWMutex

	ScreenWidth  uint16
	ScreenHeight uint16

	//used by deepin-dock/launcher/desktop
	Primary        string
	PrimaryRect    xproto.Rectangle
	PrimaryChanged func(xproto.Rectangle)

	DisplayMode   int16
	BuiltinOutput *Monitor

	HasChanged bool

	Brightness map[string]float64

	brightnessManager *brightnessMapManager

	cfg *ConfigDisplay

	setting  *gio.Settings
	blHelper *backlight.Backlight

	eventLocker sync.Mutex
	resetLocker sync.Mutex
}

func (dpy *Display) lockMonitors() {
	dpy.monitorLock.Lock()
}
func (dpy *Display) unlockMonitors() {
	dpy.monitorLock.Unlock()
}
func (dpy *Display) rLockMonitors() {
	dpy.monitorLock.RLock()
}
func (dpy *Display) rUnlockMonitors() {
	dpy.monitorLock.RUnlock()
}

//plugging out an output wouldn't always rearrange screen allocation.
func (dpy *Display) fixOutputNotClosed(op randr.Output) {
	outputs := GetDisplayInfo().ListOutputs()
	if len(outputs) == 0 {
		return
	}
	for _, present := range outputs {
		if op == present {
			return
		}
	}

	dpy.apply(true)
}

func (dpy *Display) listener() {
	for {
		e, err := xcon.WaitForEvent()
		if err != nil {
			continue
		}
		dpy.eventLocker.Lock()
		switch ee := e.(type) {
		case randr.NotifyEvent:
			switch ee.SubCode {
			case randr.NotifyCrtcChange:
			case randr.NotifyOutputChange:
				info := ee.U.Oc
				if info.Connection != randr.ConnectionConnected && info.Mode != 0 {
					randr.SetCrtcConfig(xcon, info.Crtc, xproto.TimeCurrentTime, LastConfigTimeStamp, 0, 0, 0, randr.RotationRotate0, nil)
				}
				if info.Mode == 0 || info.Crtc == 0 {
					dpy.fixOutputNotClosed(info.Output)
				}
			case randr.NotifyOutputProperty:
			}
		case randr.ScreenChangeNotifyEvent:
			dpy.setPropScreenWidth(ee.Width)
			dpy.setPropScreenHeight(ee.Height)
			GetDisplayInfo().update()

			curPlan := dpy.QueryCurrentPlanName()
			logger.Debug("[listener] Screen event:", ee.Width, ee.Height, LastConfigTimeStamp, ee.ConfigTimestamp)
			logger.Debugf("[Listener] current display config: %#v\n", dpy.cfg)
			if LastConfigTimeStamp < ee.ConfigTimestamp {
				LastConfigTimeStamp = ee.ConfigTimestamp
				if dpy.cfg == nil || dpy.cfg.CurrentPlanName != curPlan {
					logger.Info("Detect New ConfigTimestmap, try reset changes, current plan:", curPlan)
					if dpy.cfg != nil && len(curPlan) == 0 {
						dpy.cfg.CurrentPlanName = curPlan
					} else {
						dpy.ResetChanges()
						dpy.SwitchMode(dpy.DisplayMode, dpy.cfg.Plans[dpy.cfg.CurrentPlanName].DefaultOutput)
					}
				}
			}

			if len(curPlan) != 0 {
				//sync Monitor's state
				for _, m := range dpy.Monitors {
					m.updateInfo()
				}
				//changePrimary will try set an valid primary if dpy.Primary invalid
				dpy.changePrimary(dpy.Primary, true)
				dpy.mapTouchScreen()
			}
		}
		dpy.eventLocker.Unlock()
	}
}

func (dpy *Display) mapTouchScreen() {
	for output, touchscreen := range dpy.cfg.MapToTouchScreen {
		runCodeAsync(fmt.Sprintf(`xinput map-to-output "%s" "%s"`, touchscreen, output))
	}
}

func (dpy *Display) AssociateTouchScreen(output string, touchscreen string) {
	//TODO: check name valid
	dpy.cfg.MapToTouchScreen[output] = touchscreen
	if dpy.DisplayMode != DisplayModeCustom {
		dpy.cfg.Save()
	}
}

func (dpy *Display) getOutputBrightness(output string) float64 {
	isSupported := dpy.supportedBacklight(xcon, GetDisplayInfo().QueryOutputs(output))
	setter := dpy.setting.GetString(gsKeyBrightnessSetter)
	if (setter == brightnessSetterGamma) ||
		(setter == brightnessSetterAuto && !isSupported) {
		return dpy.Brightness[output]
	}

	return dpy.getBacklight(setter)
}

func (dpy *Display) RefreshBrightness() {
	for output, _ := range dpy.Brightness {
		dpy.setPropBrightness(output, dpy.getOutputBrightness(output))
	}
}

//The range of brightness value is 0.1~1.
//Generally speaking user can use media key to change brightness when the output
//supports backlight, but we can't rely on this assumption.
//If xrandr/acpi driver works, the value of zero is safety. But if the driver
//doesn't work well, ChangeBrightness has received a zero value and then the system
//will enter an unusable situation.
func (dpy *Display) ChangeBrightness(output string, v float64) error {
	if !validBrightnessValue(v) {
		//NOTO: don't use "if v < 0.1 || v > 1",  because there has some guy called NaN.
		return fmt.Errorf("Try change the brightness of %s to an invalid value(%v)", output, v)
	}

	op := GetDisplayInfo().QueryOutputs(output)
	if op == 0 {
		return fmt.Errorf("Invalid output: %v", output)
	}

	now := dpy.Brightness[output]
	if v > now-0.01 && v < now+0.01 {
		return nil
	}

	isSupported := dpy.supportedBacklight(xcon, op)
	setter := dpy.setting.GetString(gsKeyBrightnessSetter)
	if (setter == brightnessSetterGamma) ||
		(setter == brightnessSetterAuto && !isSupported) {
		err := setBrightness(xcon, op, v)
		if err != nil {
			logger.Warningf("[ChangeBrightness] query output '%v' failed, try backlight", output)
			// TODO: check whether successfully by query backlight brightness
			//dpy.setBacklight(output, brightnessSetterBacklight, v)
			return err
		}
	} else {
		dpy.setBacklight(output, setter, v)
	}

	dpy.setPropBrightness(output, v)
	return nil
}

func (dpy *Display) ResetBrightness(output string) {
	dpy.resetLocker.Lock()
	defer dpy.resetLocker.Unlock()
	dpy.brightnessManager.reset()
	for output, _ := range dpy.Brightness {
		dpy.SetBrightness(output, 1)
	}
}
func (dpy *Display) SetBrightness(output string, v float64) error {
	if err := dpy.ChangeBrightness(output, v); err != nil {
		return err
	}

	dpy.brightnessManager.set(output, v)
	return nil
}

func (dpy *Display) JoinMonitor(a string, b string) error {
	dpy.lockMonitors()
	defer dpy.unlockMonitors()

	mgroup, ok := dpy.cfg.Plans[dpy.cfg.CurrentPlanName]
	if !ok {
		return fmt.Errorf("Current plan invalid: %q", dpy.cfg.CurrentPlanName)
	}

	ms := mgroup.Monitors
	mm, ok := ms[a+joinSeparator+b]
	if !ok {
		mm, ok = ms[b+joinSeparator+a]
	}
	if ok {
		dpy.setPropMonitors([]*Monitor{NewMonitor(dpy, mm)})
		return nil
	}

	if ma, ok := ms[a]; ok {
		if mb, ok := ms[b]; ok {
			mc := mergeConfigMonitor(dpy, ma, mb)
			delete(dpy.cfg.Plans[dpy.cfg.CurrentPlanName].Monitors, a)
			delete(dpy.cfg.Plans[dpy.cfg.CurrentPlanName].Monitors, b)
			dpy.cfg.Plans[dpy.cfg.CurrentPlanName].Monitors[mc.Name] = mc

			var newMonitors []*Monitor
			for _, m := range dpy.Monitors {
				if m.Name != a && m.Name != b {
					newMonitors = append(newMonitors, m)
				}
			}
			newMonitors = append(newMonitors, NewMonitor(dpy, mc))
			dpy.setPropMonitors(newMonitors)
		} else {
			return fmt.Errorf("Can't find Monitor %s\n", b)
		}
	} else {
		return fmt.Errorf("Can't find Monitor %s\n", a)
	}
	return nil
}
func (dpy *Display) SplitMonitor(a string) error {
	dpy.lockMonitors()
	defer dpy.unlockMonitors()

	if len(GetDisplayInfo().ListOutputs()) == 0 {
		return fmt.Errorf("No output be found")
	}

	var monitors []*Monitor
	found := false
	for _, m := range dpy.Monitors {
		if m.Name == a {
			submonitors := m.split(dpy)
			if submonitors == nil {
				return fmt.Errorf("Can't find composited monitor: %s", a)
			}
			found = true
			monitors = append(monitors, submonitors...)
		} else {
			monitors = append(monitors, m)
		}
	}
	if found {
		dpy.setPropMonitors(monitors)
		return nil
	} else {
		return fmt.Errorf("Can't find composited monitor: %s", a)
	}
}
func (m *Monitor) split(dpy *Display) (r []*Monitor) {
	if !strings.Contains(m.Name, joinSeparator) {
		return
	}

	delete(dpy.cfg.Plans[dpy.QueryCurrentPlanName()].Monitors, m.Name)
	dpyinfo := GetDisplayInfo()
	for _, name := range strings.Split(m.Name, joinSeparator) {
		op := dpyinfo.QueryOutputs(name)
		if op == 0 {
			continue
		}
		mcfg, err := CreateConfigMonitor(dpy, op)
		if err != nil {
			logger.Error("Failed createconfigmonitor at split", err, name, mcfg)
			continue
		}
		dpy.cfg.Plans[dpy.QueryCurrentPlanName()].Monitors[name] = mcfg

		//TODO: check width/height value whether zero

		dpy.cfg.ensureValid(dpy)
		m := NewMonitor(dpy, mcfg)
		//TODO: change or set?
		m.SetMode((m.BestMode.ID))
		r = append(r, m)
	}
	return
}

func (dpy *Display) detectChanged() {
	if dpy.disableChanged() {
		dpy.setPropHasChanged(false)
		return
	}
	cfg := LoadConfigDisplay(dpy)
	if !cfg.ensureValid(dpy) {
		return
	}
	dpy.setPropHasChanged(!dpy.cfg.Compare(cfg))
}

func (dpy *Display) canBePrimary(name string) *Monitor {
	for _, m := range dpy.Monitors {
		if m.Name == name && m.Opened {
			return m
		}
	}
	return nil
}

func (dpy *Display) changePrimary(name string, effectRect bool) error {
	if m := dpy.canBePrimary(name); m != nil {
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
	//the output whose name is `name` didn't exists or disabled,

	if dpy.canBePrimary(dpy.Primary) != nil {
		return fmt.Errorf("can't set %s as primary, current primary %s wouldn't be changed", name, dpy.Primary)
	}

	//try set an primary
	for _, m := range dpy.Monitors {
		if dpy.canBePrimary(m.Name) != nil {
			dpy.setPropPrimary(m.Name)
			if effectRect {
				dpy.setPropPrimaryRect(xproto.Rectangle{
					X:      m.X,
					Y:      m.Y,
					Width:  m.Width,
					Height: m.Height,
				})
			}
			return fmt.Errorf("can't set %s as primary, and current parimary %s is invalid. fallback to %s",
				name, dpy.Primary, m.Name)
		}
	}
	logger.Error("can't find any valid primary", name)
	return fmt.Errorf("can't find any valid primary: %v", name)
}

func (dpy *Display) SetPrimary(name string) error {
	if err := dpy.changePrimary(name, true); err != nil {
		logger.Warning("Set primary failed:", err)
		return err
	}
	dpy.cfg.Plans[dpy.cfg.CurrentPlanName].DefaultOutput = name
	// if custom mode, must call 'SaveChanges' to save config
	if dpy.DisplayMode != DisplayModeCustom {
		dpy.cfg.Save()
	}
	return nil
}

func (dpy *Display) disableChanged() bool {
	if len(dpy.Monitors) == 1 && !dpy.Monitors[0].IsComposited {
		return false
	}
	if dpy.DisplayMode == DisplayModeCustom {
		return false
	}
	return true
}

func (dpy *Display) Apply() {
	if len(GetDisplayInfo().ListOutputs()) == 0 {
		return
	}

	logger.Debug("[Apply] start, hasChanged:", dpy.HasChanged)
	if dpy.disableChanged() {
		logger.Warning("Display.Apply only can be used in Custom DisplayMode.")
		return
	}
	dpy.apply(false)
	logger.Debug("[Apply] done, hasChanged:", dpy.HasChanged)
}

func (dpy *Display) apply(auto bool) {
	dpy.rLockMonitors()
	defer dpy.rUnlockMonitors()

	code := "xrandr "
	for _, m := range dpy.Monitors {
		code += m.generateShell()
		if auto {
			code += " --auto"
		}

		if dpy.cfg.Plans[dpy.cfg.CurrentPlanName].DefaultOutput == m.Name {
			code += " --primary"
		}
	}
	runCode(code)
}

func (dpy *Display) ResetChanges() {
	logger.Debugf("[ResetChanges] start, hasChanged: %v, primary: %s", dpy.HasChanged, dpy.Primary)
	dpy.resetLocker.Lock()
	defer dpy.resetLocker.Unlock()

	curPlan := dpy.QueryCurrentPlanName()
	if len(curPlan) == 0 {
		logger.Warning("[ResetChanges] no output be found")
		return
	}

	dpy.cfg = LoadConfigDisplay(dpy)
	dpy.cfg.attachCurrentMonitor(dpy)
	if !dpy.cfg.ensureValid(dpy) {
		logger.Infof("-------Invalid plan: %s, %#v\n", curPlan, dpy.cfg.Plans[curPlan])
		delete(dpy.cfg.Plans, curPlan)
		dpy.cfg.attachCurrentMonitor(dpy)
		if !dpy.cfg.ensureValid(dpy) {
			return
		}
	}
	dpy.cfg.Save()
	//must be invoked after LoadConfigDisplay(dpy)
	dpy.rebuildMonitors()
	if len(dpy.Monitors) == 0 {
		logger.Error("[ResetChanges] monitor is empty")
		return
	}

	if err := dpy.changePrimary(dpy.cfg.Plans[dpy.cfg.CurrentPlanName].DefaultOutput, true); err != nil {
		logger.Warning("chnagePrimary :", dpy.cfg.Plans[dpy.cfg.CurrentPlanName], err)
		runCode("xrandr --auto")
	}

	//apply the saved configurations.
	dpy.apply(false)
	dpy.setPropHasChanged(false)
	dpy.Brightness = make(map[string]float64)

	for name, v := range dpy.brightnessManager.core {
		logger.Debug("Reset brightness:", name, v)
		dpy.ChangeBrightness(name, v)
	}

	//dpy.brightnessManager may doesn't contain all output, so we must
	//reset this output's brightness to 1
	for _, mcfg := range dpy.cfg.Plans[dpy.cfg.CurrentPlanName].Monitors {
		if _, err := dpy.brightnessManager.get(mcfg.Name); err != nil {
			dpy.SetBrightness(mcfg.Name, 1)
		}
	}
	logger.Debug("[ResetChanges] done, hasChanged:", dpy.HasChanged)
}

func (dpy *Display) SaveChanges() {
	dpy.cfg.Save()
	dpy.detectChanged()
}

func (dpy *Display) Reset() {
	dpy.resetLocker.Lock()
	defer dpy.resetLocker.Unlock()
	dpy.rLockMonitors()
	defer dpy.rUnlockMonitors()

	if len(GetDisplayInfo().ListOutputs()) == 0 {
		return
	}

	for _, m := range dpy.Monitors {
		dpy.SetBrightness(m.Name, 1)
		m.SetReflect(0)
		m.SetRotation(1)
		m.SetMode(m.BestMode.ID)
	}
	for _, m := range dpy.Monitors {
		for _, output := range m.Outputs {
			dpy.SetBrightness(output, 1)
		}
	}
	dpy.apply(true)
	dpy.SaveChanges()
}

func (dpy *Display) syncDisplayMode(mode int16) {
	if mode != int16(dpy.setting.GetEnum(gsKeyDisplayMode)) {
		dpy.setting.SetEnum(gsKeyDisplayMode, int32(mode))
	}
	if dpy.cfg != nil && dpy.cfg.DisplayMode != mode {
		dpy.cfg.DisplayMode = mode
	}
}

func Start() {
	dpy := GetDisplay()
	if len(GetDisplayInfo().ListNames()) == 0 {
		logger.Debug("[Display Start] no output found, will join loop unless output connected")
		fixEmptyOutput()
	}
	err := dbus.InstallOnSession(dpy)
	if err != nil {
		logger.Error("Can't install dbus display service on session:", err)
		return
	}
	dpy.ResetChanges()
	dpy.SwitchMode(dpy.cfg.DisplayMode, dpy.cfg.Plans[dpy.cfg.CurrentPlanName].DefaultOutput)

	go dpy.listener()

	for _, m := range dpy.Monitors {
		m.updateInfo()
	}
	logger.Debugf("[Start] start finished: %#v\n", dpy.cfg)
}

func (dpy *Display) QueryOutputFeature(name string) int32 {
	support := dpy.supportedBacklight(xcon, GetDisplayInfo().QueryOutputs(name))
	if support {
		return 1
	} else {
		return 0
	}
}

func (dpy *Display) rebuildMonitors() {
	var monitors []*Monitor
	group := dpy.cfg.Plans[dpy.cfg.CurrentPlanName]
	for _, mcfg := range group.Monitors {
		m := NewMonitor(dpy, mcfg)
		if m == nil {
			continue
		}
		err := m.updateInfo()
		logger.Debugf("[NewMonitor] after update: %#v, error: %v\n", m, err)
		if err != nil {
			m.setPropOpened(false)
		} else {
			// fixed Opened == false because crtc == 0 at monitor plugin
			if group.DefaultOutput == m.Name && !m.Opened {
				m.setPropOpened(true)
			}
		}
		logger.Debugf("[NewMonitor] after fixed: %#v\n", m)
		monitors = append(monitors, m)
	}
	dpy.setPropMonitors(monitors)
}
