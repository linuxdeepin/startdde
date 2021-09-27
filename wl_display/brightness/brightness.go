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

package brightness

import (
	"errors"
	"fmt"

	"github.com/godbus/dbus"
	backlight "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.helper.backlight"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
	displayBl "pkg.deepin.io/lib/backlight/display"
	"pkg.deepin.io/lib/log"
)

const (
	SetterAuto      = "auto"
	SetterGamma     = "gamma"
	SetterBacklight = "backlight"
	SetterDDCCI     = "ddcci"
	SetterDRM       = "drm"
)

var logger = log.NewLogger("daemon/wl_display/brightness")
var helper backlight.Backlight
var ddcciHelper backlight.DDCCI

func InitBacklightHelper() {
	var err error
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return
	}
	helper = backlight.NewBacklight(sysBus)
	ddcciHelper = backlight.NewDDCCI(sysBus)
	RefreshDDCCI()
}

func RefreshDDCCI() {
	if ddcciHelper != nil {
		logger.Infof("brightness: call RefreshDisplays")
		ddcciHelper.RefreshDisplays(0)
	} else {
		logger.Warningf("brightness: failed call RefreshDisplays, helper is null")
	}
}

func getHelper() backlight.Backlight {
	if helper == nil {
		InitBacklightHelper()
	}

	return helper
}

func Set(uuid string, value float64, setter string, isBuiltin bool, edidBase64 string) error {
	if value < 0 {
		value = 0
	} else if value > 1 {
		value = 1
	}
	switch setter {
	case SetterBacklight:
		//avoid to set builtin display twice to causing brightness abnormal when press F1 set brughtness
		if isBuiltin {
			return setBacklightOnlyOne(value)
		}
		return errors.New("brightness: only buildin display support BacklightSetter")
	case SetterDDCCI:
		err := setDDCCIBrightness(value, edidBase64)
		return err
	case SetterDRM:
		err := setBrigntnessByKwin(uuid, value)
		return err
	}
	// case SetterAuto
	if isBuiltin {
		return setBacklightOnlyOne(value)
	}

	if supportDDCCIBrightness(edidBase64) {
		err := setDDCCIBrightness(value, edidBase64)
		if err == nil {
			return nil
		}
	}

	err := setBrigntnessByKwin(uuid, value)
	if err == nil {
		logger.Debug("brightness: setBrigntnessByKwin ok")
		return nil
	}
	return errors.New("brightness: AutoSetter falied")
}

//String outputs, const int brightness
func setBrigntnessByKwin(output string, value float64) error {
	logger.Info("setBrigntnessByKwin")
	sessionBus, err := dbus.SessionBus()
	if err != nil {
		return err
	}
	sessionObj := sessionBus.Object("com.deepin.daemon.KWayland", "/com/deepin/daemon/KWayland/Output")
	err = sessionObj.Call("com.deepin.daemon.KWayland.Output.setBrightness", 0, output, int32(value*100)).Store()
	if err != nil {
		logger.Warning(err)
		return err
	}
	return nil
}

func supportDDCCIBrightness(edidBase64 string) bool {
	res, err := ddcciHelper.CheckSupport(0, edidBase64)
	logger.Info("supportDDCCIBrightness", res, err)
	if err != nil {
		logger.Warningf("brightness: failed to check ddc/ci support: %v", err)
		return false
	}

	return res
}

func setDDCCIBrightness(value float64, edidBase64 string) error {
	percent := int32(value * 100)
	logger.Debugf("brightness: ddcci set brightness %d", percent)
	return ddcciHelper.SetBrightness(0, edidBase64, percent)
}

func getDDCCIBrightness(edidBase64 string) (float64, error) {
	br, err := ddcciHelper.GetBrightness(0, edidBase64)
	if err != nil {
		return 1, err
	} else {
		return (float64(br) / 100.0), err
	}
}

// unused function
func Get(setter string, isButiltin bool, outputId uint32, conn *x.Conn) (float64, error) {
	output := randr.Output(outputId)
	switch setter {
	case SetterBacklight:
		return getBacklightOnlyOne()
	case SetterGamma:
		return 1, nil
	}

	// case SetterAuto
	if isButiltin {
		if supportBacklight(output, conn) {
			return getBacklight(output, conn)
		}
	}
	return 1, nil
}

