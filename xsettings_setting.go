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
	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xprop"
	"os"
)

const (
	settingPropScreen   = "_XSETTINGS_S0"
	settingPropSettings = "_XSETTINGS_SETTINGS"

	xsDataOrder  = 0
	xsDataSerial = 0
	xsDataFormat = 8
)

func getSelectionOwner(prop string, conn *xgb.Conn) (xproto.Window, error) {
	atom, err := getAtomByProp(prop, conn)
	if err != nil {
		return 0, err
	}

	reply, err := xproto.GetSelectionOwner(conn, atom).Reply()
	if err != nil {
		return 0, err
	}

	return reply.Owner, nil
}

func isSelectionOwned(prop string, wid xproto.Window, conn *xgb.Conn) bool {
	owner, err := getSelectionOwner(prop, conn)
	if err != nil {
		return false
	}

	if owner == 0 || owner != wid {
		return false
	}

	return true
}

func getAtomByProp(prop string, conn *xgb.Conn) (xproto.Atom, error) {
	reply, err := xproto.InternAtom(conn, false,
		uint16(len(prop)), prop).Reply()
	if err != nil {
		return 0, err
	}

	return reply.Atom, nil
}

func getSettingPropValue(owner xproto.Window, conn *xgb.Conn) ([]byte, error) {
	atom, err := getAtomByProp(settingPropSettings, conn)
	if err != nil {
		return nil, err
	}

	reply, err := xproto.GetProperty(conn, false, owner,
		atom, atom, 0, 10240).Reply()
	if err != nil {
		return nil, err
	}

	return reply.Value, nil
}

func changeSettingProp(owner xproto.Window, data []byte, conn *xgb.Conn) error {
	atom, err := getAtomByProp(settingPropSettings, conn)
	if err != nil {
		return err
	}

	return xproto.ChangePropertyChecked(conn, xproto.PropModeReplace,
		owner, atom, atom,
		xsDataFormat, uint32(len(data)), data).Check()
}

func createSettingWindow(conn *xgb.Conn) (xproto.Window, error) {
	screenAtom, err := getAtomByProp(settingPropScreen, conn)
	if err != nil {
		return 0, err
	}

	wid, err := xproto.NewWindowId(conn)
	if err != nil {
		return 0, err
	}

	var screen = xproto.Setup(conn).DefaultScreen(conn)
	err = xproto.CreateWindowChecked(conn, 0, wid, screen.Root,
		0, 0, 1, 1, 0,
		xproto.WindowClassInputOnly, screen.RootVisual,
		0, nil).Check()
	if err != nil {
		return 0, err
	}

	err = changeWindowPid(wid)
	if err != nil {
		return 0, err
	}

	err = xproto.SetSelectionOwnerChecked(conn, wid, screenAtom,
		xproto.TimeCurrentTime).Check()
	if err != nil {
		return 0, err
	}

	conn.Sync()
	return wid, nil
}

func changeWindowPid(wid xproto.Window) error {
	xu, err := xgbutil.NewConn()
	if err != nil {
		return err
	}

	return xprop.ChangeProp32(xu, wid,
		"_NET_WM_PID", "CARDINAL", uint(os.Getpid()))
}
