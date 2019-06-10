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

package xsettings

import (
	"bytes"
	"encoding/binary"
	"io"
)

func (info *xsDataInfo) modifyProperty(setting xsSetting) xsItemInfos {
	var (
		tmp   xsItemInfos
		items = info.items
	)

	for _, item := range items {
		if item.header.name == setting.prop {
			var ptr = &item
			item.header.lastChangeSerial++
			ptr.changePropValue(setting.value)
		}
		tmp = append(tmp, item)
	}

	return tmp
}

func (item *xsItemInfo) changePropValue(value interface{}) {
	switch item.header.sType {
	case settingTypeInteger:
		item.changeValueInteger(value.(int32))
	case settingTypeString:
		item.changeValueString(value.(string))
	case settingTypeColor:
		item.changeValueColor(value.([4]int16))
	}
}

func (item *xsItemInfo) changeValueInteger(value int32) {
	v, ok := item.value.(*integerValueInfo)
	if !ok || v.value == value {
		return
	}

	v.value = value
}

func (item *xsItemInfo) changeValueString(value string) {
	v, ok := item.value.(*stringValueInfo)
	if !ok || v.value == value {
		return
	}

	v.length = int32(len(value))
	v.value = value
	v.pad = int(3 - (v.length+3)%4)
}

func (item *xsItemInfo) changeValueColor(value [4]int16) {
	v, ok := item.value.(*colorValueInfo)
	if !ok || (v.red == value[0] && v.blue == value[1] &&
		v.green == value[2] && v.alpha == value[3]) {
		return
	}

	v.red = value[0]
	v.blue = value[1]
	v.green = value[2]
	v.alpha = value[3]
}

func unmarshalSettingData(info *xsDataInfo) []byte {
	var buf = new(bytes.Buffer)

	pushInteger(buf, &info.byteOrder)
	pushUnused(buf, info.unused)
	pushInteger(buf, &info.serial)
	pushInteger(buf, &info.numSettings)
	for _, item := range info.items {
		pushXSItemInfo(buf, &item)
	}

	return buf.Bytes()
}

func newXSItemInteger(prop string, v int32) *xsItemInfo {
	var item = xsItemInfo{
		header: newXSItemHeader(prop),
		value: &integerValueInfo{
			value: v,
		},
	}

	item.header.sType = settingTypeInteger
	return &item
}

func newXSItemString(prop string, v string) *xsItemInfo {
	var item = xsItemInfo{
		header: newXSItemHeader(prop),
	}
	item.header.sType = settingTypeString

	var value = stringValueInfo{
		length: int32(len(v)),
		value:  v,
	}
	value.pad = int(3 - (value.length+3)%4)

	item.value = &value
	return &item
}

func newXSItemColor(prop string, v [4]int16) *xsItemInfo {
	var item = xsItemInfo{
		header: newXSItemHeader(prop),
	}
	item.header.sType = settingTypeColor

	var value = colorValueInfo{
		red:   v[0],
		blue:  v[1],
		green: v[2],
		alpha: v[3],
	}

	item.value = &value
	return &item
}

func newXSItemHeader(prop string) *xsItemHeader {
	var header = xsItemHeader{
		unused:           1,
		nameLen:          int16(len(prop)),
		name:             prop,
		lastChangeSerial: 1,
	}
	header.pad = int(3 - (header.nameLen+3)%4)

	return &header
}

func pushUnused(writer io.Writer, num int) {
	var buf = make([]byte, num)
	binary.Write(writer, defaultByteOrder, buf)
}

func pushInteger(writer io.Writer, v interface{}) {
	binary.Write(writer, defaultByteOrder, v)
}

func pushString(writer io.Writer, v string) {
	binary.Write(writer, defaultByteOrder, []byte(v))
}

func pushXSItemInfo(writer io.Writer, item *xsItemInfo) {
	pushXSInfoHeader(writer, item.header)

	switch item.header.sType {
	case settingTypeInteger:
		pushXSValueInteger(writer, item.value.(*integerValueInfo))
	case settingTypeString:
		pushXSValueString(writer, item.value.(*stringValueInfo))
	case settingTypeColor:
		pushXSValueColor(writer, item.value.(*colorValueInfo))
	}
}

func pushXSInfoHeader(writer io.Writer, header *xsItemHeader) {
	pushInteger(writer, &header.sType)
	pushUnused(writer, header.unused)
	pushInteger(writer, &header.nameLen)
	pushString(writer, header.name)
	pushUnused(writer, header.pad)
	pushInteger(writer, &header.lastChangeSerial)
}

func pushXSValueInteger(writer io.Writer, v *integerValueInfo) {
	pushInteger(writer, &v.value)
}

func pushXSValueString(writer io.Writer, v *stringValueInfo) {
	pushInteger(writer, &v.length)
	pushString(writer, v.value)
	pushUnused(writer, v.pad)
}

func pushXSValueColor(writer io.Writer, v *colorValueInfo) {
	pushInteger(writer, &v.red)
	pushInteger(writer, &v.blue)
	pushInteger(writer, &v.green)
	pushInteger(writer, &v.alpha)
}
