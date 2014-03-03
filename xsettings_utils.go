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

        XSETTINGS_STRING_ID   = "com.deepin.dde.xsettings.type-string"
        XSETTINGS_INTERGER_ID = "com.deepin.dde.xsettings.type-interger"
        XSETTINGS_COLOR_ID    = "com.deepin.dde.xsettings.type-color"
)

var (
        sReply    *xproto.GetSelectionOwnerReply
        byteOrder binary.ByteOrder

        xStrSettings   = gio.NewSettings(XSETTINGS_STRING_ID)
        xIntSettings   = gio.NewSettings(XSETTINGS_INTERGER_ID)
        xColorSettings = gio.NewSettings(XSETTINGS_COLOR_ID)
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
        strList := xStrSettings.ListKeys()
        intList := xIntSettings.ListKeys()
        colorList := xColorSettings.ListKeys()

        for _, key := range strList {
                k, ok := xsKeyMap[key]
                if !ok {
                        continue
                }
                value := xStrSettings.GetString(key)
                xsStrMap[key] = value
                setXSettingsName(k, value)
        }

        for _, key := range intList {
                k, ok := xsKeyMap[key]
                if !ok {
                        continue
                }
                value := xIntSettings.GetUint(key)
                xsIntMap[key] = uint32(value)
                setXSettingsName(k, uint32(value))
        }

        for _, key := range colorList {
                k, ok := xsKeyMap[key]
                if !ok {
                        continue
                }
                values := xColorSettings.GetStrv(key)
                xsColorMap[key] = values
                tmp := []byte{}
                for _, v := range values {
                        n, _ := strconv.ParseUint(v, 10, 16)
                        tmp = append(tmp, byte(n))
                }
                setXSettingsName(k, tmp)
        }
}

func convertStrListToColor(value []string) []byte {
        tmp := []byte{}

        for _, v := range value {
                n, _ := strconv.ParseUint(v, 10, 16)
                tmp = append(tmp, byte(n))
        }

        return tmp
}

func isStrArrayEqual(list1, list2 []string) bool {
        l1 := len(list1)
        l2 := len(list2)

        if l1 != l2 {
                return false
        }

        for i := 0; i < l1; i++ {
                if list1[i] != list2[i] {
                        return false
                }
        }

        return true
}

func getXSettingsKey(str string) string {
        tmp := ""
        for k, v := range xsKeyMap {
                if v == str {
                        tmp = k
                        break
                }
        }

        return tmp
}

func getCurrentTimestamp() int64 {
        t := time.Now().Unix()
        logger.Println("Timestamp:", t)
        return t
}
