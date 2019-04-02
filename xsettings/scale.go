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
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
	"pkg.deepin.io/dde/api/userenv"
	"pkg.deepin.io/gir/gio-2.0"
	"pkg.deepin.io/lib/dbus"
	dbus1 "pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/keyfile"
	"pkg.deepin.io/lib/xdg/basedir"
)

func (m *XSManager) getScaleFactor() float64 {
	scale := m.gs.GetDouble(gsKeyScaleFactor)
	return scale
}

const (
	gsKeyScaleFactor       = "scale-factor"
	gsKeyIndividualScaling = "individual-scaling"
	baseCursorSize         = 24
)

func (m *XSManager) setScaleFactor(scale float64) {
	logger.Debug("setScaleFactor", scale)
	m.gs.SetDouble(gsKeyScaleFactor, scale)

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

	m.setScaleFactorForPlymouth(int(windowScale))
}

func parseScreenFactors(str string) map[string]float64 {
	pairs := strings.Split(str, ";")
	result := make(map[string]float64)
	for _, value := range pairs {
		kv := strings.SplitN(value, "=", 2)
		if len(kv) != 2 {
			continue
		}

		value, err := strconv.ParseFloat(kv[1], 64)
		if err != nil {
			logger.Warning(err)
			continue
		}

		result[kv[0]] = value
	}

	return result
}

func joinScreenScaleFactors(v map[string]float64) string {
	pairs := make([]string, len(v))
	idx := 0
	for key, value := range v {
		pairs[idx] = fmt.Sprintf("%s=%.2f", key, value)
		idx++
	}
	return strings.Join(pairs, ";")
}

func getQtThemeFile() string {
	return filepath.Join(basedir.GetUserConfigDir(), "deepin/qt-theme.ini")
}

func cleanUpDdeEnv() error {
	ue, err := userenv.Load()
	if err != nil {
		return err
	}

	needSave := false
	for _, key := range []string{
		"QT_SCALE_FACTOR",
		"QT_SCREEN_SCALE_FACTORS",
		"QT_AUTO_SCREEN_SCALE_FACTOR",
		"QT_FONT_DPI",
	} {
		delete(ue, key)
		needSave = true
	}

	if needSave {
		err = userenv.Save(ue)
	}
	return err
}

func (m *XSManager) setScreenScaleFactorsForQt(factors map[string]float64) error {
	filename := getQtThemeFile()
	kf := keyfile.NewKeyFile()
	err := kf.LoadFromFile(filename)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	value := joinScreenScaleFactors(factors)
	value = strconv.Quote(value)
	kf.SetValue("Theme", "ScreenScaleFactors", value)
	kf.DeleteKey("Theme", "ScaleFactor")
	err = kf.SaveToFile(filename)
	return err
}

func getPrimaryScreenName(xConn *x.Conn) (string, error) {
	rootWin := xConn.GetDefaultScreen().Root
	getPrimaryReply, err := randr.GetOutputPrimary(xConn, rootWin).Reply(xConn)
	if err != nil {
		return "", err
	}
	outputInfo, err := randr.GetOutputInfo(xConn, getPrimaryReply.Output,
		x.CurrentTime).Reply(xConn)
	if err != nil {
		return "", err
	}
	return outputInfo.Name, nil
}

func (m *XSManager) setScreenScaleFactors(factors map[string]float64) error {
	logger.Debug("setScreenScaleFactors", factors)
	for _, f := range factors {
		if f <= 0 {
			return errors.New("invalid value")
		}
	}

	primary, err := getPrimaryScreenName(m.conn)
	if err != nil {
		return err
	}
	primaryFactor := 1.0
	if v, ok := factors[primary]; ok {
		primaryFactor = v
	} else {
		logger.Warning("not found value for primary", primary)
	}
	m.setScaleFactor(primaryFactor)

	factorsJoined := joinScreenScaleFactors(factors)
	m.gs.SetString(gsKeyIndividualScaling, factorsJoined)

	err = m.setScreenScaleFactorsForQt(factors)
	if err != nil {
		return err
	}

	err = cleanUpDdeEnv()
	if err != nil {
		logger.Warning(err)
	}

	err = m.updateGreeterQtTheme()
	return err
}

func (m *XSManager) getScreenScaleFactors() map[string]float64 {
	factorsJoined := m.gs.GetString(gsKeyIndividualScaling)
	return parseScreenFactors(factorsJoined)
}

const plymouthConfigFile = "/etc/plymouth/plymouthd.conf"

func (m *XSManager) setScaleFactorForPlymouthReal(factor int) {
	logger.Debug("scalePlymouth", factor)
	currentFactor := 0
	theme, err := getPlymouthTheme(plymouthConfigFile)
	if err == nil {
		currentFactor = getPlymouthThemeScaleFactor(theme)
	} else {
		logger.Warning(err)
	}

	if currentFactor == factor {
		logger.Debug("quick end scalePlymouth", factor)
		return
	}

	emitSignal := m.emitSignal

	if emitSignal {
		err := dbus.Emit(m, "SetScaleFactorStarted")
		if err != nil {
			logger.Warning(err)
		}
	}
	err = m.sysDaemon.ScalePlymouth(0, uint32(factor))
	if emitSignal {
		err := dbus.Emit(m, "SetScaleFactorDone")
		if err != nil {
			logger.Warning(err)
		}
	}

	logger.Debug("end scalePlymouth", factor)
	if err != nil {
		logger.Warning(err)
	}
}

func (m *XSManager) startScaleFactorForPlymouth(factor int) {
	logger.Debug("startScaleFactorForPlymouth", factor)
	go func() {
		m.setScaleFactorForPlymouthReal(factor)
		m.endScaleFactorForPlymouth()
	}()
}

func (m *XSManager) endScaleFactorForPlymouth() {
	m.plymouthScalingMu.Lock()
	defer m.plymouthScalingMu.Unlock()

	if len(m.plymouthScalingTasks) == 0 {
		// stop
		m.plymouthScaling = false
	} else {
		factor := m.plymouthScalingTasks[len(m.plymouthScalingTasks)-1]
		logger.Debug("use last in tasks:", factor, m.plymouthScalingTasks)
		m.plymouthScalingTasks = nil
		m.startScaleFactorForPlymouth(factor)
	}
}

func (m *XSManager) setScaleFactorForPlymouth(factor int) {
	if factor > 2 {
		factor = 2
	}
	m.plymouthScalingMu.Lock()

	if m.plymouthScaling {
		m.plymouthScalingTasks = append(m.plymouthScalingTasks, factor)
		logger.Debug("add to tasks", factor)
	} else {
		m.plymouthScaling = true
		m.startScaleFactorForPlymouth(factor)
	}

	m.plymouthScalingMu.Unlock()
}

func getPlymouthTheme(file string) (string, error) {
	var kf = keyfile.NewKeyFile()
	err := kf.LoadFromFile(file)
	if err != nil {
		return "", err
	}

	return kf.GetString("Daemon", "Theme")
}

func getPlymouthThemeScaleFactor(theme string) int {
	switch theme {
	case "deepin-logo", "deepin-ssd-logo":
		return 1
	case "deepin-hidpi-logo", "deepin-hidpi-ssd-logo":
		return 2
	default:
		return 0
	}
}

func (m *XSManager) updateGreeterQtTheme() error {
	filename := getQtThemeFile()
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			logger.Warning(err)
		}
	}()
	err = m.greeter.UpdateGreeterQtTheme(0, dbus1.UnixFD(f.Fd()))
	return err
}
