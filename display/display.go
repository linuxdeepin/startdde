// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display

import (
	"errors"
	"reflect"

	"github.com/godbus/dbus/v5"
	sysdisplay "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.display1"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/log"
)

var logger = log.NewLogger("daemon/display")

const (
	dbusServiceName = "org.deepin.dde.Display1"
	dbusInterface   = "org.deepin.dde.Display1"
	dbusPath        = "/org/deepin/dde/Display1"
)

var _dpy *Manager

var _greeterMode bool

func SetGreeterMode(val bool) {
	_greeterMode = val
}

type scaleFactorsHelper struct {
	changedCb func(factors map[string]float64) error
}

// ScaleFactorsHelper 全局的 scale factors 相关 helper，要传给 xsettings 模块。
var ScaleFactorsHelper scaleFactorsHelper

// 用于在 display.Start 还没被调用时，先由 xsettings.Start 调用了 ScaleFactorsHelper.SetScaleFactors, 缓存数据。
var _scaleFactors map[string]float64

func (h *scaleFactorsHelper) SetScaleFactors(factors map[string]float64) error {
	if _dpy == nil {
		_scaleFactors = factors
		return nil
	}
	return _dpy.setScaleFactors(factors)
}

func (h *scaleFactorsHelper) GetScaleFactors() (map[string]float64, error) {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	sysDisplay := sysdisplay.NewDisplay(sysBus)
	cfgJson, err := sysDisplay.GetConfig(0)
	if err != nil {
		return nil, err
	}
	var rootCfg struct {
		Config struct {
			ScaleFactors map[string]float64
		}
	}
	err = jsonUnmarshal(cfgJson, &rootCfg)
	if err != nil {
		return nil, err
	}
	return rootCfg.Config.ScaleFactors, nil
}

func (h *scaleFactorsHelper) SetChangedCb(fn func(factors map[string]float64) error) {
	h.changedCb = fn
}

func (m *Manager) setScaleFactors(factors map[string]float64) error {
	logger.Debug("setScaleFactors", factors)
	m.sysConfig.mu.Lock()
	defer m.sysConfig.mu.Unlock()

	if reflect.DeepEqual(m.sysConfig.Config.ScaleFactors, factors) {
		return nil
	}
	m.sysConfig.Config.ScaleFactors = factors
	err := m.saveSysConfigNoLock("scale factors changed")
	if err != nil {
		logger.Warning(err)
	}
	return err
}

func Start(service *dbusutil.Service) error {
	m := newManager(service)
	m.init()

	if !_greeterMode {
		// 正常 startdde
		err := service.Export(dbusPath, m)
		if err != nil {
			return err
		}
		so := service.GetServerObject(m)
		if so != nil {
			err = so.SetWriteCallback(m, "ColorTemperatureEnabled", func(write *dbusutil.PropertyWrite) *dbus.Error {
				value, ok := write.Value.(bool)
				if !ok {
					err = errors.New("Type is not bool")
					logger.Warning(err)
					return dbusutil.ToError(err)
				}
				ok = m.setPropColorTemperatureEnabled(value)
				if !ok {
					err = errors.New("Set ColorTemperatureEnabled failed!")
					logger.Warning(err)
					return dbusutil.ToError(err)
				}
				cfg := m.getSuitableUserMonitorModeConfig(m.DisplayMode)
				if cfg == nil {
					cfg = getDefaultUserMonitorModeConfig()
				}
				mode := ColorTemperatureModeNone
				if value {
					mode = cfg.ColorTemperatureModeOn
				}
				err = m.setColorTempMode(mode)
				return dbusutil.ToError(err)
			})
		}

		err = service.RequestName(dbusServiceName)
		if err != nil {
			return err
		}
	}
	_dpy = m
	return nil
}

func StartPart2() error {
	if _dpy == nil {
		return errors.New("_dpy is nil")
	}
	m := _dpy
	m.initSysDisplay()
	m.initTouchscreens()

	if !_greeterMode {
		controlRedshift("disable")
		m.applyColorTempConfig(m.DisplayMode)
	}

	return nil
}

func SetLogLevel(level log.Priority) {
	logger.SetLogLevel(level)
}
