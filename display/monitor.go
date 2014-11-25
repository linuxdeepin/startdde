package display

import "github.com/BurntSushi/xgb/randr"
import "pkg.linuxdeepin.com/lib/dbus"
import "fmt"
import "strings"
import "math"
import "runtime"
import "sort"

const joinSeparator = "="

type Monitor struct {
	cfg     *ConfigMonitor
	Outputs []string

	BestMode Mode

	IsComposited bool
	Name         string
	FullName     string

	X int16
	Y int16

	Opened   bool
	Rotation uint16
	Reflect  uint16

	CurrentMode Mode

	Width  uint16
	Height uint16
}

func (m *Monitor) ListRotations() []uint16 {
	set := newSetUint16()
	for _, oname := range m.Outputs {
		op := GetDisplayInfo().QueryOutputs(oname)
		if op == 0 {
			continue
		}
		oinfo, err := randr.GetOutputInfo(xcon, op, LastConfigTimeStamp).Reply()
		if err == nil && oinfo.Connection == randr.ConnectionConnected && oinfo.Crtc != 0 {
			cinfo, err := randr.GetCrtcInfo(xcon, oinfo.Crtc, LastConfigTimeStamp).Reply()
			if err != nil {
				continue
			}
			set.Add(parseRotations(cinfo.Rotations)...)
		}
	}
	r := set.Set()
	sort.Sort(uint16Slice(r))
	return r
}
func (m *Monitor) ListReflect() []uint16 {
	set := newSetUint16()
	for _, oname := range m.Outputs {
		op := GetDisplayInfo().QueryOutputs(oname)
		if op == 0 {
			continue
		}
		oinfo, err := randr.GetOutputInfo(xcon, op, LastConfigTimeStamp).Reply()
		if err == nil && oinfo.Connection == randr.ConnectionConnected && oinfo.Crtc != 0 {
			cinfo, err := randr.GetCrtcInfo(xcon, oinfo.Crtc, LastConfigTimeStamp).Reply()
			if err != nil {
				continue
			}
			set.Add(parseReflects(cinfo.Rotations)...)
		}
	}
	r := set.Set()
	sort.Sort(uint16Slice(r))
	return r
}

func FindMatchedMode(minfoGroup ...[]Mode) ([]Mode, error) {
	countSet := make(map[string]int)
	tmpSet := make(map[string]Mode)

	for _, minfos := range minfoGroup {
		for _, minfo := range mergeModeInfo(minfos) {
			wh := fmt.Sprintf("%d%d", minfo.Width, minfo.Height)
			countSet[wh] = countSet[wh] + 1
			tmpSet[wh] = minfo
		}
	}

	for wh, count := range countSet {
		if count < len(minfoGroup) {
			delete(tmpSet, wh)
		}
	}

	var sameModes []Mode
	for _, minfo := range tmpSet {
		sameModes = append(sameModes, minfo)
	}

	sort.Sort(Modes(sameModes))

	if len(sameModes) == 0 {
		return nil, fmt.Errorf("not found")
	}
	return sameModes, nil
}

func mergeModeInfo(minfos []Mode) []Mode {
	tmpSet := make(map[string]Mode)
	for _, minfo := range minfos {
		tmpSet[fmt.Sprintf("%d%d", minfo.Width, minfo.Height)] = minfo
	}
	var ret []Mode
	for _, minfo := range tmpSet {
		ret = append(ret, minfo)
	}
	return ret
}

func (m *Monitor) ListModes() []Mode {
	set := make(map[Mode]int)
	var allMode [][]Mode
	for _, oname := range m.Outputs {
		op := GetDisplayInfo().QueryOutputs(oname)
		if op == 0 {
			continue
		}
		oinfo, err := randr.GetOutputInfo(xcon, op, LastConfigTimeStamp).Reply()
		if err != nil {
			continue
		}
		var modeGroup []Mode
		for _, m := range oinfo.Modes {
			minfo := GetDisplayInfo().QueryModes(m)
			//TODO: handle different refresh rate but same width/height modes
			set[minfo] += 1
			modeGroup = append(modeGroup, minfo)
		}
		allMode = append(allMode, modeGroup)
	}
	modes, err := FindMatchedMode(allMode...)
	if err != nil {
		logger.Warning("ListModes failed:", err)
	}
	return modes
}

