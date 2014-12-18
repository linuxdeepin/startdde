package display

import "github.com/BurntSushi/xgb/xproto"
import "github.com/BurntSushi/xgb/randr"
import "github.com/BurntSushi/xgb/render"
import "github.com/BurntSushi/xgb"
import "dbus/com/deepin/daemon/helper/backlight"

import "os/exec"
import "math"

var backlightAtom = getAtom(xcon, "Backlight")

func runCode(code string) bool {
	err := exec.Command("sh", "-c", code).Run()
	if err != nil {
		logger.Debug("Run", code, "failed:", err)
		return false
	} else {
		logger.Debug("RunCodeOK:", code)
		return true
	}
}

func runCodeAsync(code string) bool {
	cmd := exec.Command("sh", "-c", code)
	err := cmd.Start()
	if err != nil {
		logger.Debug("Run", code, "failed:", err)
		return false
	} else {
		logger.Debug("RunCodeOK:", code)
	}
	go cmd.Wait()
	return true
}

func getAtom(c *xgb.Conn, name string) xproto.Atom {
	r, err := xproto.InternAtom(c, false, uint16(len(name)), name).Reply()
	if err != nil {
		return xproto.AtomNone
	}
	return r.Atom
}
func queryAtomName(c *xgb.Conn, atom xproto.Atom) string {
	r, err := xproto.GetAtomName(c, atom).Reply()
	if err != nil {
		return ""
	}
	return r.Name

}

var (
	edidAtom    = getAtom(xcon, "EDID")
	borderAtom  = getAtom(xcon, "Border")
	unknownAtom = getAtom(xcon, "unknown")
)

func getOutputName(data [128]byte, defaultName string) string {
	timingDescriptor := data[36:]
	for i := 0; i < 4; i++ {
		block := timingDescriptor[i*18 : (i+1)*18]
		if block[3] == 0xfc { //descriptor type == Monitor Name
			data := block[5:]
			for i := 0; i < 13; i++ {
				if data[i] == 0xa {
					return string(data[:i])
				}
			}
		}
	}
	return defaultName
}

type Mode struct {
	ID     uint32
	Width  uint16
	Height uint16
	Rate   float64
}
type Modes []Mode

func (m Modes) Len() int {
	return len(m)
}
func (m Modes) Less(i, j int) bool {
	if m[i].Width == m[j].Width && m[i].Height == m[j].Height {
		return m[i].Rate > m[j].Rate
	} else {
		return m[i].Width+m[i].Height > m[j].Width+m[j].Height
	}
}
func (m Modes) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

func buildMode(info randr.ModeInfo) Mode {
	vTotal := info.Vtotal

	if info.ModeFlags&randr.ModeFlagDoubleScan != 0 {
		vTotal *= 2
	}
	if info.ModeFlags&randr.ModeFlagInterlace != 0 {
		vTotal /= 2
	}

	rate := float64(info.DotClock) / float64(uint32(info.Htotal)*uint32(vTotal))
	rate = math.Floor(rate*10+0.5) / 10
	return Mode{info.Id, info.Width, info.Height, rate}
}

func parseRandR(randr uint16) (uint16, uint16) {
	rotation := randr & 0xf
	reflect := randr & 0xf0
	switch rotation {
	case 1, 2, 4, 8:
		break
	default:
		logger.Warning("invalid rotation value", rotation, randr)
		rotation = 1
	}
	switch reflect {
	case 0, 16, 32, 48:
		break
	default:
		logger.Warning("invalid reflect value", reflect, randr)
		reflect = 0
	}
	return rotation, reflect
}

func parseRotations(rotations uint16) (ret []uint16) {
	if rotations&randr.RotationRotate0 == randr.RotationRotate0 {
		ret = append(ret, randr.RotationRotate0)
	}
	if rotations&randr.RotationRotate90 == randr.RotationRotate90 {
		ret = append(ret, randr.RotationRotate90)
	}
	if rotations&randr.RotationRotate180 == randr.RotationRotate180 {
		ret = append(ret, randr.RotationRotate180)
	}
	if rotations&randr.RotationRotate270 == randr.RotationRotate270 {
		ret = append(ret, randr.RotationRotate270)
	}
	return
}
func parseReflects(rotations uint16) (ret []uint16) {
	ret = append(ret, 0) //the normal reflect

	if rotations&randr.RotationReflectX == randr.RotationReflectX {
		ret = append(ret, randr.RotationReflectX)
	}
	if rotations&randr.RotationReflectY == randr.RotationReflectY {
		ret = append(ret, randr.RotationReflectY)
	}

	if (rotations&randr.RotationReflectX == randr.RotationReflectX) && (rotations&randr.RotationReflectY == randr.RotationReflectY) {
		ret = append(ret, randr.RotationReflectX|randr.RotationReflectY)
	}

	return
}

func isCrtcConnected(c *xgb.Conn, crtc randr.Crtc) bool {
	cinfo, err := randr.GetCrtcInfo(c, crtc, 0).Reply()
	if err != nil {
		logger.Error(err)
		return false
	}
	if cinfo.Mode == 0 {
		return false
	} else if cinfo.NumOutputs == 0 {
		return false
	} else {
		oinfo, _ := randr.GetOutputInfo(c, cinfo.Outputs[0], 0).Reply()
		if oinfo.Crtc != crtc {
			return false
		}
	}
	return true
}

