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
)

var (
	errPropInvalid      = fmt.Errorf("Invalid xsettings property")
	errPropNotFound     = fmt.Errorf("This property not found")
	errPropTypeNotMatch = fmt.Errorf("This property's type not match")
)

func (m *XSManager) ListProps() string {
	datas, err := getSettingPropValue(m.owner, m.conn)
	if err != nil {
		return ""
	}

	infos := marshalSettingData(datas)
	if infos == nil || len(infos.items) == 0 {
		return ""
	}
	return infos.items.listProps()
}

func (m *XSManager) SetInteger(prop string, v int32) error {
	var setting = xsSetting{
		sType: settingTypeInteger,
		prop:  prop,
		value: v,
	}

	err := m.setSettings([]xsSetting{setting})
	if err != nil {
		logger.Debugf("Set '%s' to '%v' failed: %v", prop, v, err)
		return err
	}
	m.setGSettingsByXProp(prop, v)

	return nil
}

func (m *XSManager) GetInteger(prop string) (int32, error) {
	v, sType, err := m.getSettingValue(prop)
	if err != nil {
		logger.Debug("Get '%s' value failed: %v", prop, err)
		return -1, err
	}

	if sType != settingTypeInteger {
		return -1, errPropTypeNotMatch
	}

	return v.(*integerValueInfo).value, nil
}

func (m *XSManager) SetString(prop, v string) error {
	var setting = xsSetting{
		sType: settingTypeString,
		prop:  prop,
		value: v,
	}

	err := m.setSettings([]xsSetting{setting})
	if err != nil {
		logger.Debugf("Set '%s' to '%v' failed: %v", prop, v, err)
		return err
	}
	m.setGSettingsByXProp(prop, v)

	return nil
}

func (m *XSManager) GetString(prop string) (string, error) {
	v, sType, err := m.getSettingValue(prop)
	if err != nil {
		logger.Debug("Get '%s' value failed: %v", prop, err)
		return "", err
	}

	if sType != settingTypeString {
		return "", errPropTypeNotMatch
	}

	return v.(*stringValueInfo).value, nil
}

func (m *XSManager) SetColor(prop string, v [4]int16) error {
	var setting = xsSetting{
		sType: settingTypeColor,
		prop:  prop,
		value: v,
	}

	err := m.setSettings([]xsSetting{setting})
	if err != nil {
		logger.Debugf("Set '%s' to '%v' failed: %v", prop, v, err)
		return err
	}
	m.setGSettingsByXProp(prop, v)

	return nil
}

func (m *XSManager) GetColor(prop string) ([4]int16, error) {
	v, sType, err := m.getSettingValue(prop)
	if err != nil {
		logger.Debug("Get '%s' value failed: %v", prop, err)
		return [4]int16{}, err
	}

	if sType != settingTypeColor {
		return [4]int16{}, errPropTypeNotMatch
	}

	tmp := v.(*colorValueInfo)

	return [4]int16{tmp.red, tmp.blue, tmp.green, tmp.alpha}, nil
}

func (m *XSManager) getSettingValue(prop string) (interface{}, int8, error) {
	datas, err := getSettingPropValue(m.owner, m.conn)
	if err != nil {
		return nil, -1, err
	}

	xsInfo := marshalSettingData(datas)
	item := xsInfo.getPropItem(prop)
	if item == nil {
		return nil, -1, errPropNotFound
	}

	return item.value, item.header.sType, nil
}

func (m *XSManager) setGSettingsByXProp(prop string, v interface{}) {
	info := gsInfos.getInfoByXSKey(prop)
	if info == nil {
		return
	}

	info.setKeyValue(m.gs, v)
}
