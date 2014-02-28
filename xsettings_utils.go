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
        "dlib/gio-2.0"
        "dlib/logger"
        "encoding/binary"
        "github.com/BurntSushi/xgb"
        "github.com/BurntSushi/xgb/xproto"
        "strconv"
        "strings"
        "time"
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

        XSETTINGS_SCHEMA_ID = "com.deepin.dde.xsettings"
)

var (
        sReply    *xproto.GetSelectionOwnerReply
        byteOrder binary.ByteOrder

        xsSettings = gio.NewSettings(XSETTINGS_SCHEMA_ID)
)

func getAtom(X *xgb.Conn, name string) xproto.Atom {
        reply, err := xproto.InternAtom(X, false,
                uint16(len(name)), name).Reply()
        if err != nil {
                logger.Printf("'%s' Get Xproto Atom Failed: %s\n",
                        name, err)
        }

        return reply.Atom
}

func newXWindow() {
        wid, err := xproto.NewWindowId(X)
        if err != nil {
                logger.Println("New Window Id Failed:", err)
                panic(err)
        }
        logger.Println("New window id:", wid)

        setupInfo := xproto.Setup(X)
        /*
           for _, screenInfo := setupInfo.Roots {
           }
        */
        screen := setupInfo.DefaultScreen(X)
        logger.Println("root wid:", screen.Root)
        err = xproto.CreateWindowChecked(X,
                0,
                wid, screen.Root, 0, 0,
                1, 1, 0, xproto.WindowClassInputOnly,
                screen.RootVisual, 0,
                nil).Check()
        if err != nil {
                panic(err)
        }
        err = xproto.SetSelectionOwnerChecked(X, wid,
                getAtom(X, XSETTINGS_S0),
                xproto.TimeCurrentTime).Check()
        //xproto.Timestamp(getCurrentTimestamp())).Check()
        if err != nil {
                panic(err)
        }
        xproto.MapWindow(X, wid)
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
                logger.Println("Unable to connect X server:", err)
                panic(err)
        }
        logger.Println("select owner wid:", sReply.Owner)

        setAllXSettingsKeys()
}

func setAllXSettingsKeys() {
        for k, _ := range xsKeyMap {
                strs := xsSettings.GetString(k)
                a := strings.Split(strs, ";")
                if len(a) != 2 {
                        continue
                }
                t, _ := strconv.ParseInt(a[1], 10, 64)
                setXSettingsKey(k, a[0], int32(t))
        }
}

func setXSettingsKey(key, value string, t int32) {
        logger.Println("type:", t)
        v, ok := xsKeyMap[key]
        if !ok {
                return
        }
        switch t {
        case XSETTINGS_INTERGER:
                vInt, _ := strconv.ParseUint(value, 10, 32)
                logger.Printf("Set: %s, Value: %d\n", v, vInt)
                setXSettingsName(v, uint32(vInt))
        case XSETTINGS_STRING:
                logger.Printf("Set: %s, Value: %s\n", v, value)
                setXSettingsName(v, value)
        case XSETTINGS_COLOR:
                tmps := strings.Split(value, ",")
                bytes := []byte{}
                for _, s := range tmps {
                        b, _ := strconv.ParseInt(s, 10, 8)
                        bytes = append(bytes, byte(b))
                }
                logger.Println("Set:", v, ", Value:", bytes)
                setXSettingsName(v, bytes)
        }
}

func setGSettingsKey(key, value string, t int32) {
        switch t {
        case XSETTINGS_INTERGER:
                xsSettings.SetString(key, value+";0")
        case XSETTINGS_STRING:
                xsSettings.SetString(key, value+";1")
        case XSETTINGS_COLOR:
                xsSettings.SetString(key, value+";2")
        }
}

func getCurrentTimestamp() int64 {
        t := time.Now().Unix()
        logger.Println("Timestamp:", t)
        return t
}
