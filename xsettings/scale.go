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

package xsettings

import (
	"math"
	"os"
	"strconv"
	"sync"

	ddaemon "dbus/com/deepin/daemon/daemon/system"
	"dbus/com/deepin/daemon/greeter"

	"pkg.deepin.io/gir/gio-2.0"
	"pkg.deepin.io/dde/api/userenv"
	"pkg.deepin.io/lib/dbus"
)

var pamEnvFile = os.Getenv("HOME") + "/.pam_environment"

func (m *XSManager) getScaleFactor() float64 {
	scale := m.gs.GetDouble(gsKeyScaleFactor)
	return scale
}

const (
	gsKeyScaleFactor = "scale-factor"
	EnvQtScaleFactor = "QT_SCALE_FACTOR"
	EnvJavaOptions   = "_JAVA_OPTIONS"

	baseCursorSize = 24
)

func (m *XSManager) setScaleFactor(scale float64, emitDone bool) {
	logger.Debug("setScaleFactor", scale)
	setScaleStatus(true)
	m.gs.SetDouble(gsKeyScaleFactor, scale)
	// for qt
	scaleStr := strconv.FormatFloat(scale, 'f', 2, 64)
	err := userenv.Set(EnvQtScaleFactor, scaleStr)
	if err != nil {
		logger.Warning("Failed to set qt scale factor:", err)
	}
	err = os.Setenv(EnvQtScaleFactor, scaleStr)
	if err != nil {
		logger.Warning(err)
	}

	// if 1.7 < scale < 2, window scale = 2
	windowScale := int32(math.Trunc((scale+0.3)*10) / 10)
	if windowScale < 1 {
		windowScale = 1
	}
	oldWindowScale := m.gs.GetInt("window-scale")
	if oldWindowScale != windowScale {
		m.gs.SetInt("window-scale", windowScale)
	}

	cursorSize := int32(baseCursorSize * scale)
	m.gs.SetInt("gtk-cursor-theme-size", cursorSize)
	// set cursor size for deepin-metacity
	gsWrapGDI := gio.NewSettings("com.deepin.wrap.gnome.desktop.interface")
	gsWrapGDI.SetInt("cursor-size", cursorSize)
	gsWrapGDI.Unref()

	go func() {
		doScaleGreeter(scale)
		doScalePlymouth(uint32(windowScale))
		setScaleStatus(false)
		if emitDone {
			dbus.Emit(m, "SetScaleFactorDone")
		}
	}()
}

func doScaleGreeter(scale float64) {
	setter, err := greeter.NewGreeter("com.deepin.daemon.Greeter",
		"/com/deepin/daemon/Greeter")
	if err != nil {
		logger.Warning(err)
		return
	}

	err = setter.SetScaleFactor(scale)
	if err != nil {
		logger.Warning("Failed to set greeter scale:", err)
	}
}

func doScalePlymouth(scale uint32) {
	setter, err := ddaemon.NewDaemon("com.deepin.daemon.Daemon",
		"/com/deepin/daemon/Daemon")
	if err != nil {
		logger.Warning(err)
		return
	}

	err = setter.ScalePlymouth(scale)
	if err != nil {
		logger.Warning("Failed to scale plymouth:", err)
	}
}

var (
	_isScaling   = false
	_scaleLocker sync.Mutex
)

func setScaleStatus(status bool) {
	_scaleLocker.Lock()
	_isScaling = status
	_scaleLocker.Unlock()
}

func getScaleStatus() bool {
	_scaleLocker.Lock()
	defer _scaleLocker.Unlock()
	return _isScaling
}
