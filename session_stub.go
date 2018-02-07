/*
 * Copyright (C) 2014 ~ 2018 Deepin Technology Co., Ltd.
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
