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
	"encoding/binary"
	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xprop"
	"os"
)

type HeaderInfo struct {
	vType      byte
	nameLen    uint16
	name       string
	lastSerial uint32
	value      interface{}
}

const (
	XSETTINGS_S0       = "_XSETTINGS_S0"
	XSETTINGS_SETTINGS = "_XSETTINGS_SETTINGS"

	XSETTINGS_FORMAT = 8
	XSETTINGS_ORDER  = 0
	XSETTINGS_SERIAL = 0

	XSETTINGS_INTERGER = 0
	XSETTINGS_STRING   = 1
	XSETTINGS_COLOR    = 2
)

var (
	sReply    *xproto.GetSelectionOwnerReply
	byteOrder binary.ByteOrder
)

func getAtom(X *xgb.Conn, name string) xproto.Atom {
	reply, err := xproto.InternAtom(X, false,
		uint16(len(name)), name).Reply()
	if err != nil {
		logger.Infof("'%s' Get Xproto Atom Failed: %s",
			name, err)
	}

	return reply.Atom
}

func newXWindow() {
	wid, err := xproto.NewWindowId(X)
	if err != nil {
		logger.Infof("New Window Id Failed: %v", err)
		panic(err)
	}
	logger.Infof("New window id: %v", wid)

	setupInfo := xproto.Setup(X)

	screen := setupInfo.DefaultScreen(X)
	logger.Infof("root wid: %v", screen.Root)
	err = xproto.CreateWindowChecked(X,
		0,
		wid, screen.Root, 0, 0,
		1, 1, 0, xproto.WindowClassInputOnly,
		screen.RootVisual, 0,
		nil).Check()
	if err != nil {
		panic(err)
	}
	setWMPID(wid)
	err = xproto.SetSelectionOwnerChecked(X, wid,
		getAtom(X, XSETTINGS_S0),
		xproto.TimeCurrentTime).Check()
	//xproto.Timestamp(getCurrentTimestamp())).Check()
	if err != nil {
		panic(err)
	}
	X.Sync()
}

func initSelection() {
	var err error

	if XSETTINGS_ORDER == 1 {
		byteOrder = binary.BigEndian
	} else {
		byteOrder = binary.LittleEndian
	}

	sReply, err = xproto.GetSelectionOwner(X,
		getAtom(X, XSETTINGS_S0)).Reply()
	if err != nil {
		logger.Infof("Unable to connect X server: %v", err)
		panic(err)
	}
	logger.Infof("select owner wid: %v", sReply.Owner)
}

func setWMPID(wid xproto.Window) {
	xu, err := xgbutil.NewConn()
	if err != nil {
		return
	}
	xprop.ChangeProp32(xu, wid, "_NET_WM_PID", "CARDINAL", uint(os.Getpid()))
}