func (m *Monitor) changeRotation(v uint16) error {
	switch v {
	case 1, 2, 4, 8:
		break
	default:
		err := fmt.Errorf("changeRotation with invalid value ", v)
		logger.Error(err)
		return err
	}
	m.setPropRotation(v)
	return nil
}

func (m *Monitor) SetRotation(v uint16) error {
	if err := m.changeRotation(v); err != nil {
		return err
	}
	m.cfg.Rotation = v
	GetDisplay().detectChanged()
	return nil
}

func (m *Monitor) changeReflect(v uint16) error {
	switch v {
	case 0, 16, 32, 48:
		break
	default:
		err := fmt.Errorf("SetReflect with invalid value ", v)
		logger.Error(err)
		return err
	}
	m.setPropReflect(v)
	return nil
}
func (m *Monitor) SetReflect(v uint16) error {
	if err := m.changeReflect(v); err != nil {
		return err
	}
	m.cfg.Rotation = v
	GetDisplay().detectChanged()
	return nil
}

func (m *Monitor) changePos(x, y int16) error {
	m.setPropPos(x, y)
	return nil
}
func (m *Monitor) SetPos(x, y int16) error {
	if err := m.changePos(x, y); err != nil {
		return err
	}
	m.cfg.X, m.cfg.Y = x, y
	GetDisplay().detectChanged()
	return nil
}

func (m *Monitor) canCloseMonitor() bool {
	n := 0
	dpy := GetDisplay()

	dpy.rLockMonitors()
	defer dpy.rUnlockMonitors()
	for _, _m := range dpy.Monitors {
		if _m != m && _m.Opened {
			n++
		}
	}
	return n > 0
}
func (m *Monitor) changeSwitchOn(v bool) error {
	if v == false && m.canCloseMonitor() == false {
		return fmt.Errorf("reject close the last opened Output %s", m.Name)
	}
	m.setPropOpened(v)
	return nil
}

func (m *Monitor) SwitchOn(v bool) error {
	if err := m.changeSwitchOn(v); err != nil {
		return err
	}
	m.cfg.Enabled = v
	dpy := GetDisplay()
	dpy.cfg.ensureValid(dpy)
	dpy.detectChanged()
	return nil
}

func (m *Monitor) changeMode(id randr.Mode) (*Mode, error) {
	minfo := GetDisplayInfo().QueryModes(id)
	if minfo.ID == 0 {
		return nil, fmt.Errorf("can't find this mode:%d", id)
	}

	m.setPropCurrentMode(minfo)

	w, h := parseRotationSize(m.Rotation, minfo.Width, minfo.Height)

	m.setPropWidth(w)
	m.setPropHeight(h)

	return &minfo, nil
}

func (m *Monitor) SetMode(id uint32) error {
	mode, err := m.changeMode(randr.Mode(id))
	if err != nil {
		return err
	}
	m.cfg.Width, m.cfg.Height, m.cfg.RefreshRate = mode.Width, mode.Height, mode.Rate
	GetDisplay().detectChanged()
	return nil
}

