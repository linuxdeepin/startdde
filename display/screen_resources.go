/*
 *  Copyright (C) 2019 ~ 2021 Uniontech Software Technology Co.,Ltd
 *
 * Author:
 *
 * Maintainer:
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package display

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/input"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
	"github.com/linuxdeepin/go-x11-client/ext/xfixes"
)

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

	if _greeterMode {
		// 仅 greeter 需要
		_, err = xfixes.QueryVersion(xConn, xfixes.MajorVersion, xfixes.MinorVersion).Reply(xConn)
		if err != nil {
			logger.Warning(err)
		}

		_, err = input.XIQueryVersion(xConn, input.MajorVersion, input.MinorVersion).Reply(xConn)
		if err != nil {
			logger.Warning(err)
			return
		}
	}
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

func getScreenResources(xConn *x.Conn) (*randr.GetScreenResourcesReply, error) {
	root := xConn.GetDefaultScreen().Root
	resources, err := randr.GetScreenResources(xConn, root).Reply(xConn)
	return resources, err
}

type screenResourcesManager struct {
	mu                      sync.Mutex
	xConn                   *x.Conn
	hasRandr1d2             bool
	cfgTs                   x.Timestamp
	monitorsCache           []*XOutputInfo
	modes                   []randr.ModeInfo
	crtcs                   map[randr.Crtc]*CrtcInfo
	outputs                 map[randr.Output]*OutputInfo
	primary                 randr.Output
	monitorChangedCbEnabled bool
	applyMu                 sync.Mutex

	monitorChangedCb     func(id uint32)
	forceUpdateMonitorCb func(xOutputInfo *XOutputInfo)
	primaryRectChangedCb func(info primaryMonitorInfo)
}

type XOutputInfo struct {
	crtc               randr.Crtc
	ID                 uint32
	Name               string
	Connected          bool
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

func (m *XOutputInfo) dumpForDebug() {
	logger.Debugf("XOutputInfo{crtc: %d,\nID: %v,\nName: %v,\nConnected: %v,\nCurrentMode: %v,\nPreferredMode: %v,\n"+
		"X: %v, Y: %v, Width: %v, Height: %v,\nRotation: %v,\nRotations: %v,\nMmWidth: %v,\nMmHeight: %v,\nModes: %v}",
		m.crtc, m.ID, m.Name, m.Connected, m.CurrentMode, m.PreferredMode, m.X, m.Y, m.Width, m.Height, m.Rotation, m.Rotations,
		m.MmWidth, m.MmHeight, m.Modes)
}

func (m *XOutputInfo) outputId() randr.Output {
	return randr.Output(m.ID)
}

func (m *XOutputInfo) equal(other *XOutputInfo) bool {
	return reflect.DeepEqual(m, other)
}

func newScreenResourcesManager(xConn *x.Conn, hasRandr1d2 bool) *screenResourcesManager {
	srm := &screenResourcesManager{
		xConn:       xConn,
		hasRandr1d2: hasRandr1d2,
		crtcs:       make(map[randr.Crtc]*CrtcInfo),
		outputs:     make(map[randr.Output]*OutputInfo),
	}
	err := srm.init()
	if err != nil {
		logger.Warning("screenResourceManager init failed:", err)
	}
	return srm
}

type CrtcInfo randr.GetCrtcInfoReply

func (ci *CrtcInfo) getRect() x.Rectangle {
	rect := x.Rectangle{
		X:      ci.X,
		Y:      ci.Y,
		Width:  ci.Width,
		Height: ci.Height,
	}
	return rect
}

type OutputInfo randr.GetOutputInfoReply

func (oi *OutputInfo) PreferredMode() randr.Mode {
	return (*randr.GetOutputInfoReply)(oi).GetPreferredMode()
}

type primaryMonitorInfo struct {
	Name string
	Rect x.Rectangle
}

func (pmi primaryMonitorInfo) IsRectEmpty() bool {
	return pmi.Rect == x.Rectangle{}
}

func (srm *screenResourcesManager) init() error {
	if !srm.hasRandr1d2 {
		return nil
	}
	xConn := srm.xConn
	resources, err := srm.getScreenResources(xConn)
	if err != nil {
		return err
	}
	srm.cfgTs = resources.ConfigTimestamp
	srm.modes = resources.Modes

	for _, outputId := range resources.Outputs {
		reply, err := srm.getOutputInfo(outputId)
		if err != nil {
			return err
		}
		srm.outputs[outputId] = (*OutputInfo)(reply)
	}

	for _, crtcId := range resources.Crtcs {
		reply, err := srm.getCrtcInfo(crtcId)
		if err != nil {
			return err
		}
		srm.crtcs[crtcId] = (*CrtcInfo)(reply)
	}

	srm.refreshMonitorsCache()
	return nil
}

func (srm *screenResourcesManager) getCrtcs() map[randr.Crtc]*CrtcInfo {
	srm.mu.Lock()
	defer srm.mu.Unlock()

	result := make(map[randr.Crtc]*CrtcInfo, len(srm.crtcs))
	for crtc, info := range srm.crtcs {
		infoCopy := &CrtcInfo{}
		*infoCopy = *info
		result[crtc] = infoCopy
	}
	return result
}

func (srm *screenResourcesManager) getMonitor(id uint32) *XOutputInfo {
	srm.mu.Lock()
	defer srm.mu.Unlock()
	for _, monitor := range srm.monitorsCache {
		if monitor.ID == id {
			monitorCopy := *monitor
			return &monitorCopy
		}
	}
	return nil
}

func (srm *screenResourcesManager) getMonitors() []*XOutputInfo {
	srm.mu.Lock()
	monitors := make([]*XOutputInfo, len(srm.monitorsCache))
	for i := 0; i < len(srm.monitorsCache); i++ {
		monitor := &XOutputInfo{}
		*monitor = *srm.monitorsCache[i]
		monitors[i] = monitor
	}
	srm.mu.Unlock()
	return monitors
}

func (srm *screenResourcesManager) doDiff() {
	logger.Debug("srm.doDiff")
	// NOTE: 不要加锁
	oldMonitors := monitorsNewToMap(srm.monitorsCache)
	srm.refreshMonitorsCache()
	newMonitors := srm.monitorsCache
	for _, monitor := range newMonitors {
		oldMonitor, ok := oldMonitors[monitor.ID]
		if ok {
			if !monitor.equal(oldMonitor) {
				if srm.monitorChangedCbEnabled {
					if srm.monitorChangedCb != nil {
						logger.Debug("do monitorChangedCb", monitor.ID)
						// NOTE: srm.mu 已经上锁了
						srm.mu.Unlock()
						srm.monitorChangedCb(monitor.ID)
						srm.mu.Lock()
					}
				} else {
					logger.Debug("monitorChangedCb disabled")
				}
				if monitor.outputId() == srm.primary {
					srm.mu.Unlock()
					srm.invokePrimaryRectChangedCb(srm.primary)
					srm.mu.Lock()
				}
			}
		} else {
			logger.Warning("can not handle new monitor")
		}
	}
}

func (srm *screenResourcesManager) wait(monitorCrtcCfgMap map[randr.Output]crtcConfig) {
	const (
		timeout  = 5 * time.Second
		interval = 500 * time.Millisecond
		count    = int(timeout / interval)
	)
	for i := 0; i < count; i++ {
		if srm.compareAll(monitorCrtcCfgMap) {
			logger.Debug("srm wait success")
			return
		}
		time.Sleep(interval)
	}
	logger.Warning("srm wait time out")
}

func (srm *screenResourcesManager) compareAll(monitorCrtcCfgMap map[randr.Output]crtcConfig) bool {
	srm.mu.Lock()
	defer srm.mu.Unlock()

	//logger.Debug("monitorCrtcCfgMap:", spew.Sdump(monitorCrtcCfgMap))
	//logger.Debug("srm.crtcs:", spew.Sdump(srm.crtcs))
	//logger.Debug("srm.outputs:", spew.Sdump(srm.outputs))

	for monitorId, crtcCfg := range monitorCrtcCfgMap {
		crtcInfo := srm.crtcs[crtcCfg.crtc]
		if !(crtcCfg.x == crtcInfo.X &&
			crtcCfg.y == crtcInfo.Y &&
			crtcCfg.mode == crtcInfo.Mode &&
			crtcCfg.rotation == crtcInfo.Rotation &&
			outputSliceEqual(crtcCfg.outputs, crtcInfo.Outputs)) {
			logger.Debugf("[compareAll] crtc %d not match", crtcCfg.crtc)
			return false
		}

		if len(crtcCfg.outputs) > 0 {
			outputInfo := srm.outputs[crtcCfg.outputs[0]]
			if outputInfo.Crtc != crtcCfg.crtc {
				logger.Debugf("[compareAll] output %v crtc not match", monitorId)
				return false
			}
		} else {
			outputInfo := srm.outputs[monitorId]
			if outputInfo.Crtc != 0 {
				logger.Debugf("[compareAll] output %v crtc != 0", monitorId)
				return false
			}
		}
	}
	return true
}

func monitorsNewToMap(monitors []*XOutputInfo) map[uint32]*XOutputInfo {
	result := make(map[uint32]*XOutputInfo, len(monitors))
	for _, monitor := range monitors {
		result[monitor.ID] = monitor
	}
	return result
}

func (srm *screenResourcesManager) refreshMonitorsCache() {
	// NOTE: 不要加锁
	monitors := make([]*XOutputInfo, 0, len(srm.outputs))
	for outputId, outputInfo := range srm.outputs {
		monitor := &XOutputInfo{
			crtc:      outputInfo.Crtc,
			ID:        uint32(outputId),
			Name:      outputInfo.Name,
			Connected: outputInfo.Connection == randr.ConnectionConnected,
			Modes:     toModeInfos(srm.modes, outputInfo.Modes),
			MmWidth:   outputInfo.MmWidth,
			MmHeight:  outputInfo.MmHeight,
		}
		monitor.PreferredMode = getPreferredMode(monitor.Modes, uint32(outputInfo.PreferredMode()))
		var err error
		monitor.EDID, err = srm.getOutputEdid(outputId)
		if err != nil {
			logger.Warningf("get output %d edid failed: %v", outputId, err)
		}
		monitor.Manufacturer, monitor.Model = parseEdid(monitor.EDID)

		availFillModes, err := srm.getOutputAvailableFillModes(outputId)
		if err != nil {
			logger.Warningf("get output %d available fill modes failed: %v", outputId, err)
		}
		monitor.AvailableFillModes = availFillModes

		// TODO 获取显示器当前的 fill mode

		if monitor.crtc != 0 {
			crtcInfo := srm.crtcs[monitor.crtc]
			if crtcInfo != nil {
				monitor.X = crtcInfo.X
				monitor.Y = crtcInfo.Y
				monitor.Width = crtcInfo.Width
				monitor.Height = crtcInfo.Height
				monitor.Rotation = crtcInfo.Rotation
				monitor.Rotations = crtcInfo.Rotations
				monitor.CurrentMode = findModeInfo(srm.modes, crtcInfo.Mode)
			}
		}

		monitors = append(monitors, monitor)
	}

	srm.monitorsCache = monitors
}

func (srm *screenResourcesManager) findFreeCrtc(output randr.Output) randr.Crtc {
	srm.mu.Lock()
	defer srm.mu.Unlock()

	for crtc, crtcInfo := range srm.crtcs {
		if len(crtcInfo.Outputs) == 0 && outputSliceContains(crtcInfo.PossibleOutputs, output) {
			return crtc
		}
	}
	return 0
}

type applyOptions map[string]interface{}

const (
	optionDisableCrtc = "disableCrtc"
	optionOnlyOne     = "onlyOne"
)

type crtcConfig struct {
	crtc    randr.Crtc
	outputs []randr.Output

	x        int16
	y        int16
	rotation uint16
	mode     randr.Mode
}

func findOutputInMonitorCrtcCfgMap(monitorCrtcCfgMap map[randr.Output]crtcConfig, crtc randr.Crtc) randr.Output {
	for output, config := range monitorCrtcCfgMap {
		if config.crtc == crtc {
			return output
		}
	}
	return 0
}

func (srm *screenResourcesManager) apply(monitorMap map[uint32]*Monitor, prevScreenSize screenSize, options applyOptions,
	fillModes map[string]string) error {

	logger.Debug("call apply")
	optDisableCrtc, _ := options[optionDisableCrtc].(bool)

	srm.applyMu.Lock()
	x.GrabServer(srm.xConn)
	logger.Debug("grab server")
	ungrabServerDone := false
	monitorCrtcCfgMap := make(map[randr.Output]crtcConfig)
	// 禁止更新显示器对象
	srm.monitorChangedCbEnabled = false

	ungrabServer := func() {
		if !ungrabServerDone {
			logger.Debug("ungrab server")
			err := x.UngrabServerChecked(srm.xConn).Check(srm.xConn)
			if err != nil {
				logger.Warning(err)
			}
			ungrabServerDone = true
		}
	}

	defer func() {
		ungrabServer()

		srm.monitorChangedCbEnabled = true
		srm.applyMu.Unlock()
		logger.Debug("apply return")
	}()

	// 根据 monitor 的配置，准备 crtc 配置到 monitorCrtcCfgMap
	for output, monitor := range monitorMap {
		monitor.dumpInfoForDebug()
		xOutputInfo := srm.getMonitor(monitor.ID)
		if xOutputInfo == nil {
			logger.Warningf("[apply] failed to get monitor %d", monitor.ID)
			continue
		}
		crtc := xOutputInfo.crtc
		if monitor.Enabled {
			// 启用显示器
			if crtc == 0 {
				crtc = srm.findFreeCrtc(randr.Output(output))
				if crtc == 0 {
					return errors.New("failed to find free crtc")
				}
			}
			monitorCrtcCfgMap[randr.Output(output)] = crtcConfig{
				crtc:     crtc,
				x:        monitor.X,
				y:        monitor.Y,
				mode:     randr.Mode(monitor.CurrentMode.Id),
				rotation: monitor.Rotation | monitor.Reflect,
				outputs:  []randr.Output{randr.Output(output)},
			}
		} else {
			// 禁用显示器
			if crtc != 0 {
				// 禁用此 crtc，把它的 outputs 设置为空。
				monitorCrtcCfgMap[randr.Output(output)] = crtcConfig{
					crtc:     crtc,
					rotation: randr.RotationRotate0,
				}
			}
		}
	}
	logger.Debug("monitorCrtcCfgMap:", spew.Sdump(monitorCrtcCfgMap))

	// 未来的，apply 之后的屏幕所需尺寸
	screenSize := getScreenSize(monitorMap)
	logger.Debugf("screen size after apply: %+v", screenSize)

	monitors := getConnectedMonitors(monitorMap)

	for crtc, crtcInfo := range srm.getCrtcs() {
		rect := crtcInfo.getRect()
		logger.Debugf("crtc %v, crtcInfo: %+v", crtc, crtcInfo)

		// 是否考虑临时禁用 crtc
		shouldDisable := false

		if optDisableCrtc {
			// 可能是切换了显示模式
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
			output := findOutputInMonitorCrtcCfgMap(monitorCrtcCfgMap, crtc)
			if output != 0 {
				monitor := monitors.GetById(uint32(output))
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
		}

		if shouldDisable && len(crtcInfo.Outputs) > 0 {
			logger.Debugf("disable crtc %v, it's outputs: %v", crtc, crtcInfo.Outputs)
			err := srm.disableCrtc(crtc)
			if err != nil {
				return err
			}
		}
	}

	err := srm.setScreenSize(screenSize)
	if err != nil {
		return err
	}

	for output, monitor := range monitorMap {
		crtcCfg, ok := monitorCrtcCfgMap[randr.Output(output)]
		if !ok {
			continue
		}
		logger.Debugf("applyConfig output: %v %v", monitor.ID, monitor.Name)
		err := srm.applyConfig(crtcCfg)
		if err != nil {
			return err
		}
		srm.setMonitorFileMode(monitor, fillModes)
	}

	ungrabServer()

	// 等待所有事件结束
	srm.wait(monitorCrtcCfgMap)

	// 更新一遍所有显示器
	logger.Debug("update all monitors")
	for _, monitor := range monitorMap {
		xOutputInfo := srm.getMonitor(monitor.ID)
		if xOutputInfo != nil {
			if srm.forceUpdateMonitorCb != nil {
				srm.forceUpdateMonitorCb(xOutputInfo)
			}
		}
	}
	logger.Debug("after update all monitors")

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
		// TODO
		//m.PropsMu.RLock()
		//rect := m.PrimaryRect
		//m.PropsMu.RUnlock()
		//err := m.emitPropChangedPrimaryRect(rect)
		//if err != nil {
		//	logger.Warning(err)
		//}
	}

	return nil
}

func (srm *screenResourcesManager) setMonitorFileMode(monitor *Monitor, fillModes map[string]string) {
	if !monitor.Enabled {
		return
	}
	fillMode := fillModes[monitor.generateFillModeKey()]
	if len(monitor.AvailableFillModes) > 0 {
		if fillMode == "" {
			fillMode = fillModeDefault
		}

		err := srm.setOutputScalingMode(randr.Output(monitor.ID), fillMode)
		if err != nil {
			logger.Warning(err)
		} else {
			// TODO 后续可以根据 output 属性改变来处理
			monitor.setPropCurrentFillMode(fillMode)
		}
	}
}

func getConnectedMonitors(monitorMap map[uint32]*Monitor) Monitors {
	var monitors Monitors
	for _, monitor := range monitorMap {
		if monitor.Connected {
			monitors = append(monitors, monitor)
		}
	}
	return monitors
}

// getScreenSize 计算出需要的屏幕尺寸
func getScreenSize(monitorMap map[uint32]*Monitor) screenSize {
	width, height := getScreenWidthHeight(monitorMap)
	mmWidth := uint32(float64(width) / 3.792)
	mmHeight := uint32(float64(height) / 3.792)
	return screenSize{
		width:    width,
		height:   height,
		mmWidth:  mmWidth,
		mmHeight: mmHeight,
	}
}

// getScreenWidthHeight 根据 monitorMap 中显示器的设置，计算出需要的屏幕尺寸。
func getScreenWidthHeight(monitorMap map[uint32]*Monitor) (sw, sh uint16) {
	var w, h int
	for _, monitor := range monitorMap {
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

func (srm *screenResourcesManager) setScreenSize(ss screenSize) error {
	root := srm.xConn.GetDefaultScreen().Root
	err := randr.SetScreenSizeChecked(srm.xConn, root, ss.width, ss.height, ss.mmWidth,
		ss.mmHeight).Check(srm.xConn)
	logger.Debugf("set screen size %dx%d, mm: %dx%d",
		ss.width, ss.height, ss.mmWidth, ss.mmHeight)
	return err
}

func (srm *screenResourcesManager) disableCrtc(crtc randr.Crtc) error {
	srm.mu.Lock()
	cfgTs := srm.cfgTs
	srm.mu.Unlock()

	setCfg, err := randr.SetCrtcConfig(srm.xConn, crtc, 0, cfgTs,
		0, 0, 0, randr.RotationRotate0, nil).Reply(srm.xConn)
	if err != nil {
		return err
	}
	if setCfg.Status != randr.SetConfigSuccess {
		return fmt.Errorf("failed to disable crtc %d: %v",
			crtc, getRandrStatusStr(setCfg.Status))
	}
	return nil
}

func (srm *screenResourcesManager) applyConfig(cfg crtcConfig) error {
	srm.mu.Lock()
	cfgTs := srm.cfgTs
	srm.mu.Unlock()

	logger.Debugf("setCrtcConfig crtc: %v, cfgTs: %v, x: %v, y: %v,"+
		" mode: %v, rotation|reflect: %v, outputs: %v",
		cfg.crtc, cfgTs, cfg.x, cfg.y, cfg.mode, cfg.rotation, cfg.outputs)
	setCfg, err := randr.SetCrtcConfig(srm.xConn, cfg.crtc, 0, cfgTs,
		cfg.x, cfg.y, cfg.mode, cfg.rotation,
		cfg.outputs).Reply(srm.xConn)
	if err != nil {
		return err
	}
	if setCfg.Status != randr.SetConfigSuccess {
		err = fmt.Errorf("failed to configure crtc %v: %v",
			cfg.crtc, getRandrStatusStr(setCfg.Status))
		return err
	}
	return nil
}

func (srm *screenResourcesManager) getOutputAvailableFillModes(output randr.Output) ([]string, error) {
	// 判断是否有该属性
	lsPropsReply, err := randr.ListOutputProperties(srm.xConn, output).Reply(srm.xConn)
	if err != nil {
		return nil, err
	}
	atomScalingMode, err := srm.xConn.GetAtom("scaling mode")
	if err != nil {
		return nil, err
	}
	hasProp := false
	for _, atom := range lsPropsReply.Atoms {
		if atom == atomScalingMode {
			hasProp = true
			break
		}
	}
	if !hasProp {
		return nil, nil
	}
	// 获取属性可能的值
	outputProp, _ := randr.QueryOutputProperty(srm.xConn, output, atomScalingMode).Reply(srm.xConn)
	var result []string
	for _, prop := range outputProp.ValidValues {
		fillMode, _ := srm.xConn.GetAtomName(x.Atom(prop))
		if fillMode != "None" {
			result = append(result, fillMode)
		}
	}
	return result, nil
}

func (srm *screenResourcesManager) setOutputScalingMode(output randr.Output, fillMode string) error {
	if fillMode != fillModeDefault &&
		fillMode != fillModeCenter &&
		fillMode != fillModeFull {
		logger.Warning("invalid fill mode:", fillMode)
		return fmt.Errorf("invalid fill mode %q", fillMode)
	}

	xConn := srm.xConn
	fillModeU32, _ := xConn.GetAtom(fillMode)
	name, _ := xConn.GetAtom("scaling mode")

	// TODO 改成不用 get ？
	outputPropReply, err := randr.GetOutputProperty(xConn, output, name, 0, 0,
		100, false, false).Reply(xConn)
	if err != nil {
		logger.Warning("call GetOutputProperty reply err:", err)
		return err
	}

	w := x.NewWriter()
	w.Write4b(uint32(fillModeU32))
	fillModeByte := w.Bytes()
	err = randr.ChangeOutputPropertyChecked(xConn, output, name,
		outputPropReply.Type, outputPropReply.Format, 0, fillModeByte).Check(xConn)
	if err != nil {
		logger.Warning("err:", err)
		return err
	}

	return nil
}

func (srm *screenResourcesManager) SetMonitorPrimary(monitorId uint32) error {
	logger.Debug("srm.SetMonitorPrimary", monitorId)
	srm.mu.Lock()
	srm.primary = randr.Output(monitorId)
	srm.mu.Unlock()
	err := srm.setOutputPrimary(randr.Output(monitorId))
	if err != nil {
		return err
	}

	// 设置之后处理一次更新回调
	pOut, err := srm.GetOutputPrimary()
	if err != nil {
		return err
	}
	srm.invokePrimaryRectChangedCb(pOut)
	return nil
}

func (srm *screenResourcesManager) invokePrimaryRectChangedCb(pOut randr.Output) {
	// NOTE: 需要处于不加锁 srm.mu 的环境
	pmi := srm.getPrimaryMonitorInfo(pOut)
	if srm.primaryRectChangedCb != nil {
		logger.Debug("do primaryRectChangedCb", pmi)
		srm.primaryRectChangedCb(pmi)
	}
}

func (srm *screenResourcesManager) setOutputPrimary(output randr.Output) error {
	logger.Debug("set output primary", output)
	root := srm.xConn.GetDefaultScreen().Root
	return randr.SetOutputPrimaryChecked(srm.xConn, root, output).Check(srm.xConn)
}

func (srm *screenResourcesManager) GetOutputPrimary() (randr.Output, error) {
	root := srm.xConn.GetDefaultScreen().Root
	reply, err := randr.GetOutputPrimary(srm.xConn, root).Reply(srm.xConn)
	if err != nil {
		return 0, err
	}
	return reply.Output, nil
}

func (srm *screenResourcesManager) getPrimaryMonitorInfo(pOutput randr.Output) (pmi primaryMonitorInfo) {
	srm.mu.Lock()
	defer srm.mu.Unlock()

	if pOutput != 0 {
		for output, outputInfo := range srm.outputs {
			if pOutput != output {
				continue
			}

			pmi.Name = outputInfo.Name

			if outputInfo.Crtc == 0 {
				logger.Warning("new primary output crtc is 0")
			} else {
				crtcInfo := srm.crtcs[outputInfo.Crtc]
				if crtcInfo == nil {
					logger.Warning("crtcInfo is nil")
				} else {
					pmi.Rect = crtcInfo.getRect()
				}
			}
			break
		}
	}
	return
}

func (srm *screenResourcesManager) getCrtcInfo(crtc randr.Crtc) (*randr.GetCrtcInfoReply, error) {
	crtcInfo, err := randr.GetCrtcInfo(srm.xConn, crtc, srm.cfgTs).Reply(srm.xConn)
	if err != nil {
		return nil, err
	}
	if crtcInfo.Status != randr.StatusSuccess {
		return nil, fmt.Errorf("status is not success, is %v", crtcInfo.Status)
	}
	return crtcInfo, err
}

func (srm *screenResourcesManager) getOutputInfo(outputId randr.Output) (*randr.GetOutputInfoReply, error) {
	outputInfo, err := randr.GetOutputInfo(srm.xConn, outputId, srm.cfgTs).Reply(srm.xConn)
	if err != nil {
		return nil, err
	}
	if outputInfo.Status != randr.StatusSuccess {
		return nil, fmt.Errorf("status is not success, is %v", outputInfo.Status)
	}
	return outputInfo, err
}

func (srm *screenResourcesManager) getOutputEdid(output randr.Output) ([]byte, error) {
	atomEDID, err := srm.xConn.GetAtom("EDID")
	if err != nil {
		return nil, err
	}

	reply, err := randr.GetOutputProperty(srm.xConn, output,
		atomEDID, x.AtomInteger,
		0, 32, false, false).Reply(srm.xConn)
	if err != nil {
		return nil, err
	}
	return reply.Value, nil
}

func (srm *screenResourcesManager) getScreenResources(xConn *x.Conn) (*randr.GetScreenResourcesReply, error) {
	root := xConn.GetDefaultScreen().Root
	resources, err := randr.GetScreenResources(xConn, root).Reply(xConn)
	return resources, err
}

func (srm *screenResourcesManager) getScreenResourcesCurrent() (*randr.GetScreenResourcesCurrentReply, error) {
	root := srm.xConn.GetDefaultScreen().Root
	resources, err := randr.GetScreenResourcesCurrent(srm.xConn, root).Reply(srm.xConn)
	return resources, err
}

func (srm *screenResourcesManager) handleCrtcChanged(e *randr.CrtcChangeNotifyEvent) {
	// NOTE: 不要加锁
	reply, err := srm.getCrtcInfo(e.Crtc)
	if err != nil {
		logger.Warningf("get crtc %v info failed: %v", e.Crtc, err)
		return
	}
	// 这些字段使用 event 中提供的
	reply.X = e.X
	reply.Y = e.Y
	reply.Width = e.Width
	reply.Height = e.Height
	reply.Rotation = e.Rotation
	reply.Mode = e.Mode

	srm.crtcs[e.Crtc] = (*CrtcInfo)(reply)
}

func (srm *screenResourcesManager) handleOutputChanged(e *randr.OutputChangeNotifyEvent) {
	// NOTE: 不要加锁
	outputInfo := srm.outputs[e.Output]
	if outputInfo == nil {
		reply, err := srm.getOutputInfo(e.Output)
		if err != nil {
			logger.Warningf("get output %v info failed: %v", e.Output, err)
			return
		}
		srm.outputs[e.Output] = (*OutputInfo)(reply)
		return
	}

	// e.Mode 和 e.Rotation 没有被使用到
	outputInfo.Crtc = e.Crtc
	//e.Mode 有什么用？
	//e.Rotation 有什么用？
	outputInfo.Connection = e.Connection
	outputInfo.SubPixelOrder = e.SubPixelOrder
}

func (srm *screenResourcesManager) HandleEvent(ev interface{}) {
	srm.mu.Lock()
	defer srm.mu.Unlock()
	logger.Debugf("srm.HandleEvent %#v", ev)
	defer logger.Debugf("srm.HandleEvent return %#v", ev)

	switch e := ev.(type) {
	case *randr.CrtcChangeNotifyEvent:
		srm.handleCrtcChanged(e)
	case *randr.OutputChangeNotifyEvent:
		srm.handleOutputChanged(e)
		// NOTE: ScreenChangeNotifyEvent 事件比较特殊，不在这里处理。
	default:
		logger.Debug("invalid event", ev)
		return
	}

	srm.doDiff()
}

func (srm *screenResourcesManager) HandleScreenChanged(e *randr.ScreenChangeNotifyEvent) (cfgTsChanged bool) {
	srm.mu.Lock()
	defer srm.mu.Unlock()
	logger.Debugf("srm.HandleScreenChanged %#v", e)
	defer logger.Debugf("srm.HandleScreenChanged return %#v", e)
	cfgTsChanged = srm.handleScreenChanged(e)
	srm.doDiff()
	return
}

func (srm *screenResourcesManager) handleScreenChanged(e *randr.ScreenChangeNotifyEvent) (cfgTsChanged bool) {
	// NOTE: 不要加锁
	if srm.cfgTs == e.ConfigTimestamp {
		return false
	}
	cfgTsChanged = true
	resources, err := srm.getScreenResourcesCurrent()
	if err != nil {
		logger.Warning("get current screen resources failed:", err)
		return
	}
	srm.cfgTs = resources.ConfigTimestamp
	srm.modes = resources.Modes

	srm.outputs = make(map[randr.Output]*OutputInfo)
	for _, outputId := range resources.Outputs {
		reply, err := srm.getOutputInfo(outputId)
		if err != nil {
			logger.Warningf("get output %v info failed: %v", outputId, err)
			continue
		}
		srm.outputs[outputId] = (*OutputInfo)(reply)
	}

	srm.crtcs = make(map[randr.Crtc]*CrtcInfo)
	for _, crtcId := range resources.Crtcs {
		reply, err := srm.getCrtcInfo(crtcId)
		if err != nil {
			logger.Warningf("get crtc %v info failed: %v", crtcId, err)
			continue
		}
		srm.crtcs[crtcId] = (*CrtcInfo)(reply)
	}
	return
}

func (srm *screenResourcesManager) showCursor(show bool) error {
	rootWin := srm.xConn.GetDefaultScreen().Root
	var cookie x.VoidCookie
	if show {
		logger.Debug("xfixes show cursor")
		cookie = xfixes.ShowCursorChecked(srm.xConn, rootWin)
	} else {
		logger.Debug("xfixes hide cursor")
		cookie = xfixes.HideCursorChecked(srm.xConn, rootWin)
	}
	err := cookie.Check(srm.xConn)
	return err
}