func GetBacklightController(outputId uint32, conn *x.Conn) (*displayBl.Controller, error) {
	output := randr.Output(outputId)
	return getBacklightController(output, conn)
}

func supportBacklight(output randr.Output, conn *x.Conn) bool {
	if helper == nil {
		return false
	}
	c, _ := getBacklightController(output, conn)
	return c != nil
}

func setOutputCrtcGamma(value float64, output randr.Output, conn *x.Conn) error {
	oinfo, err := randr.GetOutputInfo(conn, output, x.CurrentTime).Reply(conn)
	if err != nil {
		fmt.Printf("Get output(%v) failed: %v\n", output, err)
		return err
	}

	if oinfo.Crtc == 0 || oinfo.Connection != randr.ConnectionConnected {
		fmt.Printf("Output(%s) no crtc or disconnected\n", string(oinfo.Name))
		return fmt.Errorf("Output(%v) unready", output)
	}

	gamma, err := randr.GetCrtcGammaSize(conn, oinfo.Crtc).Reply(conn)
	if err != nil {
		fmt.Printf("Failed to get gamma size: %v\n", err)
		return err
	}

	if gamma.Size == 0 {
		return fmt.Errorf("The output(%v) has invalid gamma size", output)
	}

	red, green, blue := genGammaRamp(gamma.Size, value)
	return randr.SetCrtcGammaChecked(conn, oinfo.Crtc,
		red, green, blue).Check(conn)
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

var errNotFoundBacklightController = errors.New("not found backlight controller")
var controllers displayBl.Controllers

func init() {
	var err error
	controllers, err = displayBl.List()
	if err != nil {
		fmt.Println("failed to list backlight controller:", err)
	}
}

func getBacklightController(output randr.Output, conn *x.Conn) (*displayBl.Controller, error) {
	// get output device EDID
	atomEDID, err := conn.GetAtom("EDID")
	if err != nil {
		return nil, err
	}

	edidProp, err := randr.GetOutputProperty(conn, output,
		atomEDID,      // Property
		x.AtomInteger, // Type
		0,             // LongOffset
		32,            // LongLength
		false,         //Delete
		false,         //Pending
	).Reply(conn)

	if err != nil {
		return nil, err
	}

	// find backlight controller
	if c := controllers.GetByEDID(edidProp.Value); c != nil {
		return c, nil
	}

	return nil, errNotFoundBacklightController
}

func setBacklight(value float64, output randr.Output, conn *x.Conn) error {
	controller, err := getBacklightController(output, conn)
	if err != nil {
		return err
	}
	return _setBacklight(value, controller)
}

func getBacklight(output randr.Output, conn *x.Conn) (float64, error) {
	controller, err := getBacklightController(output, conn)
	if err != nil {
		return 0.0, err
	}
	return _getBacklight(controller)
}

func _setBacklight(value float64, controller *displayBl.Controller) error {
	br := int32(float64(controller.MaxBrightness) * value)
	const backlightTypeDisplay = 1
	fmt.Printf("help set brightness %q max %v value %v br %v\n",
		controller.Name, controller.MaxBrightness, value, br)
	return getHelper().SetBrightness(0, backlightTypeDisplay, controller.Name, br)
}

func _getBacklight(controller *displayBl.Controller) (float64, error) {
	br, err := controller.GetBrightness()
	if err != nil {
		return 0.0, err
	}
	return float64(br) / float64(controller.MaxBrightness), nil
}

// there is only one backlight controller
func getBacklightControllerOnlyOne() (*displayBl.Controller, error) {
	if len(controllers) > 1 {
		return nil, errors.New("found more than one backlight controller")
	}
	if len(controllers) == 0 {
		return nil, errNotFoundBacklightController
	}
	// len(controllers) is 1
	return controllers[0], nil
}

func getBacklightOnlyOne() (float64, error) {
	controller, err := getBacklightControllerOnlyOne()
	if err != nil {
		return 0.0, err
	}
	return _getBacklight(controller)
}

func setBacklightOnlyOne(value float64) error {
	controller, err := getBacklightControllerOnlyOne()
	if err != nil {
		return err
	}
	return _setBacklight(value, controller)
}
