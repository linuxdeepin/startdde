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
	"fmt"
	"gir/gio-2.0"
	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
	"pkg.deepin.io/lib/dbus"
)

const (
	xsSchema = "com.deepin.xsettings"
)

type XSManager struct {
	conn  *xgb.Conn
	owner xproto.Window

	gs *gio.Settings
}

type xsSetting struct {
	sType int8
	prop  string
	value interface{} // int32, string, [4]int16
}

func NewXSManager(conn *xgb.Conn) (*XSManager, error) {
	var m = &XSManager{
		conn: conn,
	}

	var err error
	m.owner, err = createSettingWindow(m.conn)
	if err != nil {
		m.conn.Close()
		return nil, err
	}

	if !isSelectionOwned(settingPropScreen, m.owner, m.conn) {
		m.conn.Close()
		logger.Errorf("Owned '%s' failed", settingPropSettings)
		return nil, fmt.Errorf("Owned '%s' failed", settingPropSettings)
	}

	m.gs = gio.NewSettings(xsSchema)
	err = m.setSettings(m.getSettingsInSchema())
	if err != nil {
		logger.Warning("Change xsettings property failed:", err)
	}

	return m, nil
}

func (m *XSManager) setSettings(settings []xsSetting) error {
	datas, err := getSettingPropValue(m.owner, m.conn)
	if err != nil {
		return err
	}

	xsInfo := marshalSettingData(datas)
	xsInfo.serial = xsDataSerial
	for _, s := range settings {
		item := xsInfo.getPropItem(s.prop)
		if item != nil {
			xsInfo.items = xsInfo.modifyProperty(s)
			continue
		}

		var tmp *xsItemInfo
		switch s.sType {
		case settingTypeInteger:
			tmp = newXSItemInteger(s.prop, s.value.(int32))
		case settingTypeString:
			tmp = newXSItemString(s.prop, s.value.(string))
		case settingTypeColor:
			tmp = newXSItemColor(s.prop, s.value.([4]int16))
		}

		xsInfo.items = append(xsInfo.items, *tmp)
		xsInfo.numSettings++
	}

	data := unmarshalSettingData(xsInfo)
	return changeSettingProp(m.owner, data, m.conn)
}

func (m *XSManager) getSettingsInSchema() []xsSetting {
	var settings []xsSetting
	for _, key := range m.gs.ListKeys() {
		info := gsInfos.getInfoByGSKey(key)
		if info == nil {
			continue
		}

		settings = append(settings, xsSetting{
			sType: info.getKeySType(),
			prop:  info.xsKey,
			value: info.getKeyValue(m.gs),
		})
	}

	return settings
}

func (m *XSManager) handleGSettingsChanged() {
	m.gs.Connect("changed", func(s *gio.Settings, key string) {
		switch key {
		case "xft-dpi":
			return
		case "scale-factor":
			m.updateDPI()
			m.updateXResources()
			return
		case "gtk-cursor-theme-name":
			updateXResources(xresourceInfos{
				&xresourceInfo{
					key:   "Xcursor.theme",
					value: m.gs.GetString("gtk-cursor-theme-name"),
				},
			})
		case "gtk-cursor-theme-size":
			updateXResources(xresourceInfos{
				&xresourceInfo{
					key:   "Xcursor.size",
					value: fmt.Sprintf("%d", m.gs.GetInt("gtk-cursor-theme-size")),
				},
			})
		}

		info := gsInfos.getInfoByGSKey(key)
		if info == nil {
			return
		}

		m.setSettings([]xsSetting{{
			sType: info.getKeySType(),
			prop:  info.xsKey,
			value: info.getKeyValue(m.gs),
		},
		})
	})
	//fixed the gsettings signal handling broken after glib2.43
	m.gs.GetString("theme-name")
}

func startXSettings(conn *xgb.Conn) {
	m, err := NewXSManager(conn)
	if err != nil {
		logger.Error("Start xsettings failed:", err)
		return
	}
	m.updateDPI()
	m.updateXResources()
	go m.updateFirefoxDPI()

	err = dbus.InstallOnSession(m)
	if err != nil {
		logger.Error("Install dbus session failed:", err)
		return
	}
	dbus.DealWithUnhandledMessage()

	m.handleGSettingsChanged()
}
