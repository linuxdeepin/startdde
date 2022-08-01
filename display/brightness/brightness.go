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
	"fmt"
	"math"

	"github.com/godbus/dbus"
	backlight "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.helper.backlight"
	displayBl "github.com/linuxdeepin/go-lib/backlight/display"
	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/go-lib/multierr"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
)

var _useWayland bool

func SetUseWayland(value bool) {
	_useWayland = value
}

const (
	SetterAuto      = "auto"
	SetterGamma     = "gamma"
	SetterBacklight = "backlight"
	SetterDDCCI     = "ddcci"
	SetterDRM       = "drm"
)

var logger = log.NewLogger("daemon/display/brightness")

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
}

func Set(brightness float64, temperature int, setter string, isBuiltin bool, outputId uint32, conn *x.Conn, uuid string, edidBase64 string) error {
	if brightness < 0 {
		brightness = 0
	} else if brightness > 1 {
		brightness = 1
	}

	output := randr.Output(outputId)

	// 亮度和色温分开设置，亮度用背光，色温用 gamma
	setBlGamma := func() error {
		var errs error
		err := setBacklight(brightness, output, conn)
		if err != nil {
			errs = multierr.Append(errs, err)
		}

		err = setOutputCrtcGamma(gammaSetting{
			brightness:  1,
			temperature: temperature,
		}, output, conn)
		if err != nil {
			errs = multierr.Append(errs, err)
		}
		return errs
	}

	// 亮度和色温都用 gamma 值设置
	setGamma := func() error {
		return setOutputCrtcGamma(gammaSetting{
			brightness:  brightness,
			temperature: temperature,
		}, output, conn)
	}

	setFn := setGamma
	switch setter {
	case SetterBacklight:
		setFn = setBlGamma
	case SetterAuto:
		// 自动检测仅自适应backlight和ddcci亮度调节
		// 若两种都不支持，使用gamma调节
		if isBuiltin && supportBacklight() {
			setFn = setBlGamma
		} else if supportDDCCIBrightness(edidBase64) {
			return setDDCCIBrightness(brightness, edidBase64)
		}
	case SetterDDCCI:
		return setDDCCIBrightness(brightness, edidBase64)
	/* DRM 目前暂无检查是否支持接口，根据硬件驱动进行gsetting配置调节 */
	case SetterDRM:
		err := setBrigntnessByKwin(uuid, brightness)
		return err
		//case SetterGamma
	}
	return setFn()
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

func GetMaxBacklightBrightness() int {
	if len(controllers) == 0 {
		return 0
	}
	maxBrightness := controllers[0].MaxBrightness
	for _, controller := range controllers {
		if maxBrightness > controller.MaxBrightness {
			maxBrightness = controller.MaxBrightness
		}
	}
	return maxBrightness
}

func GetBacklightController(outputId uint32, conn *x.Conn) (*displayBl.Controller, error) {
	// TODO
	//output := randr.Output(outputId)
	//return getBacklightController(output, conn)
	return nil, nil
}

func supportBacklight() bool {
	if helper == nil {
		return false
	}
	return len(controllers) > 0
}

func setOutputCrtcGamma(setting gammaSetting, output randr.Output, conn *x.Conn) error {
	if _useWayland {
		return nil
	}

	outputInfo, err := randr.GetOutputInfo(conn, output, x.CurrentTime).Reply(conn)
	if err != nil {
		fmt.Printf("Get output(%v) failed: %v\n", output, err)
		return err
	}

	if outputInfo.Crtc == 0 || outputInfo.Connection != randr.ConnectionConnected {
		fmt.Printf("output(%s) no crtc or disconnected\n", outputInfo.Name)
		return fmt.Errorf("output(%v) unready", output)
	}

	gamma, err := randr.GetCrtcGammaSize(conn, outputInfo.Crtc).Reply(conn)
	if err != nil {
		fmt.Printf("Failed to get gamma size: %v\n", err)
		return err
	}

	if gamma.Size == 0 {
		return fmt.Errorf("output(%v) has invalid gamma size", output)
	}

	red, green, blue := initGammaRamp(int(gamma.Size))
	fillColorRamp(red, green, blue, setting)
	return randr.SetCrtcGammaChecked(conn, outputInfo.Crtc,
		red, green, blue).Check(conn)
}

func initGammaRamp(size int) (red, green, blue []uint16) {
	red = make([]uint16, size)
	green = make([]uint16, size)
	blue = make([]uint16, size)

	for i := 0; i < size; i++ {
		value := uint16(float64(i) / float64(size) * (math.MaxUint16 + 1))
		red[i] = value
		green[i] = value
		blue[i] = value
	}
	return
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
