/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
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
