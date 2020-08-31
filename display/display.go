package display

import (
	dbus "github.com/godbus/dbus"
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

func Start() error {
	sessionBus, err := dbus.SessionBus()
	if err != nil {
		return err
	}
	service := dbusutil.NewService(sessionBus)
	m := newManager(service)
	_dpy = m
	m.init()
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
			err := m.showTouchscreenDialog(touch.Serial)
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

func GetRecommendedScaleFactor() float64 {
	if _dpy == nil {
		return 1.0
	}
	return _dpy.recommendScaleFactor
}
