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
	"pkg.deepin.io/lib/dbus"
)

var busObj *dbus.Object

func initDBusObject() error {
	bus, err := dbus.SessionBus()
	if err != nil {
		return err
	}
	busObj = bus.BusObject()
	return nil
}

const orgFreedesktopDBus = "org.freedesktop.DBus"

func isDBusServiceExist(name string) (bool, error) {
	var has bool
	err := busObj.Call(orgFreedesktopDBus+".NameHasOwner",
		0, name).Store(&has)
	return has, err
}

func startService(name string) error {
	var result uint32
	err := busObj.Call(orgFreedesktopDBus+".StartServiceByName", 0,
		name, uint32(0)).Store(&result)
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
