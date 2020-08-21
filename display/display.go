package display

import (
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

func Start(service *dbusutil.Service) {
	m := newManager(service)
	m.init()
	_dpy = m
}

func StartPart2(service *dbusutil.Service) error {
	m := _dpy
	m.initTouchscreens()
	m.initTouchMap()
	err := generateRedshiftConfFile()
	if err != nil {
		logger.Warning(err)
	}
	m.initColorTemperature()
	err = service.Export(dbusPath, m)
	if err != nil {
		return err
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		return err
	}

	for _, touch := range m.Touchscreens {
		if _, ok := m.TouchMap[touch.Serial]; !ok {
			err := m.associateTouch(m.Primary, touch.Serial)
			if err != nil {
				logger.Warningf("associate touch(%v, %v) failed: %v", m.Primary, touch.Serial, err)
			}
			err = m.showTouchscreenDialog(touch.Serial)
			if err != nil {
				logger.Warning("failed to show touchscreen dialog:", err)
			}
		}
	}

	return nil
}

func SetLogLevel(level log.Priority) {
	logger.SetLogLevel(level)
}
