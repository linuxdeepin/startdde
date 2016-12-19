/**
 * Copyright (C) 2016 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package watchdog

import (
	"dbus/org/freedesktop/dbus"
)

var dbusDaemon *dbus.DBusDaemon

func initDBusDaemon() error {
	if dbusDaemon != nil {
		return nil
	}

	var err error
	dbusDaemon, err = dbus.NewDBusDaemon("org.freedesktop.DBus", "/")
	if err != nil {
		dbusDaemon = nil
		return err
	}
	return nil
}

func destroyDBusDaemon() {
	if dbusDaemon == nil {
		return
	}
	dbus.DestroyDBusDaemon(dbusDaemon)
}

func isDBusDestExist(dest string) bool {
	if err := initDBusDaemon(); err != nil {
		return false
	}

	names, err := dbusDaemon.ListNames()
	if err != nil {
		return false
	}
	return isItemInList(dest, names)
}

func startService(dest string) error {
	if err := initDBusDaemon(); err != nil {
		return err
	}

	// flag unused
	_, err := dbusDaemon.StartServiceByName(dest, 0)
	return err
}

func isItemInList(item string, list []string) bool {
	for _, v := range list {
		if item == v {
			return true
		}
	}
	return false
}
