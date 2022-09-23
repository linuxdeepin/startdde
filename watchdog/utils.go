// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package watchdog

import (
	dbus "github.com/godbus/dbus"
)

var busObj dbus.BusObject

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
