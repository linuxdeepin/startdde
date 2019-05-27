/*
 * Copyright (C) 2017 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
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
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/linuxdeepin/go-x11-client/ext/randr"
	"pkg.deepin.io/dde/api/drandr"
)

type MonitorBaseInfo struct {
	UUID        string // sum md5 of name and modes, for config
	Name        string
	Enabled     bool
	X           int16
	Y           int16
	Width       uint16
	Height      uint16
	Rotation    uint16
	Reflect     uint16
	RefreshRate float64

	nameType string
}
type MonitorBaseInfos []*MonitorBaseInfo

type MonitorInfo struct {
	locker sync.Mutex
	cfg    *MonitorBaseInfo

	// MonitorBaseInfo
	// dbus unsupported inherit
	ID             uint32
	Name           string
	Enabled        bool
	Connected      bool
	X              int16
	Y              int16
	Width          uint16
	Height         uint16
	Rotation       uint16
	Reflect        uint16
	RefreshRate    float64
	Rotations      []uint16
	Reflects       []uint16
	BestMode       drandr.ModeInfo
	CurrentMode    drandr.ModeInfo
	Modes          drandr.ModeInfos
	PreferredModes drandr.ModeInfos
}
type MonitorInfos []*MonitorInfo

func (m *MonitorInfo) generateCommandline(primary string, auto bool) string {
	m.locker.Lock()
	defer m.locker.Unlock()
	if !m.Connected {
		return ""
	}

	var cmd = ""
	cmd += " --output " + m.cfg.Name
	if !m.cfg.Enabled {
		cmd += " --off"
		return cmd
	}

	if auto {
		cmd += " --auto"
	}

	if m.cfg.Name == primary {
		cmd += " --primary"
	}
	cmd += fmt.Sprintf(" --mode %dx%d", m.cfg.Width, m.cfg.Height)
	cmd += fmt.Sprintf(" --pos %dx%d", m.cfg.X, m.cfg.Y)
	// NOTE: do not set rate, because set rate may cause xrandr to report
	// configure crtc failed error
	var ro = "normal"
	switch m.cfg.Rotation {
	case randr.RotationRotate90:
		ro = "left"
	case randr.RotationRotate180:
		ro = "inverted"
	case randr.RotationRotate270:
		ro = "right"
	}
	cmd += " --rotate " + ro

	var re = "normal"
	switch m.cfg.Reflect {
	case randr.RotationReflectX:
		re = "x"
	case randr.RotationReflectY:
		re = "y"
	case randr.RotationReflectX | randr.RotationReflectY:
		re = "xy"
	}
	cmd += " --reflect " + re
	return cmd
}

func (m *MonitorInfo) canDisable() bool {
	connected := _dpy.Monitors.listConnected()
	var count = 0
	for _, v := range connected {
		if !v.Enabled {
			continue
		}
		count++
	}
	return count > 1
}

func (m *MonitorInfo) doEnable(enabled bool) error {
	if !enabled && !m.canDisable() {
		return fmt.Errorf("Reject closed the last output")
	}
	m.setPropEnabled(enabled)
	return nil
}

func (m *MonitorInfo) queryMode(v uint32) drandr.ModeInfo {
	for _, info := range m.Modes {
		if info.Id == v {
			return info
		}
	}
	return drandr.ModeInfo{}
}

func (m *MonitorInfo) doSetMode(v uint32) error {
	info := m.queryMode(v)
	if info.Id != v {
		return fmt.Errorf("Invalid output mode: %v", v)
	}
	m.setPropCurrentMode(info)
	w, h := parseModeByRotation(info.Width, info.Height, m.Rotation)
	m.setPropWidth(w)
	m.setPropHeight(h)
	m.setPropRefreshRate(info.Rate)
	return nil
}

func (m *MonitorInfo) doSetModeBySize(w, h uint16) error {
	matches := m.Modes.QueryBySize(w, h)
	if len(matches) == 0 {
		logger.Warning("Invalid mode size:", w, h)
		return fmt.Errorf("The mode size %dx%d invalid", w, h)
	}

	// only set the first mode
	return m.doSetMode(matches[0].Id)
}

func (m *MonitorInfo) doSetRefreshRate(rate float64) error {
	matches := m.Modes.QueryBySize(m.Width, m.Height)
	if len(matches) == 0 || !matches.HasRefreshRate(rate) {
		return fmt.Errorf("Invalid refresh rate: %v", rate)
	}
	m.setPropRefreshRate(rate)
	return nil
}

func (m *MonitorInfo) doSetPosition(x, y int16) {
	m.setPropX(x)
	m.setPropY(y)
}

func (m *MonitorInfo) validRotation(v uint16) bool {
	for _, r := range m.Rotations {
		if r == v {
			return true
		}
	}
	return false
}

func (m *MonitorInfo) doSetRotation(v uint16) error {
	if !m.validRotation(v) {
		return fmt.Errorf("Invalid rotation valid: %v", v)
	}
	m.setPropRotation(v)
	return nil
}

func (m *MonitorInfo) validReflect(v uint16) bool {
	for _, r := range m.Reflects {
		if r == v {
			return true
		}
	}
	return false
}

func (m *MonitorInfo) doSetReflect(v uint16) error {
	if !m.validReflect(v) {
		return fmt.Errorf("Invalid reflect valid: %v", v)
	}
	m.setPropReflect(v)
	return nil
}

func toMonitorBaseInfo(output drandr.OutputInfo, uuid string) MonitorBaseInfo {
	var info = MonitorBaseInfo{
		UUID:     uuid,
		Name:     output.Name,
		Enabled:  output.Connection,
		X:        output.Crtc.X,
		Y:        output.Crtc.Y,
		Width:    output.Crtc.Width,
		Height:   output.Crtc.Height,
		Rotation: output.Crtc.Rotation,
		Reflect:  output.Crtc.Reflect,
		nameType: strings.ToLower(numReg.ReplaceAllString(output.Name, "")),
	}
	return info
}

func parseModeByRotation(width, height, rotation uint16) (uint16, uint16) {
	switch rotation {
	case randr.RotationRotate90, randr.RotationRotate270:
		return height, width
	default:
		return width, height
	}
}

func (ms MonitorInfos) get(id string) *MonitorInfo {
	for _, m := range ms {
		if m.cfg.UUID == id {
			return m
		}
	}
	return nil
}

func (ms MonitorInfos) getByName(name string) *MonitorInfo {
	for _, m := range ms {
		if m.Name == name {
			return m
		}
	}
	return nil
}

func (ms MonitorInfos) listConnected() MonitorInfos {
	var list MonitorInfos
	for _, m := range ms {
		if !m.Connected {
			continue
		}
		list = append(list, m)
	}
	return list
}

func (ms MonitorInfos) numberOfConnected() int {
	var cnt int = 0
	for _, m := range ms {
		if m.Connected {
			cnt += 1
		}
	}
	return cnt
}

func (ms MonitorInfos) canBePrimary(name string) *MonitorInfo {
	for _, m := range ms {
		if m.Name == name && m.Connected && m.Enabled {
			return m
		}
	}
	return nil
}

func (ms MonitorInfos) sort(priority []string) MonitorInfos {
	if ms.numberOfConnected() < 2 {
		return ms
	}
	ms = ms.sortByNameType()
	ms = ms.sortByPriority(priority)
	return ms
}

// sortByNameType preference put the bulit-in output at the top,
// the extendable output should after the built-in
func (ms MonitorInfos) sortByNameType() MonitorInfos {
	var (
		builtin MonitorInfos
		vga     MonitorInfos
		dvi     MonitorInfos
		dp      MonitorInfos
		hdmi    MonitorInfos
		other   MonitorInfos
	)
	for _, info := range ms {
		switch info.cfg.nameType {
		case "edp", "lvds", "lcd", "dsi":
			builtin = append(builtin, info)
		case "vga":
			vga = append(vga, info)
		case "dvi":
			dvi = append(dvi, info)
		case "dp":
			dp = append(dp, info)
		case "hdmi":
			hdmi = append(hdmi, info)
		default:
			other = append(other, info)
		}
	}

	var ret MonitorInfos
	if len(builtin) != 0 {
		ret = append(ret, builtin...)
	}
	if len(vga) != 0 {
		ret = append(ret, vga...)
	}
	if len(dvi) != 0 {
		ret = append(ret, dvi...)
	}
	if len(dp) != 0 {
		ret = append(ret, dp...)
	}
	if len(hdmi) != 0 {
		ret = append(ret, hdmi...)
	}
	if len(other) != 0 {
		ret = append(ret, other...)
	}
	return ret
}

func (ms MonitorInfos) sortByPriority(priority []string) MonitorInfos {
	if len(priority) == 0 {
		return ms
	}

	var ret MonitorInfos
	for _, v := range priority {
		info := ms.getByName(v)
		if info == nil {
			continue
		}
		ret = append(ret, info)
	}
	if len(ret) == 0 {
		return ms
	}

	for _, info := range ms {
		if tmp := ret.getByName(info.Name); tmp != nil {
			continue
		}
		ret = append(ret, info)
	}
	return ret
}

// see also: gnome-desktop/libgnome-desktop/gnome-rr.c
//           '_gnome_rr_output_name_is_builtin_display'
func isBuiltinOutput(name string) bool {
	name = strings.ToLower(name)
	switch {
	case strings.HasPrefix(name, "vga"):
		return false
	case strings.HasPrefix(name, "hdmi"):
		return false
	case strings.HasPrefix(name, "dvi"):
		return false

	case strings.HasPrefix(name, "lvds"):
		// Most drivers use an "LVDS" prefix
		return true
	case strings.HasPrefix(name, "lcd"):
		// fglrx uses "LCD" in some versions
		return true
	case strings.HasPrefix(name, "edp"):
		// eDP is for internal built-in panel connections
		return true
	case strings.HasPrefix(name, "dsi"):
		return true
	case name == "default":
		return true
	}
	return true
}

func (ms MonitorInfos) getMonitorsId() string {
	var ids []string
	for _, m := range ms {
		if !m.Connected {
			continue
		}
		ids = append(ids, m.cfg.UUID)
	}
	if len(ids) == 0 {
		return ""
	}
	sort.Strings(ids)
	return strings.Join(ids, monitorsIdDelimiter)
}

func (ms MonitorInfos) getBaseInfos() MonitorBaseInfos {
	var base MonitorBaseInfos
	for _, m := range ms {
		if !m.Connected {
			continue
		}
		base = append(base, m.cfg.Duplicate())
	}
	return base
}

func (ms MonitorInfos) genCommandline(primary string, auto bool) string {
	var cmd = "xrandr "
	for _, m := range ms {
		cmd += m.generateCommandline(primary, auto)
	}
	return cmd
}

func (ms MonitorInfos) isRotation() bool {
	// check all connected monitor whether rotate the same diretion
	var (
		init     bool = false
		rotation uint16
	)
	for _, m := range ms {
		if !m.Connected {
			continue
		}
		if !init {
			init = true
			rotation = m.Rotation
			continue
		}

		if rotation != m.Rotation {
			return false
		}
	}
	return true
}

func (infos MonitorInfos) FoundCommonModes() drandr.ModeInfos {
	var modeGroup []drandr.ModeInfos
	for _, m := range infos {
		if !m.Connected {
			continue
		}
		modeGroup = append(modeGroup, m.Modes)
	}

	return drandr.FindCommonModes(modeGroup...)
}

func (infos MonitorBaseInfos) String() string {
	data, _ := json.Marshal(infos)
	return string(data)
}

func (info *MonitorBaseInfo) Duplicate() *MonitorBaseInfo {
	return &MonitorBaseInfo{
		UUID:        info.UUID,
		Name:        info.Name,
		Enabled:     info.Enabled,
		X:           info.X,
		Y:           info.Y,
		Width:       info.Width,
		Height:      info.Height,
		Rotation:    info.Rotation,
		Reflect:     info.Reflect,
		RefreshRate: info.RefreshRate,
	}
}

func (info *MonitorBaseInfo) String() string {
	data, _ := json.Marshal(info)
	return string(data)
}
