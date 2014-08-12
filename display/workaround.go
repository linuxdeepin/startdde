package display

import "dbus/com/deepin/daemon/keybinding"

var __keepMediakeyManagerAlive interface{}

func (dpy *Display) workaroundBacklight() {
	mediaKeyManager, err := keybinding.NewMediaKey("com.deepin.daemon.KeyBinding", "/com/deepin/daemon/MediaKey")
	if err != nil {
		logger.Error("Can't connect to /com/deepin/daemon/MediaKey", err)
		return
	}
	__keepMediakeyManagerAlive = mediaKeyManager

	workaround := func(m *Monitor) {
		dpyinfo := GetDisplayInfo()
		for _, name := range dpyinfo.ListNames() {
			op := dpyinfo.QueryOutputs(name)
			if support := supportedBacklight(xcon, op); support {
				v := getBacklight()
				dpy.setPropBrightness(name, v)
				dpy.saveBrightness(name, v)
			}
		}
	}

	mediaKeyManager.ConnectBrightnessUp(func(onPress bool) {
		if !onPress {
			for _, m := range dpy.Monitors {
				workaround(m)
			}
		}
	})
	mediaKeyManager.ConnectBrightnessDown(func(onPress bool) {
		if !onPress {
			for _, m := range dpy.Monitors {
				workaround(m)
			}
		}
	})
}
