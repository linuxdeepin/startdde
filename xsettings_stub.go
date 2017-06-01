/**
 * Copyright (C) 2014 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package main

import (
	"pkg.deepin.io/lib/dbus"
)

const (
	xsDBusSender = "com.deepin.SessionManager"
	xsDBusPath   = "/com/deepin/XSettings"
	xsDBusIFC    = "com.deepin.XSettings"
)

func (m *XSManager) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		Dest:       xsDBusSender,
		ObjectPath: xsDBusPath,
		Interface:  xsDBusIFC,
	}
}