var setBacklight, getBacklight = func() (func(float64), func() float64) {
	helper, err := backlight.NewBacklight("com.deepin.daemon.helper.Backlight", "/com/deepin/daemon/helper/Backlight")
	if err != nil {
		logger.Warning("Can't create com.deepin.daemon.helper.Backlight")
		return func(v float64) {}, func() float64 { return 1 }
	}

	return func(v float64) {
			err := helper.SetBrightness(v)
			if err != nil {
				logger.Warning("setBacklight failed:", err)
			}
		},
		func() float64 {
			v, err := helper.GetBrightness()
			if err != nil {
				logger.Warning("getBacklight failed: ", err)
				return 1
			}
			return v
		}
}()

func supportedBacklight(c *xgb.Conn, output randr.Output) bool {
	prop, err := randr.GetOutputProperty(c, output, backlightAtom, xproto.AtomAny, 0, 1, false, false).Reply()
	pinfo, err := randr.QueryOutputProperty(c, output, backlightAtom).Reply()
	if err != nil || prop.NumItems != 1 || !pinfo.Range || len(pinfo.ValidValues) != 2 {
		return false
	}
	return true
}

func setBrightness(xcon *xgb.Conn, op randr.Output, v float64) {
	if v < 0.1 {
		logger.Warningf("setBrightness: %v is too small adjust to 0.1", v)
		v = 0.1
	}
	if v > 1 {
		logger.Warningf("setBrightness: %v is too big adjust to 1", v)
		v = 1
	}
	oinfo, err := randr.GetOutputInfo(xcon, op, LastConfigTimeStamp).Reply()
	if err != nil {
		logger.Errorf("GetOutputInfo(op=%d) failed: %v", op, err)
		return
	}
	if oinfo.Crtc == 0 || oinfo.Connection != randr.ConnectionConnected {
		logger.Warning("Try setBrightness at an unready Output ", string(oinfo.Name))
		return
	}
	gammaSize, err := randr.GetCrtcGammaSize(xcon, oinfo.Crtc).Reply()
	if err != nil {
		logger.Warning("GetCrtcGrammSize(crtc:%d) failed: %s", oinfo.Crtc, err.Error())
		return
	}
	if gammaSize.Size == 0 {
		logger.Warning("GetCrtcGrammSize == zero")
		return
	}
	red, green, blue := genGammaRamp(gammaSize.Size, v)
	randr.SetCrtcGamma(xcon, oinfo.Crtc, gammaSize.Size, red, green, blue)
}

func queryBestMode(op randr.Output) randr.Mode {
	oinfo, err := randr.GetOutputInfo(xcon, op, LastConfigTimeStamp).Reply()
	if err != nil {
		logger.Warning("can't find best mode for ", op, "(oinfo:", oinfo, ") (err:", err, ")")
		return 0
	}
	return oinfo.Modes[0]
}

func parseRotationSize(rotation, width, height uint16) (uint16, uint16) {
	if rotation == randr.RotationRotate90 || rotation == randr.RotationRotate270 {
		return height, width
	} else {
		return width, height
	}
}

func fixed2double(v render.Fixed) float32 {
	return float32(v) / 65536
}
func double2fixed(v float32) render.Fixed {
	return render.Fixed(v * 65536)
}

func genGammaRamp(size uint16, brightness float64) (red []uint16, green []uint16, blue []uint16) {
	red = make([]uint16, size)
	green = make([]uint16, size)
	blue = make([]uint16, size)

	step := uint16(65536 / uint32(size))
	for i := uint16(0); i < size; i++ {
		red[i] = uint16(float64(step*i) * brightness)
		green[i] = uint16(float64(step*i) * brightness)
		blue[i] = uint16(float64(step*i) * brightness)
	}
	return
}

func genTransformByScale(xScale float32, yScale float32) render.Transform {
	m := render.Transform{}
	m.Matrix11 = double2fixed(xScale)
	m.Matrix22 = double2fixed(yScale)
	m.Matrix33 = double2fixed(1)
	return m
}

func isOverlap(x1, y1 int16, w1, h1 uint16, x2, y2 int16, w2, h2 uint16) bool {
	if x1 == x2 && y1 == y2 && w1 == w2 && h1 == h2 {
		return true
	}

	var contain = func(px int16, py int16) bool {
		if px > x1 && px < x1+int16(w1) && py > y1 && py < y1+int16(h1) {
			return true
		} else {
			return false
		}
	}
	if contain(x2, y2) {
		return true
	}
	if contain(x2+int16(w2), y2) {
		return true
	}
	if contain(x2, y2+int16(h2)) {
		return true
	}
	if contain(x2+int16(w2), y2+int16(h2)) {
		return true
	}
	return false
}

type uint16Slice []uint16

func (s uint16Slice) Len() int {
	return len(s)
}
func (s uint16Slice) Less(i, j int) bool {
	return s[i] < s[j]
}
func (s uint16Slice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

type setUint16 map[uint16]bool

func newSetUint16() setUint16 {
	return make(map[uint16]bool)
}
func (s setUint16) Add(vs ...uint16) {
	for _, v := range vs {
		s[v] = true
	}
}
func (s setUint16) Set() []uint16 {
	var r []uint16
	for k, _ := range s {
		r = append(r, k)
	}
	return r
}

type setMode map[Mode]bool

func newSetMode() setMode {
	return make(map[Mode]bool)
}
func (s setMode) Add(vs ...Mode) {
	for _, v := range vs {
		s[v] = true
	}
}
func (s setMode) Set() []Mode {
	var r []Mode
	for k, _ := range s {
		r = append(r, k)
	}
	return r
}
