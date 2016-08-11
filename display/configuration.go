/**
 * Copyright (C) 2014 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package display

import "github.com/BurntSushi/xgb/randr"
import "encoding/json"
import "fmt"
import "os"
import "sync"
import "strings"
import "sort"

var hasCFG = false

type monitorGroup struct {
	DefaultOutput string
	Monitors      map[string]*ConfigMonitor
}

type ConfigDisplay struct {
	DisplayMode     int16
	CurrentPlanName string
	Plans           map[string]*monitorGroup

	Brightness       map[string]float64
	MapToTouchScreen map[string]string
}

var (
	homeDir     = os.Getenv("HOME")
	_ConfigPath = homeDir + "/.config/deepin_monitors.json"
	configLock  sync.RWMutex
)

func (dpy *Display) ListOutputNames() []string {
	names := GetDisplayInfo().ListNames()
	sort.Strings(names)
	return names
}

func (dpy *Display) QueryCurrentPlanName() string {
	names := GetDisplayInfo().ListNames()
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	return strings.Join(names, ",") + fmt.Sprintf(",mode%v", dpy.DisplayMode)
	//return base64.NewEncoding("1").EncodeToString([]byte(strings.Join(names, ",")))
}

func (cfg *ConfigDisplay) attachCurrentMonitor(dpy *Display) (added bool) {
	cfg.CurrentPlanName = dpy.QueryCurrentPlanName()
	if _, ok := cfg.Plans[cfg.CurrentPlanName]; ok {
		return false
	}
	logger.Info("attachCurrentMonitor: build info", cfg.CurrentPlanName)
	hasCFG = false

	//grab and build monitors information
	monitors := &monitorGroup{
		DefaultOutput: dpy.Primary,
		Monitors:      make(map[string]*ConfigMonitor),
	}
	for _, op := range GetDisplayInfo().ListOutputs() {
		mcfg, err := CreateConfigMonitor(dpy, op)
		if err != nil {
			logger.Warning("skip invalid monitor", op)
			continue
		}
		monitors.Monitors[mcfg.Name] = mcfg
	}

	//save it at CurrentPlanName slot
	cfg.Plans[cfg.CurrentPlanName] = monitors

	for _, name := range GetDisplayInfo().ListNames() {
		if dpy.supportedBacklight(xcon, GetDisplayInfo().QueryOutputs(name)) {
			cfg.Brightness[name] = dpy.getBacklight(dpy.setting.GetString(gsKeyBrightnessSetter))
		} else {
			cfg.Brightness[name] = 1
		}
	}
	return true
}

func createConfigDisplay(dpy *Display) *ConfigDisplay {
	cfg := &ConfigDisplay{}
	cfg.Plans = make(map[string]*monitorGroup)
	cfg.Brightness = make(map[string]float64)
	cfg.MapToTouchScreen = make(map[string]string)
	cfg.DisplayMode = DisplayModeExtend

	cfg.attachCurrentMonitor(dpy)
	cfg.ensureValid(dpy)
	return cfg
}

func (cfg *ConfigDisplay) updateMonitorPlan(dpy *Display) {
}

func (cfg *ConfigDisplay) ensureValid(dpy *Display) bool {
	var opend []*ConfigMonitor
	var any *ConfigMonitor
	GetDisplayInfo().update()

	logger.Debug("[ensureValid] current plan:", cfg.CurrentPlanName, cfg)
	for _, m := range cfg.Plans[cfg.CurrentPlanName].Monitors {
		any = m
		if m.Enabled {
			opend = append(opend, m)
		}

		//1.1. ensure the output support the mode which be matched with the width/height
		valid := false
		for _, opName := range m.Outputs {
			op := GetDisplayInfo().QueryOutputs(opName)
			oinfo, err := randr.GetOutputInfo(xcon, op, LastConfigTimeStamp).Reply()
			if err != nil {
				logger.Error("ensureValid failed:", opName, "OP:", op, err)
				continue
			}
			if len(oinfo.Modes) == 0 {
				logger.Error("ensureValid failed:", opName, "hasn't any mode info")
				continue
			}

			for _, id := range oinfo.Modes {
				minfo := GetDisplayInfo().QueryModes(id)
				if minfo.Width == m.Width && minfo.Height == m.Height {
					valid = true
					break
				}
			}
		}
		if !valid {
		}
	}
	if any == nil {
		logger.Error("Can't find any ConfigMonitor at ", cfg.CurrentPlanName)
		return false
	}
	//1. ensure there has a opened monitor.
	if len(opend) == 0 {
		any.Enabled = true
		opend = append(opend, any)
	}

	//2. ensure primary is opened
	primaryOk := false
	for _, m := range opend {
		if cfg.Plans[cfg.CurrentPlanName].DefaultOutput == m.Name {
			primaryOk = true
			break
		}
	}
	if !primaryOk {
		cfg.Plans[cfg.CurrentPlanName].DefaultOutput = any.Name
	}

	//4. avoid monitor allocation overlay
	valid := true
	for _, m1 := range cfg.Plans[cfg.CurrentPlanName].Monitors {
		for _, m2 := range cfg.Plans[cfg.CurrentPlanName].Monitors {
			if m1 != m2 {
				if isOverlap(m1.X, m1.Y, m1.Width, m1.Height, m2.X, m2.Y, m2.Width, m2.Height) {
					logger.Debugf("%s(%d,%d,%d,%d) is ovlerlap with %s(%d,%d,%d,%d)! **rearrange all monitor**\n",
						m1.Name, m1.X, m1.Y, m1.Width, m1.Height, m2.Name, m2.X, m2.Y, m2.Width, m2.Height)
					valid = false
					break
				}
			}
		}
	}
	if !valid {
		curGroup := cfg.Plans[cfg.CurrentPlanName]
		pm := curGroup.Monitors[curGroup.DefaultOutput]
		cx, cy, pw, ph := int16(0), int16(0), pm.Width, pm.Height
		pm.X, pm.Y = 0, 0
		logger.Debugf("Rearrange %s to (%d,%d,%d,%d)\n", pm.Name, pm.X, pm.Y, pm.Width, pm.Height)
		for _, m := range cfg.Plans[cfg.CurrentPlanName].Monitors {
			if m != pm {
				cx += int16(pw)
				cy += int16(ph)
				m.X, m.Y = cx, 0
				logger.Debugf("Rearrange %s to (%d,%d,%d,%d)\n", m.Name, m.X, m.Y, m.Width, m.Height)
			}
		}
	}
	return true
}

func validBrightnessValue(v float64) bool {
	if v < 0 || v > 1 {
		return false
	}
	return true
}

func validConfig(r *ConfigDisplay) bool {
	for _, v := range r.Brightness {
		if !validBrightnessValue(v) {
			return false
		}
	}
	return true
}

func LoadConfigDisplay(dpy *Display) *ConfigDisplay {
	configLock.RLock()
	defer configLock.RUnlock()

	cfg, err := loadConfigFromFile(dpy, _ConfigPath)
	if err != nil {
		logger.Warningf("Load config '%s' failed: %v", _ConfigPath, err)
		hasCFG = false
		cfg = createConfigDisplay(dpy)
		cfg.attachCurrentMonitor(dpy)
		dpy.syncDisplayMode(cfg.DisplayMode)
	} else {
		hasCFG = true
	}

	return cfg
}

func (c *ConfigDisplay) Compare(cfg *ConfigDisplay) bool {
	if c.CurrentPlanName != cfg.CurrentPlanName {
		logger.Errorf("Compare tow ConfigDisply which hasn't same CurrentPlaneName %q != %q",
			c.CurrentPlanName, cfg.CurrentPlanName)
		return false
	}

	if c.Plans[c.CurrentPlanName].DefaultOutput != cfg.Plans[cfg.CurrentPlanName].DefaultOutput {
		return false
	}

	for _, m1 := range c.Plans[c.CurrentPlanName].Monitors {
		if m2, ok := cfg.Plans[c.CurrentPlanName].Monitors[m1.Name]; ok {
			if m1.Compare(m2) == false {
				return false
			}
		}
	}

	return true
}
func (c *ConfigDisplay) Save() {
	configLock.Lock()
	defer configLock.Unlock()

	fw, err := os.Create(_ConfigPath)
	if err != nil {
		logger.Error("Cant create configure:", err)
		return
	}
	defer fw.Close()

	err = json.NewEncoder(fw).Encode(c)
	if err != nil {
		logger.Warningf("Save config '%s' failed: %v", _ConfigPath, err)
		return
	}
	hasCFG = true
}

type ConfigMonitor struct {
	Name    string
	Outputs []string

	Width, Height uint16
	RefreshRate   float64

	X, Y int16

	Enabled  bool
	Rotation uint16
	Reflect  uint16
}

func mergeConfigMonitor(dpy *Display, a *ConfigMonitor, b *ConfigMonitor) *ConfigMonitor {
	c := &ConfigMonitor{}
	c.Outputs = append(a.Outputs, b.Outputs...)
	c.Name = a.Name + joinSeparator + b.Name
	c.Reflect = 0
	c.Rotation = 1
	c.X, c.Y = 0, 0

	var ops []randr.Output
	for _, opName := range c.Outputs {
		op := GetDisplayInfo().QueryOutputs(opName)
		if op != 0 {
			ops = append(ops, op)
		}
	}
	c.Width, c.Height = getMatchedSize(ops)
	c.Enabled = true
	return c
}

func CreateConfigMonitor(dpy *Display, op randr.Output) (*ConfigMonitor, error) {
	cfg := &ConfigMonitor{}
	oinfo, err := randr.GetOutputInfo(xcon, op, LastConfigTimeStamp).Reply()
	if err != nil {
		return nil, err
	}
	if len(oinfo.Modes) == 0 {
		return nil, fmt.Errorf("can't find any modeinfo")
	}
	cfg.Name = string(oinfo.Name)
	cfg.Outputs = append(cfg.Outputs, cfg.Name)

	logger.Debugf("[CreateConfigMonitor] output '%v' info: %#v\n", op, oinfo)
	if oinfo.Crtc != 0 && oinfo.Connection == randr.ConnectionConnected {
		cinfo, err := randr.GetCrtcInfo(xcon, oinfo.Crtc, LastConfigTimeStamp).Reply()
		if err != nil {
			return nil, err
		}
		cfg.Width, cfg.Height = cinfo.Width, cinfo.Height

		cfg.Rotation, cfg.Reflect = parseRandR(cinfo.Rotation)

		cfg.Enabled = true
	} else {
		if len(oinfo.Modes) == 0 {
			return nil, fmt.Errorf("%s hasn't any mode info", string(oinfo.Name))
		}
		minfo := GetDisplayInfo().QueryModes(oinfo.Modes[0])
		cfg.Width, cfg.Height = minfo.Width, minfo.Height
		cfg.Rotation, cfg.Reflect = 1, 0
		//try opening all outputs if there hasn't configuration currently.
		if !hasCFG {
			logger.Debug("[CreateConfigMonitor] no config exist")
			cfg.Enabled = true
		} else {
			cfg.Enabled = false
		}
	}

	logger.Debugf("[CreateConfigMonitor] create finish: %#v\n", cfg)
	return cfg, nil
}

func (c *ConfigMonitor) Save() {
	cfg := LoadConfigDisplay(GetDisplay())
	configLock.Lock()
	defer configLock.Unlock()

	for i, m := range cfg.Plans[cfg.CurrentPlanName].Monitors {
		if m.Name == c.Name {
			cfg.Plans[cfg.CurrentPlanName].Monitors[i] = c
			cfg.Save()
			return
		}
	}
	logger.Error("not reached")
}

func (m1 *ConfigMonitor) Compare(m2 *ConfigMonitor) bool {
	if m1.Enabled != m2.Enabled {
		return false
	}
	if m1.Width != m2.Width || m1.Height != m2.Height {
		return false
	}
	if m1.X != m2.X || m1.Y != m2.Y {
		return false
	}
	if m1.Reflect != m2.Reflect {
		return false
	}
	if m1.Rotation != m2.Rotation {
		return false
	}
	return true
}

func (dpy *Display) saveBrightness(output string, v float64) {
	dpy.cfg.Brightness[output] = v
	dpy.cfg.Save()
}

func (dpy *Display) savePrimary(output string) {
	dpy.cfg.Plans[dpy.cfg.CurrentPlanName].DefaultOutput = output
	dpy.cfg.Save()
}

func (dpy *Display) saveTouchScreen(output string, touchscreen string) {
	dpy.cfg.MapToTouchScreen[output] = touchscreen
	dpy.cfg.Save()
}

func (dpy *Display) saveDisplayMode(mode int16, output string) {
	dpy.cfg.DisplayMode = mode
	if mode == DisplayModeOnlyOne {
		dpy.cfg.Plans[dpy.cfg.CurrentPlanName].DefaultOutput = output
	}
	dpy.cfg.Save()
}

func loadConfigFromFile(dpy *Display, file string) (*ConfigDisplay, error) {
	fr, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fr.Close()

	cfg := &ConfigDisplay{
		Brightness:       make(map[string]float64),
		Plans:            make(map[string]*monitorGroup),
		MapToTouchScreen: make(map[string]string),
	}
	err = json.NewDecoder(fr).Decode(cfg)
	if err != nil {
		return nil, err
	}

	if !validConfig(cfg) {
		return nil, fmt.Errorf("Invalid config file: %v", file)
	}

	return cfg, nil
}
