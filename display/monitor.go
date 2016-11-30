package display

import (
	"encoding/json"
	"fmt"
	"github.com/BurntSushi/xgb/randr"
	"pkg.deepin.io/dde/api/drandr"
	"strings"
	"sync"
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
}
type MonitorBaseInfos []*MonitorBaseInfo

type MonitorInfo struct {
	locker sync.Mutex
	cfg    *MonitorBaseInfo

	// MonitorBaseInfo
	// dbus unsupported inherit
	uuid        string // sum md5 of name and modes, for config
	Name        string
	Enabled     bool
	Connected   bool
	X           int16
	Y           int16
	Width       uint16
	Height      uint16
	Rotation    uint16
	Reflect     uint16
	RefreshRate float64
	Rotations   []uint16
	Reflects    []uint16
	BestMode    drandr.ModeInfo
	CurrentMode drandr.ModeInfo
	Modes       drandr.ModeInfos
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
	cmd = fmt.Sprintf("%s --mode %dx%d --rate %.2f --pos %dx%d --scale 1x1", cmd,
		m.cfg.Width, m.cfg.Height, m.cfg.RefreshRate, m.cfg.X, m.cfg.Y)
	var ro string = "normal"
	switch m.cfg.Rotation {
	case randr.RotationRotate90:
		ro = "left"
	case randr.RotationRotate180:
		ro = "inverted"
	case randr.RotationRotate270:
		ro = "right"
	}
	cmd += " --rotate " + ro

	var re string = "normal"
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
	return len(_dpy.Monitors.listConnected()) > 1
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
		if m.uuid == id {
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

func (ms MonitorInfos) sort() MonitorInfos {
	if ms.numberOfConnected() < 2 {
		return ms
	}
	ms = ms.sortByNamType()
	// ms = ms.sortByPrimary(primary)
	return ms
}

// sortByNamType preference put the bulit-in output at the top,
// the extendable output should after the built-in
func (ms MonitorInfos) sortByNamType() MonitorInfos {
	var list MonitorInfos
	for _, m := range ms {
		if isBuiltinOuput(m.Name) {
			list = append(MonitorInfos{m}, list...)
		} else {
			list = append(list, m)
		}
	}
	return list
}

// sortByPrimary the primary output should at the first
func (ms MonitorInfos) sortByPrimary(primary string) MonitorInfos {
	if ms[0].Name == primary {
		return ms
	}

	var list MonitorInfos
	for _, m := range ms {
		if m.Name == primary {
			list = append(MonitorInfos{m}, list...)
		} else {
			list = append(list, m)
		}
	}
	return list
}

// see alse: gnome-desktop/libgnome-desktop/gnome-rr.c
//           '_gnome_rr_output_name_is_builtin_display'
func isBuiltinOuput(name string) bool {
	name = strings.ToLower(name)
	switch {
	case strings.Contains(name, "lvds"):
		// Most drivers use an "LVDS" prefix
		fallthrough
	case strings.Contains(name, "lcd"):
		// fglrx uses "LCD" in some versions
		fallthrough
	case strings.Contains(name, "edp"):
		// eDP is for internal built-in panel connections
		fallthrough
	case strings.Contains(name, "dsi"):
		return true
	}
	return false
}

func (ms MonitorInfos) getMonitorsId() string {
	var ids []string
	for _, m := range ms {
		if !m.Connected {
			continue
		}
		ids = append(ids, m.uuid)
	}
	if len(ids) == 0 {
		return ""
	}
	return strings.Join(ids, ",")
}

func (ms MonitorInfos) getBaseInfos() MonitorBaseInfos {
	var base MonitorBaseInfos
	for _, m := range ms {
		if !m.Connected {
			continue
		}
		base = append(base, m.cfg)
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

func (infos MonitorBaseInfos) String() string {
	data, _ := json.Marshal(infos)
	return string(data)
}

func (info *MonitorBaseInfo) String() string {
	data, _ := json.Marshal(info)
	return string(data)
}
