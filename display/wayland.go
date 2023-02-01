// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/godbus/dbus"
	kwayland "github.com/linuxdeepin/go-dbus-factory/session/org.deepin.dde.kwayland1"
	gio "github.com/linuxdeepin/go-gir/gio-2.0"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
)

type monitorIdGenerator struct {
	nextId    uint32
	uuidIdMap map[string]uint32
	mu        sync.Mutex
}

func newMonitorIdGenerator() *monitorIdGenerator {
	return &monitorIdGenerator{
		nextId:    1,
		uuidIdMap: make(map[string]uint32),
	}
}

func (mig *monitorIdGenerator) getId(uuid string) uint32 {
	mig.mu.Lock()
	defer mig.mu.Unlock()

	id, ok := mig.uuidIdMap[uuid]
	if ok {
		return id
	}
	id = mig.nextId
	mig.nextId++
	mig.uuidIdMap[uuid] = id
	return id
}

func (mig *monitorIdGenerator) getUuidById(id uint32) string {
	mig.mu.Lock()
	defer mig.mu.Unlock()

	for uuid, id0 := range mig.uuidIdMap {
		if id0 == id {
			return uuid
		}
	}
	return ""
}

type KOutputInfo struct {
	// json 中不提供 id
	id           uint32
	UUID         string      `json:"uuid"`
	Name         string      `json:"name"`
	EdidBase64   string      `json:"edid_base64"`
	Enabled      int32       `json:"enabled"`
	X            int32       `json:"x"`
	Y            int32       `json:"y"`
	Width        int32       `json:"width"`
	Height       int32       `json:"height"`
	RefreshRate  int32       `json:"refresh_rate"`
	Manufacturer string      `json:"manufacturer"`
	Model        string      `json:"model"`
	ModeInfos    []KModeInfo `json:"ModeInfo"`
	PhysHeight   int32       `json:"phys_height"`
	PhysWidth    int32       `json:"phys_width"`
	Transform    int32       `json:"transform"`
	Scale        float64     `json:"scale"`
}

func (oi *KOutputInfo) setId(mig *monitorIdGenerator) {
	oi.id = mig.getId(oi.UUID)
}

func (oi *KOutputInfo) getModes() (result []ModeInfo) {
	for _, mi := range oi.ModeInfos {
		result = append(result, mi.toModeInfo())
	}
	sort.Sort(sort.Reverse(ModeInfos(result)))
	return
}

const (
	OutputDeviceTransformNormal     = 0
	OutputDeviceTransform90         = 1
	OutputDeviceTransform180        = 2
	OutputDeviceTransform270        = 3
	OutputDeviceTransformFlipped    = 4
	OutputDeviceTransformFlipped90  = 5
	OutputDeviceTransformFlipped180 = 6
	OutputDeviceTransformFlipped270 = 7
)

const (
	OutputDeviceModeCurrent   = 1 << 0
	OutputDeviceModePreferred = 1 << 1
)

const (
	gsSchemaXSettings      = "com.deepin.xsettings"
	gsXSettingsPrimaryName = "primary-monitor-name"
)

func (oi *KOutputInfo) getBestMode() ModeInfo {
	var preferredMode *KModeInfo
	for _, info := range oi.ModeInfos {
		if info.Flags&OutputDeviceModePreferred != 0 {
			preferredMode = &info
			break
		}
	}

	if preferredMode == nil {
		// not found preferred mode
		return getMaxAreaOutputDeviceMode(oi.ModeInfos).toModeInfo()
	}
	return preferredMode.toModeInfo()
}

func (oi *KOutputInfo) getCurrentMode() ModeInfo {
	for _, info := range oi.ModeInfos {
		if info.Flags&OutputDeviceModeCurrent != 0 {
			return info.toModeInfo()
		}
	}
	return ModeInfo{}
}

func (oi *KOutputInfo) rotation() uint16 {
	switch oi.Transform {
	case OutputDeviceTransformNormal:
		return randr.RotationRotate0
	case OutputDeviceTransform90:
		return randr.RotationRotate90
	case OutputDeviceTransform180:
		return randr.RotationRotate180
	case OutputDeviceTransform270:
		return randr.RotationRotate270

	case OutputDeviceTransformFlipped:
		return randr.RotationRotate0
	case OutputDeviceTransformFlipped90:
		return randr.RotationRotate90
	case OutputDeviceTransformFlipped180:
		return randr.RotationRotate180
	case OutputDeviceTransformFlipped270:
		return randr.RotationRotate270
	}
	return 0
}

