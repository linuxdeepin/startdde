package brightness

import (
	"dbus/com/deepin/daemon/helper/backlight"
	"errors"
	"fmt"
	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/randr"
	"github.com/BurntSushi/xgb/xproto"
	displayBl "pkg.deepin.io/lib/backlight/display"
)

const (
	SetterAuto      = "auto"
	SetterGamma     = "gamma"
	SetterBacklight = "backlight"
)

var helper *backlight.Backlight

func init() {
	var err error
	helper, err = backlight.NewBacklight("com.deepin.daemon.helper.Backlight",
		"/com/deepin/daemon/helper/Backlight")
	if err != nil {
		fmt.Println("New backlight helper failed:", err)
	}
}

func Set(value float64, setter string, outputId uint32, conn *xgb.Conn) error {
	if value < 0 {
		value = 0
	} else if value > 1 {
		value = 1
	}

	output := randr.Output(outputId)
	switch setter {
	case SetterBacklight:
		return setBacklight(value, output, conn)
	case SetterGamma:
		return setGammaSize(value, output, conn)
	}
	// case SetterAuto
	if supportBacklight(output, conn) {
		return setBacklight(value, output, conn)
	}
	return setGammaSize(value, output, conn)
}

func Get(setter string, outputId uint32, conn *xgb.Conn) (float64, error) {
	output := randr.Output(outputId)
	switch setter {
	case SetterBacklight:
		return getBacklight(output, conn)
	case SetterGamma:
		return 1, nil
	}

	// case SetterAuto
	if supportBacklight(output, conn) {
		return getBacklight(output, conn)
	}
	return 1, nil
}

func getBacklightController(output randr.Output, conn *xgb.Conn) *displayBl.Controller {
	if !hasBacklightProp(output, conn) {
		return nil
	}
	c, _ := getDisplayBlController(output, conn)
	return c
}

func GetBacklightController(outputId uint32, conn *xgb.Conn) *displayBl.Controller {
	output := randr.Output(outputId)
	return getBacklightController(output, conn)
}

func supportBacklight(output randr.Output, conn *xgb.Conn) bool {
	if helper == nil {
		return false
	}
	c := getBacklightController(output, conn)
	return c != nil
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

var backlightAtom, edidAtom xproto.Atom

const (
	backlightAtomName = "Backlight"
	edidAtomName      = "EDID"
)

func getBacklightAtom(conn *xgb.Conn) xproto.Atom {
	if backlightAtom != 0 {
		return backlightAtom
	}

	atom, err := getAtom(conn, backlightAtomName)
	if err != nil {
		return xproto.AtomNone
	}
	backlightAtom = atom
	return backlightAtom
}

func getEDIDAtom(conn *xgb.Conn) xproto.Atom {
	if edidAtom != 0 {
		return edidAtom
	}

	atom, err := getAtom(conn, edidAtomName)
	if err != nil {
		return xproto.AtomNone
	}
	edidAtom = atom
	return edidAtom
}

func getAtom(conn *xgb.Conn, name string) (xproto.Atom, error) {
	reply, err := xproto.InternAtom(conn, false, uint16(len(name)), name).Reply()
	if err != nil {
		fmt.Println("get %q atom failed:", name, err)
		return 0, err
	}
	return reply.Atom, nil
}

func hasBacklightProp(output randr.Output, conn *xgb.Conn) bool {
	backlightProp, err := randr.QueryOutputProperty(conn, output, getBacklightAtom(conn)).Reply()
	if err != nil {
		return false
	}
	return backlightProp.Range && len(backlightProp.ValidValues) == 2
}

var errNotFoundBacklightController = errors.New("not found backlight controller")

func getDisplayBlController(output randr.Output, conn *xgb.Conn) (*displayBl.Controller, error) {
	// get output device edid
	edidAtom := getEDIDAtom(conn)
	edidProp, err := randr.GetOutputProperty(conn, output,
		edidAtom,           // Property
		xproto.AtomInteger, // Type
		0,                  // LongOffset
		128,                // LongLength
		false,              //Delete
		false,              //Pending
	).Reply()

	if err != nil {
		return nil, err
	}
	// get backlight controller
	controllers, err := displayBl.List()
	if err != nil {
		return nil, err
	}
	if c := controllers.GetByEDID(edidProp.Data); c != nil {
		return c, nil
	}
	return nil, errNotFoundBacklightController
}

func setBacklight(value float64, output randr.Output, conn *xgb.Conn) error {
	controller, err := getDisplayBlController(output, conn)
	if err != nil {
		return err
	}

	br := int32(float64(controller.MaxBrightness) * value)
	const displayBacklight = 1
	fmt.Printf("help set brightness %q max %v value %v br %v\n",
		controller.Name, controller.MaxBrightness, value, br)
	return helper.SetBrightness(displayBacklight, controller.Name, br)
}

func getBacklight(output randr.Output, conn *xgb.Conn) (float64, error) {
	controller, err := getDisplayBlController(output, conn)
	if err != nil {
		return 0.0, err
	}

	br, err := controller.GetBrightness()
	if err != nil {
		return 0.0, err
	}
	return float64(br) / float64(controller.MaxBrightness), err
}
