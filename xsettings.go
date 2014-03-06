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
	"dlib/gio-2.0"
	"github.com/BurntSushi/xgb"
	"strconv"
)

type XSettingsManager struct {
	PropList    []string
	PropChanged func(string, string)
}

const (
	XSETTINGS_DEST = "com.deepin.SessionManager"
	XSETTINGS_PATH = "/com/deepin/XSettings"
	XSETTINGS_IFC  = "com.deepin.XSettings"
)

var (
	X          *xgb.Conn
	xsIntMap   map[string]uint32
	xsStrMap   map[string]string
	xsColorMap map[string][]string
)

func (op *XSettingsManager) SetInterger(key string, value uint32) {
	k := getXSettingsKey(key)
	if len(k) <= 0 {
		return
	}
	setXSettingsName(key, value)
	xIntSettings.SetUint(k, int(value))
}

func (op *XSettingsManager) GetInterger(key string) (uint32, bool) {
	ret, ok := xsIntMap[getXSettingsKey(key)]
	if !ok {
		return 0, false
	}

	return ret, true
}

func (op *XSettingsManager) SetString(key, value string) {
	k := getXSettingsKey(key)
	if len(k) <= 0 {
		return
	}
	setXSettingsName(key, value)
	xStrSettings.SetString(k, value)
}

func (op *XSettingsManager) GetString(key string) (string, bool) {
	ret, ok := xsStrMap[getXSettingsKey(key)]
	if !ok {
		return "", false
	}

	return ret, true
}

/*
  Color: [4]string
*/
func (op *XSettingsManager) SetColor(key string, value []string) {
	k := getXSettingsKey(key)
	if len(k) <= 0 {
		return
	}
	tmp := []byte{}

	for _, v := range value {
		n, _ := strconv.ParseUint(v, 10, 16)
		tmp = append(tmp, byte(n))
	}
	setXSettingsName(key, tmp)
	xColorSettings.SetStrv(k, value)
}

func (op *XSettingsManager) GetColor(key string) ([]string, bool) {
	ret, ok := xsColorMap[getXSettingsKey(key)]
	if !ok {
		return []string{}, false
	}

	return ret, true
}

func (op *XSettingsManager) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		XSETTINGS_DEST,
		XSETTINGS_PATH,
		XSETTINGS_IFC,
	}
}

func (op *XSettingsManager) listenGSettings() {
	xStrSettings.Connect("changed", func(s *gio.Settings, key string) {
		value := xStrSettings.GetString(key)
		v := xsStrMap[key]
		if v != value {
			k := xsKeyMap[key]
			setXSettingsName(k, value)
			xsStrMap[key] = value
			op.PropChanged(key, "string")
		}
	})

	xIntSettings.Connect("changed", func(s *gio.Settings, key string) {
		value := xIntSettings.GetUint(key)
		v := xsIntMap[key]
		if int(v) != value {
			k := xsKeyMap[key]
			setXSettingsName(k, uint32(value))
			xsIntMap[key] = uint32(value)
			op.PropChanged(key, "int")
		}
	})

	xColorSettings.Connect("changed", func(s *gio.Settings, key string) {
		value := xColorSettings.GetStrv(key)
		v := xsColorMap[key]
		if !isStrArrayEqual(value, v) {
			k := xsKeyMap[key]
			setXSettingsName(k, convertStrListToColor(value))
			xsColorMap[key] = value
			op.PropChanged(key, "color")
		}
	})
}

func newXSettingsManager() *XSettingsManager {
	m := &XSettingsManager{}

	for _, v := range xsKeyMap {
		m.PropList = append(m.PropList, v)
	}

	return m
}

func startXSettings() {
	var err error
	X, err = xgb.NewConn()
	if err != nil {
		Logger.Info("Unable to connect X server:", err)
		panic(err)
	}
	xsIntMap = make(map[string]uint32)
	xsStrMap = make(map[string]string)
	xsColorMap = make(map[string][]string)

	newXWindow()
	initSelection()

	m := newXSettingsManager()
	dbus.InstallOnSession(m)
	//dbus.DealWithUnhandledMessage()

	//select {}
}