func randrRotationToTransform(rotation int) int {
	switch rotation {
	case randr.RotationRotate0:
		return OutputDeviceTransformNormal
	case randr.RotationRotate90:
		return OutputDeviceTransform90
	case randr.RotationRotate180:
		return OutputDeviceTransform180
	case randr.RotationRotate270:
		return OutputDeviceTransform270
	}
	return 0
}

func getMaxAreaOutputDeviceMode(modes []KModeInfo) KModeInfo {
	if len(modes) == 0 {
		return KModeInfo{}
	}
	maxAreaMode := modes[0]
	for _, mode := range modes[1:] {
		if int(maxAreaMode.Width)*int(maxAreaMode.Height) < int(mode.Width)*int(mode.Height) {
			maxAreaMode = mode
		}
	}
	return maxAreaMode
}

func (oi *KOutputInfo) getEnabled() bool {
	return int32ToBool(oi.Enabled)
}

func (oi *KOutputInfo) toMonitorInfo(mm *kMonitorManager) *MonitorInfo {
	mi := &MonitorInfo{
		Enabled:          oi.Enabled != 0,
		ID:               oi.id,
		Name:             oi.Name,
		Connected:        true,
		VirtualConnected: true,
		Modes:            oi.getModes(),
		CurrentMode:      oi.getCurrentMode(),
		PreferredMode:    oi.getBestMode(),
		X:                int16(oi.X),
		Y:                int16(oi.Y),
		Width:            uint16(oi.Width),
		Height:           uint16(oi.Height),
		Rotation:         oi.rotation(),
		Rotations: randr.RotationRotate0 | randr.RotationRotate90 |
			randr.RotationRotate180 | randr.RotationRotate270,
		MmHeight:     uint32(oi.PhysHeight),
		MmWidth:      uint32(oi.PhysWidth),
		Manufacturer: oi.Manufacturer,
		Model:        oi.Model,
	}
	edid, err := decodeEdidBase64(oi.EdidBase64)
	if err != nil {
		logger.Warningf("decode monitor %v %v edid failed: %v", mi.ID, mi.Name, err)
	} else {
		mi.EDID = edid
	}
	if logger.GetLogLevel() == log.LevelDebug {
		logger.Debugf("monitor %v %v edid: %v", mi.ID, mi.Name, spew.Sdump(mi.EDID))
		manufacturer, model := parseEdid(mi.EDID)
		logger.Debugf("parse edid manufacturer: %q, model: %q", manufacturer, model)
	}

	mi.UUID = oi.UUID
	mi.UuidV0 = oi.UUID
	if mi.Connected {
		stdName, err := mm.getStdMonitorName(mi.Name, mi.EDID)
		if err != nil {
			logger.Warningf("get monitor %v std name failed: %v", mi.Name, err)
		}
		mi.UUID = getOutputUuid(mi.Name, stdName, mi.EDID)
	}
	return mi
}

func decodeEdidBase64(edidB64 string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(edidB64)
}

func encodeEdidBase64(edid []byte) string {
	return base64.StdEncoding.EncodeToString(edid)
}

type KModeInfo struct {
	Id          int32 `json:"id"`
	Width       int32 `json:"width"`
	Height      int32 `json:"height"`
	RefreshRate int32 `json:"refresh_rate"`
	Flags       int32 `json:"flags"`
}

func (mi KModeInfo) toModeInfo() ModeInfo {
	return ModeInfo{
		Id:     uint32(mi.Id),
		name:   mi.name(),
		Width:  uint16(mi.Width),
		Height: uint16(mi.Height),
		Rate:   mi.rate(),
	}
}

func (mi KModeInfo) name() string {
	return fmt.Sprintf("%dx%d", mi.Width, mi.Height)
}

func (mi KModeInfo) rate() float64 {
	return float64(mi.RefreshRate) / 1000.0
}

func unmarshalOutputInfos(str string) ([]*KOutputInfo, error) {
	var v outputInfoWrap
	err := json.Unmarshal([]byte(str), &v)
	if err != nil {
		return nil, err
	}
	return v.OutputInfo, nil
}

func unmarshalOutputInfo(str string) (*KOutputInfo, error) {
	var v outputInfoWrap
	err := json.Unmarshal([]byte(str), &v)
	if err != nil {
		return nil, err
	}
	if len(v.OutputInfo) == 0 {
		return nil, errors.New("length of slice v.OutputInfo is 0")
	}
	return v.OutputInfo[0], nil
}

