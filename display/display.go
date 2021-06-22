package display

import (
	"errors"
	"os"

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

func Start(service *dbusutil.Service) error {
	m := newManager(service)
	m.init()
	err := service.Export(dbusPath, m)
	if err != nil {
		return err
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		return err
	}
	_dpy = m
	return nil
}

func StartPart2() error {
	if _dpy == nil {
		return errors.New("_dpy is nil")
	}
	m := _dpy
	m.initTouchscreens()
	m.initTouchMap()
	if os.Getenv("XDG_SESSION_DESKTOP") != padEnv {
		err := generateRedshiftConfFile()
		if err != nil {
			logger.Warning(err)
		}

		m.initColorTemperature()
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
