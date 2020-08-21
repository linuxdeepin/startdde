/*
 * Copyright (C) 2017 ~ 2018 Deepin Technology Co., Ltd.
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

package wm

import (
	x "github.com/linuxdeepin/go-x11-client"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/log"
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
