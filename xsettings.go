/**
 * Copyright (c) 2011 ~ 2013 Deepin, Inc.
 *               2011 ~ 2013 jouyouyun
 *
 * Author:      jouyouyun <jouyouwen717@gmail.com>
 * Maintainer:  jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program; if not, see <http://www.gnu.org/licenses/>.
 **/

package main

import (
        "dlib/dbus"
        "dlib/logger"
        "github.com/BurntSushi/xgb"
)

type XSettingsManager struct{}

const (
        XSETTINGS_DEST = "com.deepin.SessionManager"
        XSETTINGS_PATH = "/com/deepin/XSettings"
        XSETTINGS_IFC  = "com.deepin.XSettings"
)

var (
        X *xgb.Conn
)

/*
 * vType value : 0, 1, 2
 * vType = 0, int, value = "123"
 * vType = 1, string, value = ""
 * vType = 2, color, value = "1,2,3,4"
 */
func (op *XSettingsManager) SetXSettingsKey(key, value string, vType int32) {
        setXSettingsKey(key, value, vType)
        setGSettingsKey(key, value, vType)
}

func (op *XSettingsManager) GetDBusInfo() dbus.DBusInfo {
        return dbus.DBusInfo{
                XSETTINGS_DEST,
                XSETTINGS_PATH,
                XSETTINGS_IFC,
        }
}

func startXSettings() {
        var err error
        X, err = xgb.NewConn()
        if err != nil {
                logger.Println("Unable to connect X server:", err)
                panic(err)
        }

        newXWindow()
        initSelection()

        m := &XSettingsManager{}
        dbus.InstallOnSession(m)
        //dbus.DealWithUnhandledMessage()

        //select {}
}
