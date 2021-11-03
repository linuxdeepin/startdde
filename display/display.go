package display

import (
	"errors"

	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/log"
)

var logger = log.NewLogger("daemon/display")

const (
	dbusServiceName = "com.deepin.daemon.Display"
	dbusInterface   = "com.deepin.daemon.Display"
	dbusPath        = "/com/deepin/daemon/Display"
)

var _dpy *Manager

var _greeterMode bool

func SetGreeterMode(val bool) {
	_greeterMode = val
}

// 用于在 display.Start 还没被调用时，先调用了 SetScaleFactors, 缓存数据。
var _factors map[string]float64

func SetScaleFactors(factors map[string]float64) error {
	if _dpy == nil {
		_factors = factors
		return nil
	}
	return _dpy.setScaleFactors(factors)
}

func (m *Manager) setScaleFactors(factors map[string]float64) error {
	m.sysConfig.mu.Lock()
	defer m.sysConfig.mu.Unlock()

	m.sysConfig.Config.ScaleFactors = factors
	err := m.saveSysConfigNoLock()
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
	m.initDisplayCfg()
	m.initTouchscreens()

	if !_greeterMode {
		err := generateRedshiftConfFile()
		if err != nil {
			logger.Warning(err)
		}
		m.applyColorTempConfig(m.DisplayMode)
	}

	return nil
}

func SetLogLevel(level log.Priority) {
	logger.SetLogLevel(level)
}