func (m *Monitor) generateShell() string {
	code := " "
	names := strings.Split(m.Name, joinSeparator)
	for _, name := range names {
		code = fmt.Sprintf("%s --output %s", code, name)
		if !m.cfg.Enabled {
			code = fmt.Sprintf(" %s --off", code)
			continue
		}

		code = fmt.Sprintf("%s --mode %dx%d --rate %f", code, m.cfg.Width, m.cfg.Height, m.cfg.RefreshRate)

		code = fmt.Sprintf(" %s --pos %dx%d", code, m.cfg.X, m.cfg.Y)

		code = fmt.Sprintf("%s --scale 1x1", code)

		switch m.cfg.Rotation {
		case randr.RotationRotate90:
			code = fmt.Sprintf("%s --rotate left", code)
		case randr.RotationRotate180:
			code = fmt.Sprintf("%s --rotate inverted", code)
		case randr.RotationRotate270:
			code = fmt.Sprintf("%s --rotate right", code)
		default:
			code = fmt.Sprintf("%s --rotate normal", code)
		}
		switch m.cfg.Reflect {
		case randr.RotationReflectX:
			code = fmt.Sprintf("%s --reflect x", code)
		case randr.RotationReflectY:
			code = fmt.Sprintf("%s --reflect y", code)
		case randr.RotationReflectX | randr.RotationReflectY:
			code = fmt.Sprintf("%s --reflect xy", code)
		default:
			code = fmt.Sprintf("%s --reflect normal", code)
		}
	}
	return code + " "
}

func (m *Monitor) updateInfo() {
	op := GetDisplayInfo().QueryOutputs(m.Outputs[0])
	oinfo, err := randr.GetOutputInfo(xcon, op, LastConfigTimeStamp).Reply()
	if err != nil {
		logger.Warning(m.Name, "updateInfo error:", err, "outpu:", op)
		return
	}
	if oinfo.Crtc == 0 {
		m.changeSwitchOn(false)
	} else {
		m.changeSwitchOn(true)
		cinfo, err := randr.GetCrtcInfo(xcon, oinfo.Crtc, LastConfigTimeStamp).Reply()
		if err != nil {
			logger.Warning("UpdateInfo Failed:", (m.Name), oinfo.Crtc, err)
			return
		}
		rotation, reflect := parseRandR(cinfo.Rotation)
		m.changeRotation(rotation)
		m.changeReflect(reflect)

		//Note: changeMode should after changeRotation
		m.changePos(cinfo.X, cinfo.Y)
		m.changeMode(cinfo.Mode)
	}
}

func NewMonitor(dpy *Display, info *ConfigMonitor) *Monitor {
	m := &Monitor{}
	m.cfg = info
	m.Name = info.Name
	m.setPropPos(info.X, info.Y)
	m.setPropWidth(info.Width)
	m.setPropHeight(info.Height)
	m.setPropRotation(info.Rotation)
	m.setPropReflect(info.Reflect)
	m.setPropOpened(info.Enabled)

	m.Outputs = info.Outputs
	runtime.SetFinalizer(m, func(o interface{}) { dbus.UnInstallObject(m) })
	m.setPropIsComposited(len(m.Outputs) > 1)

	m.FullName = m.Name

	if m.IsComposited {
		best := Mode{}
		for _, mode := range m.ListModes() {
			if mode.Width+mode.Height > best.Width+best.Height {
				best = mode
			}
		}
		m.setPropBestMode(best)
		m.setPropCurrentMode(best)
	} else {
		op := GetDisplayInfo().QueryOutputs(m.Outputs[0])
		mode := queryBestMode(op)
		modeinfo := GetDisplayInfo().QueryModes(mode)
		m.setPropBestMode(modeinfo)
		m.setPropCurrentMode(m.queryModeBySize(info.Width, info.Height))
	}

	return m
}

func (m *Monitor) queryModeBySize(width, height uint16) Mode {
	for _, m := range m.ListModes() {
		if m.Width == width && m.Height == height {
			return m
		}
	}
	logger.Error("queryModeBySize error:", m.Name, width, height, m.ListModes())
	return Mode{}
}

func (m *Monitor) ensureSize(w, h uint16) {
	//find the nearest mode
	delta := float64(w + h)
	modeID := uint32(0)
	for _, mInfo := range m.ListModes() {
		t := math.Abs(float64((mInfo.Width + mInfo.Height) - (w + h)))
		if t <= delta {
			delta = t
			modeID = mInfo.ID
			if modeID == m.BestMode.ID {
				break
			}
		}
	}
	if modeID != 0 {
		m.changeMode(randr.Mode(modeID))
		if delta != 0 {
			logger.Warningf("Can't ensureSize(%s) to %d %d", m.Name, w, h)
		}
	}
}
