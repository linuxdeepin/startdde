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

	backlight "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.helper.backlight"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
	"pkg.deepin.io/dde/startdde/display/utils"
	displayBl "pkg.deepin.io/lib/backlight/display"
	dbus "pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/log"
)

const (
	SetterAuto      = "auto"
	SetterGamma     = "gamma"
	SetterBacklight = "backlight"
	SetterDDCCI     = "ddcci"
)

var logger = log.NewLogger("daemon/display")

var helper *backlight.Backlight

func InitBacklightHelper() {
	var err error
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return
	}
	helper = backlight.NewBacklight(sysBus)
}

func Set(value float64, setter string, isBuiltin bool, outputId uint32, conn *x.Conn) error {
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
		return setOutputCrtcGamma(value, output, conn)
	case SetterDDCCI:
		return setDDCCIBacklight(value, output, conn)
	}
	// case SetterAuto
	if isBuiltin {
		if supportBacklight(output, conn) {
			return setBacklight(value, output, conn)
		}
	}
	if supportDDCCIBacklight(output, conn) {
		return setDDCCIBacklight(value, output, conn)
	}

	return setOutputCrtcGamma(value, output, conn)
}

// unused function
//func Get(setter string, isButiltin bool, outputId uint32, conn *x.Conn) (float64, error) {
//	output := randr.Output(outputId)
//	switch setter {
//	case SetterBacklight:
//		return getBacklightOnlyOne()
//	case SetterGamma:
//		return 1, nil
//	}
//
//	// case SetterAuto
//	if isButiltin {
//		if supportBacklight(output, conn) {
//			return getBacklight(output, conn)
//		}
//	}
//	return 1, nil
//}

func GetBacklightController(outputId uint32, conn *x.Conn) (*displayBl.Controller, error) {
	// TODO
	//output := randr.Output(outputId)
	//return getBacklightController(output, conn)
	return nil, nil
}

func supportBacklight(output randr.Output, conn *x.Conn) bool {
	if helper == nil {
		return false
	}
	return len(controllers) > 0
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

func setBacklight(value float64, output randr.Output, conn *x.Conn) error {
	for _, controller := range controllers {
		err := _setBacklight(value, controller)
		if err != nil {
			fmt.Printf("WARN: failed to set backlight %s: %v", controller.Name, err)
		}
	}
	return nil
}

func _setBacklight(value float64, controller *displayBl.Controller) error {
	br := int32(float64(controller.MaxBrightness) * value)
	const backlightTypeDisplay = 1
	fmt.Printf("help set brightness %q max %v value %v br %v\n",
		controller.Name, controller.MaxBrightness, value, br)
	return helper.SetBrightness(0, backlightTypeDisplay, controller.Name, br)
}

func supportDDCCIBacklight(output randr.Output, conn *x.Conn) bool {
	edid, err := utils.GetOutputEDID(conn, output)
	if err != nil {
		return false
	}

	edidChecksum := utils.GetEDIDChecksum(edid)
	return DDCBrightness.SupportBrightness(edidChecksum)
}

func setDDCCIBacklight(value float64, output randr.Output, conn *x.Conn) error {
	edid, err := utils.GetOutputEDID(conn, output)
	if err != nil {
		return err
	}

	edidChecksum := utils.GetEDIDChecksum(edid)

	percent := int(value * 100)
	logger.Debugf("output %d, ddcci set brightness %d", output, percent)
	return DDCBrightness.SetBrightness(edidChecksum, percent)
}
