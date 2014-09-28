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
	"github.com/BurntSushi/xgb"
	"pkg.linuxdeepin.com/lib/dbus"
)

type XSettingsManager struct {
	PropList []string
}

const (
	XSETTINGS_DEST = "com.deepin.SessionManager"
	XSETTINGS_PATH = "/com/deepin/XSettings"
	XSETTINGS_IFC  = "com.deepin.XSettings"
)

var (
	X             *xgb.Conn
	xsKeyValueMap = make(map[string]interface{})
)

func (op *XSettingsManager) SetInteger(key string, value uint32) {
	if len(key) <= 0 {
		return
	}
	setXSettingsName(key, value)
	xsKeyValueMap[key] = value
	op.setPropName("PropList")
}

func (op *XSettingsManager) GetInteger(key string) (uint32, bool) {
	ret, ok := xsKeyValueMap[key]
	if !ok {
		return 0, false
	}

	return ret.(uint32), true
}

func (op *XSettingsManager) SetString(key, value string) {
	if len(key) <= 0 {
		return
	}
	setXSettingsName(key, value)
	xsKeyValueMap[key] = value
	op.setPropName("PropList")
}

func (op *XSettingsManager) GetString(key string) (string, bool) {
	ret, ok := xsKeyValueMap[key]
	if !ok {
		return "", false
	}

	return ret.(string), true
}

/*
  Color: [4]string
*/
func (op *XSettingsManager) SetColor(key string, value []byte) {
	if len(key) <= 0 || len(value) != 4 {
		return
	}
	setXSettingsName(key, value)
	xsKeyValueMap[key] = value
	op.setPropName("PropList")
}

func (op *XSettingsManager) GetColor(key string) ([]byte, bool) {
	ret, ok := xsKeyValueMap[key]
	if !ok {
		return []byte{}, false
	}

	return ret.([]byte), true
}

func (op *XSettingsManager) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		XSETTINGS_DEST,
		XSETTINGS_PATH,
		XSETTINGS_IFC,
	}
}

func (op *XSettingsManager) setPropName(propName string) {
	switch propName {
	case "PropList":
		list := []string{}
		for k, _ := range xsKeyValueMap {
			list = append(list, k)
		}
		op.PropList = list
	}
}

func newXSettingsManager() *XSettingsManager {
	m := &XSettingsManager{}
	m.setPropName("PropList")

	return m
}

func startXSettings() {
	var err error
	X, err = xgb.NewConn()
	if err != nil {
		logger.Info("Unable to connect X server:", err)
		panic(err)
	}

	newXWindow()
	initSelection()

	m := newXSettingsManager()
	dbus.InstallOnSession(m)
	//dbus.DealWithUnhandledMessage()

	//select {}
}
