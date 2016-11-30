package brightness

import (
	"dbus/com/deepin/daemon/helper/backlight"
	"fmt"
	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/randr"
	"github.com/BurntSushi/xgb/xproto"
)

const (
	SetterAuto      = "auto"
	SetterGamma     = "gamma"
	SetterBacklight = "backlight"
	SetterRaw       = "backlight-raw"
	SetterPlatform  = "backlight-platform"
	SetterFirmware  = "backlight-firmware"
)

var (
	helper *backlight.Backlight
)

func init() {
	var err error
	helper, err = backlight.NewBacklight("com.deepin.daemon.helper.Backlight",
		"/com/deepin/daemon/helper/Backlight")
	if err != nil {
		fmt.Println("New backlight helper failed:", err)
	}
}

func Set(value float64, setter string, output uint32, conn *xgb.Conn) error {
	if value < 0.01 {
		value = 0.01
	} else if value > 1 {
		value = 1
	}

	switch setter {
	case SetterGamma:
		return setGammaSize(value, randr.Output(output), conn)
	case SetterBacklight, SetterRaw, SetterPlatform, SetterFirmware:
		return setBacklight(value, setter)
	}

	if helper != nil && HasPropBacklight(output, conn) {
		return setBacklight(value, SetterBacklight)
	}

	return setGammaSize(value, randr.Output(output), conn)
}

func Get(setter string, output uint32, conn *xgb.Conn) (float64, error) {
	if helper == nil || !HasPropBacklight(output, conn) {
		// TODO: get brightness from xrandr
		return 1, nil
	}
	return doGetBacklight(setter)
}

func GetMax(setter string) (int32, error) {
	if helper == nil {
		return 0, fmt.Errorf("Failed to initialize backlight helper")
	}

	sysPath := getSysPath(setter)
	if len(sysPath) == 0 {
		return 0, fmt.Errorf("No backlight syspath found")
	}

	return helper.GetMaxBrightness(sysPath)
}

func HasPropBacklight(output uint32, conn *xgb.Conn) bool {
	op := randr.Output(output)
	prop, err := randr.GetOutputProperty(conn, op, backlightAtom, xproto.AtomAny,
		0, 1, false, false).Reply()
	if err != nil {
		fmt.Printf("Get output(%v) backlight prop failed: %v\n", op, err)
		return false
	}

	pinfo, err := randr.QueryOutputProperty(conn, op, backlightAtom).Reply()
	if err != nil {
		fmt.Printf("Qeury output(%v) backlight prop failed: %v\n", op, err)
		return false
	}

	if prop.NumItems != 1 || !pinfo.Range || len(pinfo.ValidValues) != 2 {
		return false
	}
	return true
}

func doGetBacklight(setter string) (float64, error) {
	if helper == nil {
		return 0, fmt.Errorf("Failed to initialize backlight helper")
	}

	sysPath := getSysPath(setter)
	if len(sysPath) == 0 {
		return 0, fmt.Errorf("No backlight syspath found")
	}

	v, err := helper.GetBrightness(sysPath)
	if err != nil {
		return 0, err
	}

	max, err := helper.GetMaxBrightness(sysPath)
	if err != nil {
		return 0, err
	}

	if max < 1 {
		return 0, fmt.Errorf("Failed to get max brightness for %s", sysPath)
	}

	return float64(v) / float64(max), nil
}

func setGammaSize(value float64, output randr.Output, conn *xgb.Conn) error {
	oinfo, err := randr.GetOutputInfo(conn, output, xproto.TimeCurrentTime).Reply()
	if err != nil {
		fmt.Printf("Get output(%v) failed: %v\n", output, err)
		return err
	}

	if oinfo.Crtc == 0 || oinfo.Connection != randr.ConnectionConnected {
		fmt.Printf("Output(%s) no crtc or disconnected\n", string(oinfo.Name))
		return fmt.Errorf("Output(%v) unready", output)
	}

	gamma, err := randr.GetCrtcGammaSize(conn, oinfo.Crtc).Reply()
	if err != nil {
		fmt.Printf("Failed to get gamma size: %v\n", err)
		return err
	}

	if gamma.Size == 0 {
		return fmt.Errorf("The output(%v) has invalid gamma size", output)
	}

	red, green, blue := genGammaRamp(gamma.Size, value)
	return randr.SetCrtcGammaChecked(conn, oinfo.Crtc, gamma.Size,
		red, green, blue).Check()
}

func genGammaRamp(size uint16, brightness float64) (red, green, blue []uint16) {
	red = make([]uint16, size)
	green = make([]uint16, size)
	blue = make([]uint16, size)

	step := uint16(65535 / uint32(size))
	for i := uint16(0); i < size; i++ {
		red[i] = uint16(float64(step*i) * brightness)
		green[i] = uint16(float64(step*i) * brightness)
		blue[i] = uint16(float64(step*i) * brightness)
	}
	return
}

func setBacklight(value float64, setter string) error {
	if helper == nil {
		return fmt.Errorf("Failed to initialize backlight helper")
	}

	if setter != SetterBacklight {
		return doSetByBacklight(value, setter)
	}

	for _, p := range getSysPathList() {
		max, err := helper.GetMaxBrightness(p)
		if err != nil || max < 1 {
			continue
		}

		helper.SetBrightness(p, int32(float64(max)*value))
	}
	return nil
}

func doSetByBacklight(value float64, setter string) error {
	if isBrightnessEqual(value, setter) {
		return nil
	}

	sysPath := getSysPath(setter)
	if len(sysPath) == 0 {
		return fmt.Errorf("No backlight syspath found")
	}

	max, err := helper.GetMaxBrightness(sysPath)
	if err != nil {
		return err
	}
	if max < 1 {
		return fmt.Errorf("Failed to get max brightness for %s", sysPath)
	}

	return helper.SetBrightness(sysPath, int32(float64(max)*value))
}

func getSysPath(setter string) string {
	var ty string
	switch setter {
	case SetterBacklight, SetterRaw:
		ty = "raw"
	case SetterPlatform:
		ty = "platform"
	case SetterFirmware:
		ty = "firmware"
	default:
		ty = "raw"
	}

	sysPath, _ := helper.GetSysPathByType(ty)
	return sysPath
}

func getSysPathList() []string {
	list, _ := helper.ListSysPath()
	return list
}

func isBrightnessEqual(value float64, setter string) bool {
	cur, err := doGetBacklight(setter)
	if err != nil {
		return false
	}
	if value < cur+0.001 && value > cur-0.001 {
		return true
	}
	return false
}

var backlightAtom xproto.Atom = 0

func getBacklightAtom(conn *xgb.Conn) xproto.Atom {
	if backlightAtom != 0 {
		return backlightAtom
	}

	var name = "backlight"
	reply, err := xproto.InternAtom(conn, false, uint16(len(name)), name).Reply()
	if err != nil {
		return xproto.AtomNone
	}
	backlightAtom = reply.Atom
	return backlightAtom
}
