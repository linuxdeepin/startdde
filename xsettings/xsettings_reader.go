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
	"fmt"
	"io"
)

const (
	settingTypeInteger int8 = iota
	settingTypeString
	settingTypeColor
)

var (
	defaultByteOrder = binary.LittleEndian
)

type stringValueInfo struct {
	length int32
	value  string
	pad    int
}

type integerValueInfo struct {
	value int32
}

type colorValueInfo struct {
	red   int16
	blue  int16
	green int16
	//If the setting does not need the alpha field,
	//it should be set to 65535.
	alpha int16
}

type xsItemHeader struct {
	sType            int8  // setting type
	unused           int   //1byte
	nameLen          int16 // name length
	name             string
	pad              int // 用于内存对齐，计算方法：3-(nameLen+3)%4
	lastChangeSerial int32
}

type xsItemInfo struct {
	header *xsItemHeader
	value  interface{}
}

type xsItemInfos []xsItemInfo

type xsDataInfo struct {
	byteOrder   int8
	unused      int // 3byte
	serial      int32
	numSettings int32

	items xsItemInfos
}

func (infos xsItemInfos) listProps() string {
	var content = "["
	for i, info := range infos {
		if i != 0 {
			content += ","
		}
		content += fmt.Sprintf("%q", info.header.name)
	}
	return content + "]"
}

func (info *xsDataInfo) getEnabledProps() []string {
	var props []string
	for _, item := range info.items {
		props = append(props, item.header.name)
	}

	return props
}

func (info *xsDataInfo) getPropItem(prop string) *xsItemInfo {
	for _, item := range info.items {
		if prop == item.header.name {
			return &item
		}
	}

	return nil
}

func marshalSettingData(datas []byte) *xsDataInfo {
	var info = xsDataInfo{unused: 3}
	if len(datas) == 0 {
		info.byteOrder = xsDataOrder
		info.numSettings = 0
		info.serial = xsDataSerial
		return &info
	}

	var reader = bytes.NewReader(datas)

	popInteger(reader, &info.byteOrder)
	popUnused(reader, info.unused)
	popInteger(reader, &info.serial)
	popInteger(reader, &info.numSettings)
	for i := 0; i < int(info.numSettings); i++ {
		var item = xsItemInfo{
			header: &xsItemHeader{
				unused: 1,
			},
		}
		popXSItemInfo(reader, &item)
		info.items = append(info.items, item)
	}

	return &info
}

func popUnused(reader io.Reader, num int) {
	var buf = make([]byte, num)
	binary.Read(reader, defaultByteOrder, &buf)
}

func popInteger(reader io.Reader, v interface{}) {
	binary.Read(reader, defaultByteOrder, v)
}

func popString(reader io.Reader, v *string, length int) {
	var buf = make([]byte, length)
	binary.Read(reader, defaultByteOrder, &buf)
	*v = string(buf)
}

func popXSItemInfo(reader io.Reader, item *xsItemInfo) {
	popXSItemHeader(reader, item.header)

	switch item.header.sType {
	case settingTypeInteger:
		var v = integerValueInfo{}
		popXSValueInteger(reader, &v)
		item.value = &v
	case settingTypeString:
		var v = stringValueInfo{}
		popXSValueString(reader, &v)
		item.value = &v
	case settingTypeColor:
		var v = colorValueInfo{}
		popXSValueColor(reader, &v)
		item.value = &v
	}
}

func popXSItemHeader(reader io.Reader, header *xsItemHeader) {
	popInteger(reader, &header.sType)
	header.unused = 1
	popUnused(reader, header.unused)
	popInteger(reader, &header.nameLen)
	popString(reader, &header.name, int(header.nameLen))
	header.pad = int(3 - (header.nameLen+3)%4)
	popUnused(reader, header.pad)
	popInteger(reader, &header.lastChangeSerial)
}

func popXSValueInteger(reader io.Reader, v *integerValueInfo) {
	popInteger(reader, &v.value)
}

func popXSValueString(reader io.Reader, v *stringValueInfo) {
	popInteger(reader, &v.length)
	popString(reader, &v.value, int(v.length))
	v.pad = int(3 - (v.length+3)%4)
	popUnused(reader, v.pad)
}

func popXSValueColor(reader io.Reader, v *colorValueInfo) {
	popInteger(reader, &v.red)
	popInteger(reader, &v.blue)
	popInteger(reader, &v.green)
	popInteger(reader, &v.alpha)
}
