package display

import (
	"pkg.deepin.io/lib/dbus"
)

func Start() error {
	manager, err := newManager()
	if err != nil {
		logger.Error("New display manager failed:", err)
		return err
	}
	err = dbus.InstallOnSession(manager)
	if err != nil {
		logger.Error("Install session bus failed:", err)
		return err
	}
	_dpy = manager
	manager.init()
	manager.listenEvent()
	return nil
}