type outputInfoWrap struct {
	OutputInfo []*KOutputInfo
}

func int32ToBool(v int32) bool {
	return v != 0
}

func (m *Manager) removeMonitor(id uint32) *Monitor {
	m.monitorMapMu.Lock()

	monitor, ok := m.monitorMap[id]
	if !ok {
		m.monitorMapMu.Unlock()
		return nil
	}
	delete(m.monitorMap, id)
	m.monitorMapMu.Unlock()

	err := m.service.StopExport(monitor)
	if err != nil {
		logger.Warning(err)
	}
	return monitor
}

type kMonitorManager struct {
	hooks          monitorManagerHooks
	sessionSigLoop *dbusutil.SignalLoop
	mu             sync.Mutex
	management     kwayland.OutputManagement
	mig            *monitorIdGenerator
	monitorMap     map[uint32]*MonitorInfo
	primary        uint32
	// 键是 wayland 原始数据 model 字段处理过后的显示器名称，值是标准名。
	stdNamesCache map[string]string

	xSettingsGs *gio.Settings
}

func newKMonitorManager(sessionSigLoop *dbusutil.SignalLoop) *kMonitorManager {
	kmm := &kMonitorManager{
		sessionSigLoop: sessionSigLoop,
		monitorMap:     make(map[uint32]*MonitorInfo),
		stdNamesCache:  make(map[string]string),
	}

	sessionBus := sessionSigLoop.Conn()
	kmm.mig = newMonitorIdGenerator()
	kmm.management = kwayland.NewOutputManagement(sessionBus)
	err := kmm.init()
	if err != nil {
		logger.Warning(err)
	}
	return kmm
}

func (mm *kMonitorManager) syncPrimary() error {
	logger.Info("syncPrimary", mm.primary)
	monitor := mm.getMonitor(mm.primary)
	if monitor == nil {
		return errors.New("cannot match primary, sync primary failed")
	}

	// 启动后先同步一次,将主屏幕名称同步到xsettings
	mm.xSettingsGs = gio.NewSettings(gsSchemaXSettings)
	for _, key := range mm.xSettingsGs.ListKeys() {
		if gsXSettingsPrimaryName == key {
			oldPrimary := mm.xSettingsGs.GetString(gsXSettingsPrimaryName)
			if oldPrimary != monitor.Name {
				mm.xSettingsGs.SetString(gsXSettingsPrimaryName, monitor.Name)
				break
			}
		}
	}

	// 同步设置到窗管
	sessionBus, err := dbus.SessionBus()
	if err != nil {
		return err
	}
	sessionObj := sessionBus.Object("org.deepin.dde.KWayland1", "/org/deepin/dde/KWayland1/Output")
	err = sessionObj.Call("org.deepin.dde.KWayland1.Output.setPrimary", 0, monitor.Name).Store()
	if err != nil {
		logger.Warning(err)
		return err
	}
	return nil
}

func (mm *kMonitorManager) init() error {
	mm.listenDBusSignals()
	// init monitorMap
	kOutputInfos, err := mm.listOutput()
	if err != nil {
		logger.Warning(err)
	}
	mm.mu.Lock()
	monitors := kOutputInfos.toMonitorInfos(mm)
	for _, monitor := range monitors {
		mm.monitorMap[monitor.ID] = monitor
	}
	if len(monitors) != 0 {
		mm.primary = monitors[0].ID
	}
	mm.mu.Unlock()
	return nil
}

type KOutputInfos []*KOutputInfo

func (infos KOutputInfos) toMonitorInfos(mm *kMonitorManager) []*MonitorInfo {
	result := make([]*MonitorInfo, 0, len(infos))
	for _, info := range infos {
		result = append(result, info.toMonitorInfo(mm))
	}
	return result
}

func (mm *kMonitorManager) listOutput() (KOutputInfos, error) {
	var outputJ string
	var duration = 500 * time.Millisecond
	// sometimes got the output list will return nil, this is the output service not inited yet.
	// so try got 3 times.
	for i := 0; i < 3; i++ {
		data, err := mm.management.ListOutput(0)
		if len(data) != 0 {
			outputJ = data
			break
		}

		if err != nil || len(data) == 0 {
			logger.Warning("Failed to get output list:", err)
		}
		time.Sleep(duration)
		duration += 100 * time.Millisecond
	}
	logger.Debug("outputJ:", outputJ)
	outputInfos, err := unmarshalOutputInfos(outputJ)
	if err != nil {
		return nil, err
	}
	// 补充 id
	for _, info := range outputInfos {
		info.setId(mm.mig)
	}
	return outputInfos, nil
}

