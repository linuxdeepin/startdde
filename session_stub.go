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
	"os/user"
	"pkg.deepin.io/lib/dbus"
)

const (
	START_DDE_DEST = "com.deepin.SessionManager"
	SHUTDOWN_PATH  = "/com/deepin/SessionManager"
	SHUTDOWN_IFC   = "com.deepin.SessionManager"
)

func (m *SessionManager) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		START_DDE_DEST,
		SHUTDOWN_PATH,
		SHUTDOWN_IFC,
	}
}

func (op *SessionManager) setPropName(name string) {
	switch name {
	case "CurrentUid":
		info, err := user.Current()
		if err != nil {
			logger.Infof("Get Current User Info Failed: %v", err)
			return
		}
		op.CurrentUid = info.Uid
	}
}

func (m *SessionManager) setPropStage(v int32) {
	if m.Stage != v {
		m.Stage = v
		dbus.NotifyChange(m, "Stage")
	}
}
