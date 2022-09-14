// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package wm

import (
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/log"
)

var _s *Switcher

// Start launch wm
func Start(conn *x.Conn, logger *log.Logger, wmChooserLaunched bool, service *dbusutil.Service) error {
	if _s != nil {
		return nil
	}

	_s = new(Switcher)
	_s.service = service
	_s.conn = conn
	_s.wmChooserLaunched = wmChooserLaunched
	_s.logger = logger
	_s.init()
	_s.listenStartupReady()
	_s.listenWMChanged()
	_s.adjustSogouSkin()

	err := service.Export(swDBusPath, _s)
	if err != nil {
		return err
	}

	err = service.RequestName(swDBusDest)
	return err
}

// GetWM return current window manager
func GetWM() string {
	if _s != nil {
		return _s.getWM()
	}
	return ""
}

func ShouldWait() bool {
	if _s != nil {
		return _s.shouldWait()
	}
	return true
}