func (mm *kMonitorManager) setHooks(hooks monitorManagerHooks) {
	mm.hooks = hooks
}

func (mm *kMonitorManager) setMonitorFillMode(monitor *Monitor, fillMode string) error {
	return nil
}

func (mm *kMonitorManager) getMonitors() []*MonitorInfo {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	monitors := make([]*MonitorInfo, 0, len(mm.monitorMap))
	for _, monitor := range mm.monitorMap {
		monitorCp := *monitor
		monitors = append(monitors, &monitorCp)
	}
	return monitors
}

func (mm *kMonitorManager) getMonitor(id uint32) *MonitorInfo {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	return mm.getMonitorNoLock(id)
}

func (mm *kMonitorManager) getMonitorNoLock(id uint32) *MonitorInfo {
	monitorInfo, ok := mm.monitorMap[id]
	if !ok {
		return nil
	}
	monitor := *monitorInfo
	return &monitor
}

func (mm *kMonitorManager) getPrimaryMonitor() *MonitorInfo {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	return mm.getMonitorNoLock(mm.primary)
}

func (mm *kMonitorManager) getStdMonitorName(name string, edid []byte) (string, error) {
	// NOTE：不要加锁
	stdName := mm.stdNamesCache[name]
	if stdName != "" {
		return stdName, nil
	}

	stdName, err := getStdMonitorName(edid)
	if err != nil {
		return "", err
	}
	mm.stdNamesCache[name] = stdName
	return stdName, nil
}

func (mm *kMonitorManager) apply(monitorsId monitorsId, monitorMap map[uint32]*Monitor, prevScreenSize screenSize,
	options applyOptions, fillModes map[string]string, primaryMonitorID uint32, displayMode byte) error {
	return mm.applyByWLOutput(monitorMap)
}

func (mm *kMonitorManager) applyByWLOutput(monitorMap map[uint32]*Monitor) error {
	var args_enable []string
	var args_disable []string
	for _, monitor := range monitorMap {
		trans := int32(randrRotationToTransform(int(monitor.Rotation)))
		uuid := mm.mig.getUuidById(monitor.ID)
		if uuid == "" {
			logger.Warningf("get monitor %d uuid failed", monitor.ID)
			return fmt.Errorf("get monitor %d uuid failed", monitor.ID)
		}
		logger.Debugf("apply name: %q, uuid: %q, enabled: %v, x: %v, y: %v, mode: %+v, trans:%v",
			monitor.Name, uuid, monitor.Enabled, monitor.X, monitor.Y, monitor.CurrentMode, trans)

		if !monitor.Enabled {
			args_disable = append(args_disable, uuid, "0",
				strconv.Itoa(int(monitor.X)), strconv.Itoa(int(monitor.Y)),
				strconv.Itoa(int(monitor.CurrentMode.Width)),
				strconv.Itoa(int(monitor.CurrentMode.Height)),
				strconv.Itoa(int(monitor.CurrentMode.Rate*1000)),
				strconv.Itoa(int(trans)))
		} else {
			args_enable = append(args_enable, uuid, "1",
				strconv.Itoa(int(monitor.X)), strconv.Itoa(int(monitor.Y)),
				strconv.Itoa(int(monitor.CurrentMode.Width)),
				strconv.Itoa(int(monitor.CurrentMode.Height)),
				strconv.Itoa(int(monitor.CurrentMode.Rate*1000)),
				strconv.Itoa(int(trans)))

		}
	}
	
	if len(args_enable) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		cmdline := exec.CommandContext(ctx, "/usr/bin/dde_wloutput", "set")
		cmdline.Args = append(cmdline.Args, args_enable...)
		logger.Info("cmd line args_enable:", cmdline.Args)

		data, err := cmdline.CombinedOutput()
		cancel()
		// ignore timeout signal
		if err != nil && !strings.Contains(err.Error(), "killed") {
			logger.Warningf("%s(%s)", string(data), err)
			return err
		}
		// wait request done
		//time.Sleep(time.Millisecond * 500)
	}
	if len(args_disable) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		cmdline := exec.CommandContext(ctx, "/usr/bin/dde_wloutput", "set")
		cmdline.Args = append(cmdline.Args, args_disable...)
		logger.Info("cmd line args_disable:", cmdline.Args)

		data, err := cmdline.CombinedOutput()
		cancel()
		// ignore timeout signal
		if err != nil && !strings.Contains(err.Error(), "killed") {
			logger.Warningf("%s(%s)", string(data), err)
			return err
		}
		// wait request done
		//time.Sleep(time.Millisecond * 500)
	}
	return nil
}

func (mm *kMonitorManager) handleOutputAdded(outputInfo *KOutputInfo) {
	logger.Debugf("OutputAdded %#v", outputInfo)

	mm.mu.Lock()
	monitorInfo := outputInfo.toMonitorInfo(mm)
	mm.monitorMap[outputInfo.id] = monitorInfo
	mm.mu.Unlock()

	if mm.hooks != nil {
		mm.hooks.handleMonitorAdded(monitorInfo)
	}
}

func (mm *kMonitorManager) handleOutputChanged(outputInfo *KOutputInfo) {
	logger.Debugf("OutputChanged %#v", outputInfo)

	mm.mu.Lock()
	monitorInfo := outputInfo.toMonitorInfo(mm)
	mm.monitorMap[outputInfo.id] = monitorInfo
	primary := mm.primary
	mm.mu.Unlock()

	if monitorInfo.ID == primary {
		mm.invokePrimaryRectChangedCb(monitorInfo)
	}

	if mm.hooks != nil {
		mm.hooks.handleMonitorChanged(monitorInfo)
	}

	// TODO
	//if m.checkKwinMonitorData(monitor, kinfo) == true {
	//	m.updateMonitor(monitor, kinfo)
	//}
}

func (mm *kMonitorManager) handleOutputRemoved(outputInfo *KOutputInfo) {
	logger.Debugf("OutputRemoved %#v", outputInfo)

	mm.mu.Lock()
	delete(mm.monitorMap, outputInfo.id)
	mm.mu.Unlock()

	if mm.hooks != nil {
		mm.hooks.handleMonitorRemoved(outputInfo.id)
	}
}

func (mm *kMonitorManager) listenDBusSignals() {
	mm.management.InitSignalExt(mm.sessionSigLoop, true)

	_, err := mm.management.ConnectOutputAdded(func(output string) {
		outputInfo, err := unmarshalOutputInfo(output)
		if err != nil {
			logger.Warning(err)
			return
		}
		outputInfo.setId(mm.mig)
		mm.handleOutputAdded(outputInfo)

	})
	if err != nil {
		logger.Warning(err)
	}

	_, err = mm.management.ConnectOutputChanged(func(output string) {
		outputInfo, err := unmarshalOutputInfo(output)
		if err != nil {
			logger.Warning(err)
			return
		}

		outputInfo.setId(mm.mig)
		// 根据选择 transform 修正宽和高，应该只有 OutputChanged 事件才有必要。
		swapWidthHeightWithRotationInt32(outputInfo.rotation(), &outputInfo.Width, &outputInfo.Height)
		mm.handleOutputChanged(outputInfo)

	})
	if err != nil {
		logger.Warning(err)
	}

	_, err = mm.management.ConnectOutputRemoved(func(output string) {
		outputInfo, err := unmarshalOutputInfo(output)
		if err != nil {
			logger.Warning(err)
			return
		}
		outputInfo.setId(mm.mig)
		mm.handleOutputRemoved(outputInfo)
	})
	if err != nil {
		logger.Warning(err)
	}
}

func (mm *kMonitorManager) invokePrimaryRectChangedCb(monitorInfo *MonitorInfo) {
	if mm.hooks != nil {
		mm.hooks.handlePrimaryRectChanged(monitorInfo)
	}
}

func (mm *kMonitorManager) setMonitorPrimary(monitorId uint32) error {
	logger.Debug("mm.setMonitorPrimary", monitorId)

	mm.mu.Lock()
	monitor := mm.getMonitorNoLock(monitorId)
	if monitor == nil {
		mm.mu.Unlock()
		return fmt.Errorf("invalid monitor id %v", monitorId)
	}

	mm.primary = monitorId
	mm.mu.Unlock()

	mm.syncPrimary()

	mm.invokePrimaryRectChangedCb(monitor)
	return nil
}

func (mm *kMonitorManager) showCursor(show bool) error {
	return nil
}

func (mm *kMonitorManager) HandleEvent(ev interface{}) {

}

func (mm *kMonitorManager) HandleScreenChanged(e *randr.ScreenChangeNotifyEvent) (cfgTsChanged bool) {
	return false
}
